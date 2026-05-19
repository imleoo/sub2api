package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

// Phase 3 P3-1 Bridge Registry 单测。

func TestNewProtocolBridgeRegistry_RegistersForkBridges(t *testing.T) {
	r := NewProtocolBridgeRegistry()
	// Phase 3 P3-3 后桥数 = 3：fork 2 条 + anthropic→chat stub
	require.Equal(t, 3, r.Count(), "fork 2 桥（forward as anthropic / forward as responses）+ P3-3 anthropic→chat stub")

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
	_, ok := r.Lookup(domain.ProtocolGeminiV1Beta, domain.ProtocolAnthropicMessages)
	require.False(t, ok, "Gemini→Anthropic 桥在 Phase 4 才注册")
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
	require.Len(t, list, 3) // P3-3 后 3 桥
	// ID 字典序：anthropic_messages->openai_chat / openai_responses / openai_responses->anthropic_messages
	require.Equal(t, "anthropic_messages->openai_chat", list[0].ID)
	require.Equal(t, "anthropic_messages->openai_responses", list[1].ID)
	require.Equal(t, "openai_responses->anthropic_messages", list[2].ID)
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
