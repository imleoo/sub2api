// Phase 3 P3-3：anthropic_messages → openai_chat 桥占位（Stub）。
//
// 实现状态：**Stub / 占位**
//   - 当前桥仅完成 Registry 元数据注册（让桥数=3 解锁 P3-4 interface 抽离）
//   - 实际请求/响应转换 + 流式 SSE 留独立 PR（plan §11 标 3 人天，超出当前 batch 范围）
//   - 调用方目前不应路由到此桥；P3-6 BridgeCapabilities 应标 FitUnknown
//
// 完整实现路线（按 docs/relay-architecture-design.md §4.3 P3-3 验收）：
//
//	请求方向：解析 ResponsesRequest.Input (json.RawMessage 形态) → 提取 messages →
//	         构造 ChatCompletionsRequest（tools / system / multi-turn / vision placeholder）
//	响应方向：ChatCompletionsResponse → 重组 ResponsesResponse → ResponsesToAnthropic
//	流式：    ChatCompletions SSE chunk → ResponsesEvent → AnthropicStreamEvent
//	         （状态机参考 fork 已有 ResponsesEventToAnthropic）
//
// 关联：
//   - service.NewProtocolBridgeRegistry 会在 init 时把本桥的元数据注册进去
//     （由 P3-3 follow-up commit 完成）
package apicompat

import "errors"

// ErrAnthropicToChatNotImplemented 桥尚未实现的明确信号；完整 PR 合并后删除此变量。
var ErrAnthropicToChatNotImplemented = errors.New(
	"anthropic_messages->openai_chat bridge not yet implemented (Phase 3 P3-3 stub; " +
		"see docs/relay-architecture-design.md §4.3 for the full implementation plan)",
)

// ForwardAnthropicAsChatCompletions Stub 实现，运行时调用即返回 ErrAnthropicToChatNotImplemented。
//
// 签名预留：调用方拿到 *AnthropicRequest + outbound endpoint，返回 *AnthropicResponse 或错误。
// 完整实现见上方 "完整实现路线"。
func ForwardAnthropicAsChatCompletions(_ *AnthropicRequest) (*AnthropicResponse, error) {
	return nil, ErrAnthropicToChatNotImplemented
}
