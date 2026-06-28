package conversecompat

import (
	"encoding/json"

	"github.com/tidwall/gjson"
)

// stream.go —— native Anthropic SSE 事件流 → Bedrock Converse 二进制事件流。
//
// native 序列：message_start → content_block_start → content_block_delta×N
//              → content_block_stop → message_delta → message_stop
// Converse 序列：messageStart → contentBlockStart → contentBlockDelta×N
//              → contentBlockStop → messageStop → metadata(usage)
//
// Converse 把全部 usage 挪到末尾单个 metadata 帧（§11.3）；StreamConverter 跨事件缓存
// input/output/cache 标量，由 Finish() 合成 metadata 帧。每个流一个 StreamConverter 实例。

// StreamConverter 持有单个流的 usage 缓存（非并发安全，每流一个实例）。
type StreamConverter struct {
	inputTokens      int
	outputTokens     int
	cacheReadTokens  int
	cacheWriteTokens int
}

// NewStreamConverter 创建一个流转换器。
func NewStreamConverter() *StreamConverter { return &StreamConverter{} }

// Convert 把一条 native 事件（eventType + data JSON）转为 Converse 二进制帧。
// 返回 nil 表示该事件无对应输出（ping / message_stop / 未知）。
func (sc *StreamConverter) Convert(eventType string, data []byte) []byte {
	switch eventType {
	case "message_start":
		if u := gjson.GetBytes(data, "message.usage"); u.Exists() {
			sc.inputTokens = int(u.Get("input_tokens").Int())
			sc.cacheReadTokens = int(u.Get("cache_read_input_tokens").Int())
			sc.cacheWriteTokens = int(u.Get("cache_creation_input_tokens").Int())
		}
		role := gjson.GetBytes(data, "message.role").String()
		if role == "" {
			role = "assistant"
		}
		return EncodeEventStreamFrame("messageStart", mustJSONMarshal(map[string]any{"role": role}))

	case "content_block_start":
		idx := int(gjson.GetBytes(data, "index").Int())
		cb := gjson.GetBytes(data, "content_block")
		payload := map[string]any{"contentBlockIndex": idx}
		switch cb.Get("type").String() {
		case "tool_use", "server_tool_use":
			payload["start"] = map[string]any{
				"toolUse": map[string]any{
					"toolUseId": cb.Get("id").String(), // 原样保留 toolu_bdrk_ 前缀
					"name":      cb.Get("name").String(),
				},
			}
		}
		// text/thinking 块的 contentBlockStart 无 start 内容。
		return EncodeEventStreamFrame("contentBlockStart", mustJSONMarshal(payload))

	case "content_block_delta":
		idx := int(gjson.GetBytes(data, "index").Int())
		converseDelta := mapStreamDelta(gjson.GetBytes(data, "delta"))
		if converseDelta == nil {
			return nil
		}
		return EncodeEventStreamFrame("contentBlockDelta", mustJSONMarshal(map[string]any{
			"contentBlockIndex": idx,
			"delta":             converseDelta,
		}))

	case "content_block_stop":
		idx := int(gjson.GetBytes(data, "index").Int())
		return EncodeEventStreamFrame("contentBlockStop", mustJSONMarshal(map[string]any{"contentBlockIndex": idx}))

	case "message_delta":
		if o := gjson.GetBytes(data, "usage.output_tokens"); o.Exists() {
			sc.outputTokens = int(o.Int())
		}
		sr := gjson.GetBytes(data, "delta.stop_reason").String()
		return EncodeEventStreamFrame("messageStop", mustJSONMarshal(map[string]any{"stopReason": mapStopReason(sr)}))

	default:
		// message_stop（Converse 末尾用 metadata 收尾）/ ping / 未知 → 跳过。
		return nil
	}
}

// Finish 合成并返回末尾 metadata 帧（Converse usage 只在此出现）。
func (sc *StreamConverter) Finish() []byte {
	return EncodeEventStreamFrame("metadata", mustJSONMarshal(map[string]any{
		"usage": map[string]any{
			"inputTokens":           sc.inputTokens,
			"outputTokens":          sc.outputTokens,
			"totalTokens":           sc.inputTokens + sc.outputTokens,
			"cacheReadInputTokens":  sc.cacheReadTokens,
			"cacheWriteInputTokens": sc.cacheWriteTokens,
		},
		"metrics": map[string]any{"latencyMs": 0},
	}))
}

// mapStreamDelta 把 native content_block_delta.delta 转为 Converse contentBlockDelta.delta。
func mapStreamDelta(d gjson.Result) map[string]any {
	switch d.Get("type").String() {
	case "text_delta":
		return map[string]any{"text": d.Get("text").String()}
	case "input_json_delta":
		// 工具入参以 String 分片增量送达（拼接由下游消费方负责）。
		return map[string]any{"toolUse": map[string]any{"input": d.Get("partial_json").String()}}
	case "thinking_delta":
		return map[string]any{"reasoningContent": map[string]any{"text": d.Get("thinking").String()}}
	case "signature_delta":
		return map[string]any{"reasoningContent": map[string]any{"signature": d.Get("signature").String()}}
	default:
		return nil
	}
}

func mustJSONMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
