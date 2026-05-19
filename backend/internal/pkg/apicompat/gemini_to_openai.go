// Phase 4 P4-1：gemini_v1beta → openai_chat 桥占位（Stub）。
//
// 实现状态：Stub（占位）
//   - 完整实现路线（docs/relay-architecture-design.md §4.3 + sprint-plan §11）：
//     请求：Gemini generateContent payload → ChatCompletionsRequest（messages/tools/system_instruction 映射）
//     响应：ChatCompletionsResponse → Gemini generateContentResponse（candidates 数组 + finishReason）
//     流式：streamGenerateContent SSE → ChatCompletions chunk
//   - function_calling schema 映射零丢失（Gemini functionDeclarations ↔ OpenAI tools）
//
// 用户端通过 /v1beta/models/:model:generateContent 访问 DeepSeek/Kimi/Qwen 等 OpenAI 兼容上游。
package apicompat

import "errors"

// ErrGeminiToOpenAINotImplemented 桥尚未实现的明确信号。
var ErrGeminiToOpenAINotImplemented = errors.New(
	"gemini_v1beta->openai_chat bridge not yet implemented (Phase 4 P4-1 stub)",
)

// ForwardGeminiAsChat Stub：调用即返回 ErrGeminiToOpenAINotImplemented。
func ForwardGeminiAsChat(_ []byte) ([]byte, error) {
	return nil, ErrGeminiToOpenAINotImplemented
}
