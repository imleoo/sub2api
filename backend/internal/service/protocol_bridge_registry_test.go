package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

// Phase 3 P3-1 Bridge Registry 单测。

func TestNewProtocolBridgeRegistry_RegistersForkBridges(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	// Phase 4 P4-1 后桥数 = 7：fork 2 条 + P3-3 stub + P3-5 双 stub + P4-1 gemini 双 stub
	require.Equal(t, 7, r.Count(), "fork 2 桥 + P3-3 stub + P3-5 双 stub + P4-1 gemini 双 stub")

	// Bridge #1
	b1, ok := r.Lookup(domain.ProtocolAnthropicMessages, domain.ProtocolOpenAIResponses)
	require.True(t, ok)
	require.Equal(t, "anthropic_messages->openai_responses", b1.ID)
	require.Contains(t, b1.Implementation, "ForwardAsAnthropic")

	// Bridge #2
	b2, ok := r.Lookup(domain.ProtocolOpenAIResponses, domain.ProtocolAnthropicMessages)
	require.True(t, ok)
	require.Equal(t, "openai_responses->anthropic_messages", b2.ID)
	require.Contains(t, b2.Implementation, "ForwardAsResponses")

	// Bridge #3 (P3-3 stub)
	b3, ok := r.Lookup(domain.ProtocolAnthropicMessages, domain.ProtocolOpenAIChat)
	require.True(t, ok)
	require.Equal(t, "anthropic_messages->openai_chat", b3.ID)
	require.Contains(t, b3.Implementation, "stub")
}

func TestBridgeID_Format(t *testing.T) {
	require.Equal(t, "anthropic_messages->openai_responses",
		BridgeID(domain.ProtocolAnthropicMessages, domain.ProtocolOpenAIResponses))
}

func TestRegistry_LookupMissReturnsFalse(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	// gemini_v1beta->openai_responses 桥未在任何 Phase 注册
	_, ok := r.Lookup(domain.ProtocolGeminiV1Beta, domain.ProtocolOpenAIResponses)
	require.False(t, ok, "gemini_v1beta->openai_responses 桥未注册")
}

func TestRegistry_GeminiBridgesRegistered(t *testing.T) {
	r := NewProtocolBridgeRegistry()

	// Bridge #6 (P4-1 stub): gemini_v1beta → openai_chat
	b6, ok := r.Lookup(domain.ProtocolGeminiV1Beta, domain.ProtocolOpenAIChat)
	require.True(t, ok)
	require.Equal(t, "gemini_v1beta->openai_chat", b6.ID)
	require.Contains(t, b6.Implementation, "stub")

	// Bridge #7 (P4-1 stub): gemini_v1beta → anthropic_messages
	b7, ok := r.Lookup(domain.ProtocolGeminiV1Beta, domain.ProtocolAnthropicMessages)
	require.True(t, ok)
	require.Equal(t, "gemini_v1beta->anthropic_messages", b7.ID)
	require.Contains(t, b7.Implementation, "stub")
}

func TestRegistry_RegisterRejectsInvalidProtocol(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	err := r.Register(BridgeMetadata{
		ID:               "openai->anthropic",
		InboundProtocol:  "openai", // 短名，违反 glossary §1.1 决议 #7
		OutboundProtocol: domain.ProtocolAnthropicMessages,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid inbound protocol")
}

func TestRegistry_RegisterRejectsDuplicate(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	err := r.Register(BridgeMetadata{
		ID:               BridgeID(domain.ProtocolAnthropicMessages, domain.ProtocolOpenAIResponses),
		InboundProtocol:  domain.ProtocolAnthropicMessages,
		OutboundProtocol: domain.ProtocolOpenAIResponses,
		Implementation:   "dup",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

func TestRegistry_RegisterEmptyID(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	err := r.Register(BridgeMetadata{
		ID:               "",
		InboundProtocol:  domain.ProtocolAnthropicMessages,
		OutboundProtocol: domain.ProtocolOpenAIResponses,
	})
	require.Error(t, err)
}

func TestRegistry_ListReturnsSorted(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	list := r.List()
	require.Len(t, list, 7) // P4-1 后 7 桥
	// ID 字典序
	require.Equal(t, "anthropic_messages->openai_chat", list[0].ID)
	require.Equal(t, "anthropic_messages->openai_responses", list[1].ID)
	require.Equal(t, "gemini_v1beta->anthropic_messages", list[2].ID)
	require.Equal(t, "gemini_v1beta->openai_chat", list[3].ID)
	require.Equal(t, "openai_chat->anthropic_messages", list[4].ID)
	require.Equal(t, "openai_responses->anthropic_messages", list[5].ID)
	require.Equal(t, "openai_responses->openai_chat", list[6].ID)
}

func TestRegistry_LookupByID(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	meta, ok := r.LookupByID("openai_responses->anthropic_messages")
	require.True(t, ok)
	require.Equal(t, domain.ProtocolOpenAIResponses, meta.InboundProtocol)

	_, ok = r.LookupByID("nonexistent")
	require.False(t, ok)
}

func TestRegistry_NilReceiverSafe(t *testing.T) {
	var r *ProtocolBridgeRegistry
	require.Equal(t, 0, r.Count())
	_, ok := r.Lookup("a", "b")
	require.False(t, ok)
	require.Nil(t, r.List())
}
