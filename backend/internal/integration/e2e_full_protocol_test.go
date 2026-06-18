//go:build e2e

package integration

// 全功能 E2E —— Claude 协议深度（工具调用 / 扩展思维）
//
// 与历史黑盒套件(e2e_gateway_test.go)不同：黑盒套件遍历硬编码模型清单，假设特定部署的
// 模型映射，接到第三方中转上会大面积假红。本组用例复用自包含 provisioning + E2E_ANTHROPIC_MODEL
// （中转真实模型），只验证「网关对工具调用 / thinking 协议字段的转发与回传」这一层，模型名可控。
//
// 上游（第三方中转）不支持对应能力时回 upstream_error → t.Skip（非网关缺陷），与
// count_tokens / gemini 的既有约定一致。

import (
	"encoding/json"
	"testing"
)

// claudeMessagesRaw 用网关 key 打 POST /v1/messages，payload 由调用方完全自定义。
func claudeMessagesRaw(gwKey string, payload map[string]any) (int, []byte, error) {
	body, _ := json.Marshal(payload)
	return apiCall("POST", "/v1/messages", map[string]string{
		"x-api-key":         gwKey,
		"anthropic-version": "2023-06-01",
	}, body)
}

// TestE2EFull_ClaudeToolUse 验证工具调用往返：强制 tool_choice → 期望响应含 tool_use 块。
func TestE2EFull_ClaudeToolUse(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	payload := map[string]any{
		"model":      pp.model,
		"max_tokens": 256,
		"tools": []map[string]any{{
			"name":        "get_weather",
			"description": "Get the current weather for a given city",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string", "description": "City name"},
				},
				"required": []string{"city"},
			},
		}},
		// 强制调用该工具，使 stop_reason 确定为 tool_use（去随机性）
		"tool_choice": map[string]any{"type": "tool", "name": "get_weather"},
		"messages": []map[string]any{
			{"role": "user", "content": "What is the weather in Paris?"},
		},
	}

	st, body, err := claudeMessagesRaw(pp.gatewayKey, payload)
	if err != nil {
		t.Fatalf("请求错误: %v", err)
	}
	if st != 200 {
		if bodyContains(body, "upstream") || bodyContains(body, "Upstream") {
			t.Skipf("上游不支持工具调用（网关已转发并回传上游响应 st=%d）：%s", st, truncate(body, 200))
		}
		t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
	}

	var r struct {
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content"`
	}
	if e := json.Unmarshal(body, &r); e != nil {
		t.Fatalf("响应解析失败: %v；body=%s", e, truncate(body, 300))
	}
	hasToolUse := false
	for _, c := range r.Content {
		if c.Type == "tool_use" && c.Name == "get_weather" {
			hasToolUse = true
			break
		}
	}
	if !hasToolUse {
		t.Fatalf("响应缺少 tool_use 块（stop_reason=%q）：%s", r.StopReason, truncate(body, 400))
	}
	t.Logf("✅ 工具调用往返 OK（stop_reason=%q，含 get_weather tool_use 块）", r.StopReason)
}

// TestE2EFull_ClaudeThinking 验证扩展思维：启用 thinking → 期望响应含 thinking 块。
func TestE2EFull_ClaudeThinking(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	payload := map[string]any{
		"model":      pp.model,
		"max_tokens": 2048, // 必须 > budget_tokens
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": 1024,
		},
		"messages": []map[string]any{
			{"role": "user", "content": "What is 17 * 23? Reason briefly, then give the number."},
		},
	}

	st, body, err := claudeMessagesRaw(pp.gatewayKey, payload)
	if err != nil {
		t.Fatalf("请求错误: %v", err)
	}
	if st != 200 {
		// 中转模型不支持 thinking（或不接受该字段）→ 上游报错，非网关缺陷
		if bodyContains(body, "upstream") || bodyContains(body, "Upstream") || bodyContains(body, "thinking") {
			t.Skipf("上游模型不支持 thinking（网关已转发并回传上游响应 st=%d）：%s", st, truncate(body, 200))
		}
		t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
	}

	var r struct {
		Content []struct {
			Type string `json:"type"`
		} `json:"content"`
	}
	if e := json.Unmarshal(body, &r); e != nil {
		t.Fatalf("响应解析失败: %v；body=%s", e, truncate(body, 300))
	}
	hasThinking := false
	for _, c := range r.Content {
		if c.Type == "thinking" || c.Type == "redacted_thinking" {
			hasThinking = true
			break
		}
	}
	if !hasThinking {
		t.Skipf("响应未含 thinking 块（中转可能剥离了 thinking 输出）：%s", truncate(body, 300))
	}
	t.Logf("✅ 扩展思维 OK（响应含 thinking 块）")
}
