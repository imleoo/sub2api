package domain

// Protocol constants (Phase 2 P2-2 引入).
//
// 取值与 docs/glossary.md §1.1 一致——单一权威源；新增 protocol 必须先改 glossary.md，
// 再加这里。废弃所有短名写法（"openai"/"anthropic"/"gemini"），改用长名。
//
// Bridge ID 命名规则：<inbound>-><outbound>，如 "anthropic_messages->openai_responses"。
const (
	ProtocolAnthropicMessages = "anthropic_messages"
	ProtocolOpenAIChat        = "openai_chat"
	ProtocolOpenAIResponses   = "openai_responses"
	ProtocolGeminiV1Beta      = "gemini_v1beta"
)

// AllProtocols 返回所有合法 protocol 取值（按 §1.1 顺序）。
func AllProtocols() []string {
	return []string{
		ProtocolAnthropicMessages,
		ProtocolOpenAIChat,
		ProtocolOpenAIResponses,
		ProtocolGeminiV1Beta,
	}
}

// IsValidProtocol 判断给定字符串是否是合法的 protocol 长名。
//
// 注意：空字符串视为"未指定"，由 ResolveInboundProtocol / ResolveOutboundProtocol
// 按 platform 派生，因此本函数对空串返回 false（强约束）。
func IsValidProtocol(s string) bool {
	switch s {
	case ProtocolAnthropicMessages, ProtocolOpenAIChat, ProtocolOpenAIResponses, ProtocolGeminiV1Beta:
		return true
	}
	return false
}

// platformDefaultInboundProtocol 是 platform → 默认 inbound_protocol 的派生表（Phase 2 兼容层）。
//
// 用于 Group.inbound_protocol 为空时按 Group.platform 推断。这是 Phase 2 双写期的
// 兼容兜底；Phase 5 切单桶后将废弃。
var platformDefaultInboundProtocol = map[string]string{
	PlatformAnthropic: ProtocolAnthropicMessages,
	PlatformOpenAI:    ProtocolOpenAIChat,
	PlatformGemini:    ProtocolGeminiV1Beta,
	// lingjing 不走通用 inbound 路由（自带 /lingjing/v1/video/* 专用路由组），保持空
}

// ResolveInboundProtocol 派生 Group 的 inbound_protocol：
//
//  1. 若 inboundProtocol 非空且合法 → 返回原值
//  2. 否则按 platform 查派生表
//  3. 派生表未命中 → 返回空串（调用方应回退到旧 platform 路由分流）
//
// Deprecated for future Phase 5: 双写期临时函数；Phase 5 切单桶后应直接读 inbound_protocol。
func ResolveInboundProtocol(inboundProtocol, platform string) string {
	if IsValidProtocol(inboundProtocol) {
		return inboundProtocol
	}
	return platformDefaultInboundProtocol[platform]
}

// platformDefaultOutboundProtocol 是 platform → 默认 outbound_protocol 的派生表。
//
// 注意：OpenAI 账号实际上 outbound 既可以是 openai_chat 又可以是 openai_responses，
// 这里只给出"无能力探测时"的默认值（openai_chat 最保守）。能力探测在 Phase 3 P3-6 落地。
var platformDefaultOutboundProtocol = map[string]string{
	PlatformAnthropic: ProtocolAnthropicMessages,
	PlatformOpenAI:    ProtocolOpenAIChat,
	PlatformGemini:    ProtocolGeminiV1Beta,
}

// ResolveOutboundProtocol 派生 Account 的 outbound_protocol（语义同 ResolveInboundProtocol）。
//
// Deprecated for future Phase 5: 双写期临时函数。
func ResolveOutboundProtocol(outboundProtocol, platform string) string {
	if IsValidProtocol(outboundProtocol) {
		return outboundProtocol
	}
	return platformDefaultOutboundProtocol[platform]
}
