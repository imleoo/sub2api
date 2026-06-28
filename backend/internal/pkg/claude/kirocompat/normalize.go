// Package kirocompat 把 Kiro 上游响应归一为标准 Anthropic Messages 形态。
//
// 基于真实 Kiro 样本（openclaw kiro 分组，2026-06-28）分析，Kiro 响应已高度符合标准
// Anthropic，偏差仅两处：
//   - 非流式 tool_use 响应**缺 stop_reason**（标准应为 "tool_use"）→ 补齐
//   - usage 含非标准字段 inference_geo（Kiro/Vertex 特有）→ 移除
//
// 流式响应实测无 stop_reason 偏差（message_delta 正常带 stop_reason）；仅 usage 可能带
// inference_geo（多余字段，不破坏客户端解析），故流式归一为可选、低优先。
package kirocompat

import (
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NormalizeNonStream 把 Kiro 非流式响应归一为标准 Anthropic。
// 非 assistant message 响应（错误体等）原样返回。
func NormalizeNonStream(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	if gjson.GetBytes(body, "type").String() != "message" {
		return body
	}

	out := body

	// 1) 移除非标准 usage.inference_geo
	if gjson.GetBytes(out, "usage.inference_geo").Exists() {
		if deleted, err := sjson.DeleteBytes(out, "usage.inference_geo"); err == nil {
			out = deleted
		}
	}

	// 2) 补缺失的 stop_reason（Kiro tool_use 响应不带 stop_reason）
	if sr := gjson.GetBytes(out, "stop_reason"); !sr.Exists() || sr.Type == gjson.Null {
		reason := "end_turn"
		gjson.GetBytes(out, "content").ForEach(func(_, block gjson.Result) bool {
			if block.Get("type").String() == "tool_use" {
				reason = "tool_use"
				return false // 命中即停
			}
			return true
		})
		if set, err := sjson.SetBytes(out, "stop_reason", reason); err == nil {
			out = set
		}
	}

	return out
}
