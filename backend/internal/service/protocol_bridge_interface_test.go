package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

// Phase 3 P3-4 interface 抽离单测。

// fakeBridge 用于测试 ProtocolBridge interface 的最小实现。
type fakeBridge struct {
	meta BridgeMetadata
}

func (f *fakeBridge) Metadata() BridgeMetadata { return f.meta }
func (f *fakeBridge) Forward(_ context.Context, _ *BridgePayload) (*BridgeResult, error) {
	return &BridgeResult{Body: []byte("ok")}, nil
}

func TestRegisterBridge_AcceptsValidImplementation(t *testing.T) {
	ResetBridgeImpls()

	b := &fakeBridge{
		meta: BridgeMetadata{
			ID:               BridgeID(domain.ProtocolGeminiV1Beta, domain.ProtocolOpenAIChat),
			InboundProtocol:  domain.ProtocolGeminiV1Beta,
			OutboundProtocol: domain.ProtocolOpenAIChat,
			Implementation:   "fakeBridge",
		},
	}
	require.NoError(t, RegisterBridge(b))

	got, ok := LookupBridge(b.meta.ID)
	require.True(t, ok)
	require.Equal(t, b.meta.ID, got.Metadata().ID)
}

func TestRegisterBridge_RejectsInvalidProtocol(t *testing.T) {
	ResetBridgeImpls()

	b := &fakeBridge{
		meta: BridgeMetadata{
			ID:               "openai->anthropic",
			InboundProtocol:  "openai", // 短名，违反 §1.1 决议 #7
			OutboundProtocol: domain.ProtocolAnthropicMessages,
		},
	}
	require.Error(t, RegisterBridge(b))
}

func TestRegisterBridge_RejectsDuplicate(t *testing.T) {
	ResetBridgeImpls()

	b := &fakeBridge{
		meta: BridgeMetadata{
			ID:               BridgeID(domain.ProtocolGeminiV1Beta, domain.ProtocolOpenAIChat),
			InboundProtocol:  domain.ProtocolGeminiV1Beta,
			OutboundProtocol: domain.ProtocolOpenAIChat,
		},
	}
	require.NoError(t, RegisterBridge(b))
	require.Error(t, RegisterBridge(b), "重复 ID 应被拒绝")
}

func TestRegisterBridge_RejectsNil(t *testing.T) {
	ResetBridgeImpls()
	require.Error(t, RegisterBridge(nil))
}

func TestLookupBridge_MissReturnsFalse(t *testing.T) {
	ResetBridgeImpls()
	_, ok := LookupBridge("nonexistent")
	require.False(t, ok)
}

func TestProtocolBridge_ForwardSignature(t *testing.T) {
	ResetBridgeImpls()
	b := &fakeBridge{
		meta: BridgeMetadata{
			ID:               BridgeID(domain.ProtocolGeminiV1Beta, domain.ProtocolOpenAIChat),
			InboundProtocol:  domain.ProtocolGeminiV1Beta,
			OutboundProtocol: domain.ProtocolOpenAIChat,
		},
	}
	require.NoError(t, RegisterBridge(b))

	result, err := b.Forward(context.Background(), &BridgePayload{
		Body:   []byte("{}"),
		Model:  "test",
		Stream: false,
	})
	require.NoError(t, err)
	require.Equal(t, []byte("ok"), result.Body)
}

// TestMetadataAndImplsParallel 验证元数据 Registry 与 interface 实现 Registry 是两套独立索引：
//   - 元数据 Registry 可保留 fork 旧桥（不需要 interface 实现）
//   - interface Registry 只装新桥
func TestMetadataAndImplsParallel(t *testing.T) {
	ResetBridgeImpls()

	// 元数据 Registry：fork 启动时注册 3 条（含 stub）
	meta := NewProtocolBridgeRegistry()
	require.Equal(t, 3, meta.Count())

	// interface 实现 Registry：暂时为空（fork 旧桥未迁移）
	_, ok := LookupBridge(BridgeID(domain.ProtocolAnthropicMessages, domain.ProtocolOpenAIResponses))
	require.False(t, ok, "fork 已有桥暂未迁移到 interface 实现")

	// 新桥注册到 interface 实现后才能 Forward
	b := &fakeBridge{meta: BridgeMetadata{
		ID:               BridgeID(domain.ProtocolGeminiV1Beta, domain.ProtocolOpenAIChat),
		InboundProtocol:  domain.ProtocolGeminiV1Beta,
		OutboundProtocol: domain.ProtocolOpenAIChat,
		Implementation:   "fakeBridge",
	}}
	require.NoError(t, RegisterBridge(b))
	_, ok = LookupBridge(b.meta.ID)
	require.True(t, ok)
}
