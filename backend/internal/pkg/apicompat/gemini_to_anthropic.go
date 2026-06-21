// Phase 4 P4-1/P4-2：gemini_v1beta → anthropic_messages 桥占位（Stub）。
//
// 实现状态：Stub（占位）
//   - 完整实现路线（docs/relay-architecture-design.md §4.3 + sprint-plan §11）：
//     请求：GeminiGenerateContentRequest → AnthropicRequest
//     contents → messages（role 映射 user/model→user/assistant；parts→content blocks）
//     systemInstruction → system 字段（string 或 []ContentBlock）
//     tools[].functionDeclarations → tools（name/description/input_schema 直透）
//     generationConfig.{temperature,topP,maxOutputTokens,stopSequences} → 对应 Anthropic 字段
//     GeminiBlob(inlineData) → AnthropicContentBlock{type:"image",source:{type:"base64",...}}
//     响应：AnthropicResponse → GeminiGenerateContentResponse
//     content[].text → candidates[0].content.parts[0].text
//     content[].tool_use → candidates[0].content.parts[0].functionCall
//     stop_reason（end_turn/max_tokens/tool_use）→ STOP/MAX_TOKENS/STOP
//     usage.{input_tokens,output_tokens} → usageMetadata
//     流式：streamGenerateContent SSE → Anthropic stream_event → Gemini SSE chunk
//     Anthropic text_delta → 对应 GeminiCandidate parts[text] chunk
//     Anthropic tool_use 流式状态机 → functionCall chunk
//     message_stop → 最后 finishReason 帧
//   - function_calling：GeminiFunctionDeclarations.Parameters（JSON Schema）直透到
//     AnthropicTool.InputSchema，零字段丢失
//
// 用户端通过 /v1beta/models/:model:generateContent 访问 Anthropic Claude 上游。
package apicompat

import (
	"errors"
	"io"
)

// ErrGeminiToAnthropicNotImplemented 桥尚未实现的明确信号。
var ErrGeminiToAnthropicNotImplemented = errors.New(
	"gemini_v1beta->anthropic_messages bridge not yet implemented (Phase 4 P4-1 stub)",
)

// ForwardGeminiAsAnthropic Stub（非流式）：请求 GeminiGenerateContentRequest → 上游 anthropic_messages → 响应 GeminiGenerateContentResponse。
//
// 调用即返回 ErrGeminiToAnthropicNotImplemented；完整实现见上方"实现路线"。
func ForwardGeminiAsAnthropic(_ *GeminiGenerateContentRequest) (*GeminiGenerateContentResponse, error) {
	return nil, ErrGeminiToAnthropicNotImplemented
}

// ForwardGeminiStreamAsAnthropic Stub（流式）：将 streamGenerateContent 请求转发到 anthropic_messages 上游，
// 并将 Anthropic SSE 事件流转换回 Gemini SSE 格式写入 w。
//
// 调用即返回 ErrGeminiToAnthropicNotImplemented；完整实现负责：
//   - 上游 Anthropic text_delta / tool_use 事件逐帧读取
//   - 每帧转换为 GeminiGenerateContentResponse chunk 并写入 `data: {...}\n\n`
//   - message_stop 事件触发最后一帧（含 finishReason + usageMetadata）
func ForwardGeminiStreamAsAnthropic(_ *GeminiGenerateContentRequest, _ io.Writer) error {
	return ErrGeminiToAnthropicNotImplemented
}
