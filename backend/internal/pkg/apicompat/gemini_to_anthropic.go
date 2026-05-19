// Phase 4 P4-1：gemini_v1beta → anthropic_messages 桥占位（Stub）。
//
// 实现状态：Stub（占位）
//   - 完整实现路线（docs/relay-architecture-design.md §4.3 + sprint-plan §11）：
//     请求：Gemini generateContent payload → AnthropicMessagesRequest（role/content/tool_use 映射）
//     响应：AnthropicMessagesResponse → Gemini generateContentResponse（candidates 数组 + finishReason）
//     流式：streamGenerateContent SSE → Anthropic stream_event
//   - function_calling：Gemini functionDeclarations ↔ Anthropic tools 双向映射
//   - system_instruction → Anthropic system 字段（single-turn 场景直接提升）
//
// 用户端通过 /v1beta/models/:model:generateContent 访问 Claude 等 Anthropic 上游。
package apicompat

import "errors"

// ErrGeminiToAnthropicNotImplemented 桥尚未实现的明确信号。
var ErrGeminiToAnthropicNotImplemented = errors.New(
	"gemini_v1beta->anthropic_messages bridge not yet implemented (Phase 4 P4-1 stub)",
)

// ForwardGeminiAsAnthropic Stub：调用即返回 ErrGeminiToAnthropicNotImplemented。
func ForwardGeminiAsAnthropic(_ []byte) ([]byte, error) {
	return nil, ErrGeminiToAnthropicNotImplemented
}
