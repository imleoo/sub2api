// Phase 3 P3-5：openai_chat → anthropic_messages 桥占位（Stub）。
//
// 实现状态：Stub（占位）
//   - Registry 注册：让桥矩阵在 P3-6 BridgeCapabilities 编排时可见
//   - 完整实现路线（docs/relay-architecture-design.md §4.3 P3-5）：
//     请求方向：ChatCompletionsRequest → ChatCompletionsToResponses（已有）→
//               ResponsesToAnthropic 请求形态（fork apicompat 已有 ResponsesToAnthropic 是响应方向，
//               需写请求方向 responses_to_anthropic_request.go - **该文件已存在**）
//     响应方向：AnthropicResponse → AnthropicToResponsesResponse → ResponsesToChatCompletions
//     流式：    Anthropic SSE → Responses event → ChatCompletions chunk
//
// 用户端通过 /v1/chat/completions 访问 Anthropic 上游账号（Anthropic Messages 协议）。
package apicompat

import "errors"

// ErrChatToAnthropicNotImplemented 桥尚未实现的明确信号；完整 PR 合并后删除。
var ErrChatToAnthropicNotImplemented = errors.New(
	"openai_chat->anthropic_messages bridge not yet implemented (Phase 3 P3-5 stub)",
)

// ForwardChatAsAnthropic Stub：调用即返回 ErrChatToAnthropicNotImplemented。
//
// 签名预留：调用方拿到 *ChatCompletionsRequest + outbound endpoint，
// 返回 *ChatCompletionsResponse（已从 Anthropic 上游转换回 chat 形态）或错误。
func ForwardChatAsAnthropic(_ *ChatCompletionsRequest) (*ChatCompletionsResponse, error) {
	return nil, ErrChatToAnthropicNotImplemented
}
