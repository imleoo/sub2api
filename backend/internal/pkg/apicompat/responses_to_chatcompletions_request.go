// Phase 3 P3-5：openai_responses → openai_chat 桥占位（同源协议互转）。
//
// 实现状态：Stub（占位）
//   - apicompat 现有 ResponsesToChatCompletions 是响应方向（resp → resp）
//   - 本桥需要补：请求方向 ResponsesRequest → ChatCompletionsRequest（与 chatcompletions_to_responses 反向）
//   - sprint-plan §11 标"同源协议互转，黏合工作量最低"——P3-5 实际实现工作量较小，
//     但完整需要 + 流式 SSE 桥，超出当前 batch 范围，留独立 PR
//
// 用户端通过 /v1/responses 访问只支持 Chat Completions 的上游账号。
package apicompat

import "errors"

// ErrResponsesToChatRequestNotImplemented 桥尚未实现的明确信号。
var ErrResponsesToChatRequestNotImplemented = errors.New(
	"openai_responses->openai_chat bridge not yet implemented (Phase 3 P3-5 stub)",
)

// ForwardResponsesAsChat Stub：调用即返回 ErrResponsesToChatRequestNotImplemented。
func ForwardResponsesAsChat(_ *ResponsesRequest) (*ResponsesResponse, error) {
	return nil, ErrResponsesToChatRequestNotImplemented
}
