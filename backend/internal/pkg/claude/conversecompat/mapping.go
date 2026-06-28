// Package conversecompat 把标准 Anthropic Messages 响应转换为 AWS Bedrock Converse 形态。
//
// 纯转换包：无 service 依赖，仅做 JSON → JSON 字段重映射。字段映射依据
// claudedocs/账号级协议适配-bedrock与kiro-设计方案.md 的 §5.4②/§11.2 表 A / §11.3 陷阱。
//
// 当前实现覆盖非流式响应（AnthropicToConverseJSON）。流式（二进制 EventStream）留待后续批次。
package conversecompat

import (
	"encoding/json"
	"log/slog"
)

// nativeResponse 是标准 Anthropic 非流式响应的关心字段。
type nativeResponse struct {
	Type       string            `json:"type"`
	Role       string            `json:"role"`
	Content    []json.RawMessage `json:"content"`
	StopReason string            `json:"stop_reason"`
	Usage      nativeUsage       `json:"usage"`
}

type nativeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// converseResponse 是 Bedrock Converse 非流式响应形态。
type converseResponse struct {
	Output     converseOutput `json:"output"`
	StopReason string         `json:"stopReason"`
	Usage      converseUsage  `json:"usage"`
}

type converseOutput struct {
	Message converseMessage `json:"message"`
}

type converseMessage struct {
	Role    string           `json:"role"`
	Content []map[string]any `json:"content"`
}

type converseUsage struct {
	InputTokens           int `json:"inputTokens"`
	OutputTokens          int `json:"outputTokens"`
	TotalTokens           int `json:"totalTokens"`
	CacheReadInputTokens  int `json:"cacheReadInputTokens"`
	CacheWriteInputTokens int `json:"cacheWriteInputTokens"`
}

// AnthropicToConverseJSON 把标准 Anthropic 非流式响应体改写为 Converse JSON。
// 解析失败或不是 assistant message 响应（如错误体）时原样返回，避免破坏非目标响应。
func AnthropicToConverseJSON(native []byte) []byte {
	var nr nativeResponse
	if err := json.Unmarshal(native, &nr); err != nil {
		return native // 非合法 JSON：原样透传
	}
	// 只改写 assistant message 响应；错误体 / 其它形态原样透传。
	if nr.Type != "" && nr.Type != "message" {
		return native
	}
	if nr.Content == nil {
		return native
	}

	role := nr.Role
	if role == "" {
		role = "assistant"
	}

	content := make([]map[string]any, 0, len(nr.Content))
	for _, raw := range nr.Content {
		content = append(content, mapContentBlock(raw))
	}

	out := converseResponse{
		Output: converseOutput{
			Message: converseMessage{Role: role, Content: content},
		},
		StopReason: mapStopReason(nr.StopReason),
		Usage: converseUsage{
			InputTokens:          nr.Usage.InputTokens,
			OutputTokens:         nr.Usage.OutputTokens,
			TotalTokens:          nr.Usage.InputTokens + nr.Usage.OutputTokens,
			CacheReadInputTokens: nr.Usage.CacheReadInputTokens,
			// creation→write 改名（§11.3 唯一非纯 snake→camel）；5m/1h 细分已汇总在此，丢弃。
			CacheWriteInputTokens: nr.Usage.CacheCreationInputTokens,
		},
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return native // 理论不可达：构造失败时透传
	}
	return encoded
}

// mapContentBlock 把单个 Anthropic content 块映射为 Converse content 块（表 A）。
func mapContentBlock(raw json.RawMessage) map[string]any {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return map[string]any{"text": "[unsupported block]"}
	}

	switch head.Type {
	case "text":
		var b struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(raw, &b)
		return map[string]any{"text": b.Text}

	case "tool_use", "server_tool_use":
		// server_tool_use 与 tool_use 结构同构，按 tool_use 同款映射。
		var b struct {
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		_ = json.Unmarshal(raw, &b)
		return map[string]any{
			"toolUse": map[string]any{
				"toolUseId": b.ID, // 原样保留（含 toolu_bdrk_ 前缀），勿剥
				"name":      b.Name,
				"input":     rawOrNull(b.Input),
			},
		}

	case "thinking":
		var b struct {
			Thinking  string `json:"thinking"`
			Signature string `json:"signature"`
		}
		_ = json.Unmarshal(raw, &b)
		return map[string]any{
			"reasoningContent": map[string]any{
				"reasoningText": map[string]any{
					"text":      b.Thinking, // thinking→reasoningText.text
					"signature": b.Signature,
				},
			},
		}

	case "redacted_thinking":
		var b struct {
			Data string `json:"data"`
		}
		_ = json.Unmarshal(raw, &b)
		return map[string]any{
			"reasoningContent": map[string]any{
				"redactedContent": b.Data, // 扁平 bytes 成员
			},
		}

	case "image":
		var b struct {
			Source struct {
				MediaType string `json:"media_type"`
				Data      string `json:"data"`
			} `json:"source"`
		}
		_ = json.Unmarshal(raw, &b)
		return map[string]any{
			"image": map[string]any{
				"format": imageFormat(b.Source.MediaType), // image/png → png
				"source": map[string]any{"bytes": b.Source.Data},
			},
		}

	default:
		// 未知块：降级为可观测占位，保证 Converse 结构合法、不中断响应。
		slog.Warn("conversecompat: unsupported content block, downgraded to text",
			"type", head.Type)
		return map[string]any{"text": "[unsupported block: " + head.Type + "]"}
	}
}

// mapStopReason 把 Anthropic stop_reason 映射为 Converse stopReason。
func mapStopReason(s string) string {
	switch s {
	case "end_turn", "max_tokens", "stop_sequence", "tool_use", "model_context_window_exceeded":
		return s // 5 值同名直传
	case "pause_turn":
		return "end_turn"
	case "refusal":
		return "content_filtered"
	case "":
		return "" // 缺省：不强加
	default:
		slog.Warn("conversecompat: unknown stop_reason, mapped to end_turn", "stop_reason", s)
		return "end_turn"
	}
}

// imageFormat 从 media_type（如 image/png）取 Converse format（png）。
func imageFormat(mediaType string) string {
	for i := 0; i < len(mediaType); i++ {
		if mediaType[i] == '/' {
			return mediaType[i+1:]
		}
	}
	return mediaType
}

// rawOrNull 把 json.RawMessage 转为可被 json.Marshal 原样输出的值；空时返回 nil。
func rawOrNull(r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	return r
}
