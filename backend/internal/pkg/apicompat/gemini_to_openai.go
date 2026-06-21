// Phase 4 P4-1/P4-2：gemini_v1beta → openai_chat 桥占位（Stub）。
//
// 实现状态：Stub（占位）
//   - 完整实现路线（docs/relay-architecture-design.md §4.3 + sprint-plan §11）：
//     请求：GeminiGenerateContentRequest → ChatCompletionsRequest
//     contents[].parts → messages (role 映射 user/model→user/assistant)
//     tools[].functionDeclarations → tools[].function（JSON Schema 直透，零丢失）
//     systemInstruction → ChatMessage{role:"system"}
//     generationConfig → temperature / max_tokens / stop
//     响应：ChatCompletionsResponse → GeminiGenerateContentResponse
//     choices[0].message → candidates[0].content
//     finish_reason（stop/length/tool_calls）→ STOP/MAX_TOKENS/STOP
//     usage → usageMetadata
//     流式：streamGenerateContent SSE → ChatCompletions chunk
//     每条 Gemini `data: {...}` chunk → 对应一条 ChatCompletionsChunk
//     finishReason 在最后一条 chunk 写入
//   - function_calling schema 映射零丢失（GeminiFunctionDeclarations.Parameters 直透，不转换）
//
// 用户端通过 /v1beta/models/:model:generateContent 访问 DeepSeek/Kimi/Qwen 等 OpenAI 兼容上游。
package apicompat

import (
	"errors"
	"io"
)

// ErrGeminiToOpenAINotImplemented 桥尚未实现的明确信号。
var ErrGeminiToOpenAINotImplemented = errors.New(
	"gemini_v1beta->openai_chat bridge not yet implemented (Phase 4 P4-1 stub)",
)

// ForwardGeminiAsChat Stub（非流式）：请求 GeminiGenerateContentRequest → 上游 openai_chat → 响应 GeminiGenerateContentResponse。
//
// 调用即返回 ErrGeminiToOpenAINotImplemented；完整实现见上方"实现路线"。
func ForwardGeminiAsChat(_ *GeminiGenerateContentRequest) (*GeminiGenerateContentResponse, error) {
	return nil, ErrGeminiToOpenAINotImplemented
}

// ForwardGeminiStreamAsChat Stub（流式）：将 streamGenerateContent 请求转发到 openai_chat 上游，
// 并将 ChatCompletions SSE chunks 转换回 Gemini SSE 格式写入 w。
//
// 调用即返回 ErrGeminiToOpenAINotImplemented；完整实现负责：
//   - 上游 SSE chunk 逐帧读取（不整体 buffer）
//   - 每帧写入一条 `data: {...}\n\n`（Gemini streamGenerateContent 格式）
//   - 最后一帧追加 finishReason + usageMetadata
func ForwardGeminiStreamAsChat(_ *GeminiGenerateContentRequest, _ io.Writer) error {
	return ErrGeminiToOpenAINotImplemented
}
