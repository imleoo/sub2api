package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase 3 P3-6 RequestFeatures + ScoreFit 单测。

func TestSniffAnthropic_PureText(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]
	}`)
	f := SniffAnthropic(body)
	require.False(t, f.HasRequired(FeatureVision))
	require.False(t, f.HasRequired(FeatureDocument))
	require.False(t, f.HasRequired(FeatureToolUse))
	require.False(t, f.HasRequired(FeatureStreaming))
}

func TestSniffAnthropic_DetectsStreaming(t *testing.T) {
	body := []byte(`{"stream": true, "messages": []}`)
	f := SniffAnthropic(body)
	require.True(t, f.HasRequired(FeatureStreaming))
}

func TestSniffAnthropic_DetectsVision(t *testing.T) {
	body := []byte(`{
		"messages": [{"role": "user", "content": [
			{"type": "text", "text": "describe"},
			{"type": "image", "source": {"data": "..."}}
		]}]
	}`)
	f := SniffAnthropic(body)
	require.True(t, f.HasRequired(FeatureVision))
}

func TestSniffAnthropic_DetectsDocument(t *testing.T) {
	body := []byte(`{
		"messages": [{"role": "user", "content": [{"type": "document", "source": {}}]}]
	}`)
	f := SniffAnthropic(body)
	require.True(t, f.HasRequired(FeatureDocument))
}

func TestSniffAnthropic_DetectsCacheControl(t *testing.T) {
	body := []byte(`{
		"system": [{"type": "text", "text": "...", "cache_control": {"type": "ephemeral"}}],
		"messages": []
	}`)
	f := SniffAnthropic(body)
	require.True(t, f.HasRequired(FeatureCacheControl))
}

func TestSniffAnthropic_DetectsExtendedThinking(t *testing.T) {
	body := []byte(`{"thinking": {"type": "enabled"}, "messages": []}`)
	f := SniffAnthropic(body)
	require.True(t, f.HasRequired(FeatureExtendedThinking))
}

func TestSniffAnthropic_DetectsToolUse(t *testing.T) {
	body := []byte(`{
		"tools": [{"name": "search", "description": "..."}],
		"messages": []
	}`)
	f := SniffAnthropic(body)
	require.True(t, f.HasRequired(FeatureToolUse))
	require.False(t, f.HasRequired(FeatureComputerUse))
}

func TestSniffAnthropic_DetectsComputerUse(t *testing.T) {
	body := []byte(`{
		"tools": [{"name": "computer_use", "type": "computer_20241022"}],
		"messages": []
	}`)
	f := SniffAnthropic(body)
	require.True(t, f.HasRequired(FeatureComputerUse))
	// computer_use 优先于 tool_use（同一 tools 数组只标更具体的）
	require.False(t, f.HasRequired(FeatureToolUse))
}

func TestSniffAnthropic_InvalidJSON(t *testing.T) {
	f := SniffAnthropic([]byte("not json"))
	require.NotNil(t, f)
	require.Empty(t, f.Required, "无效 JSON 不应崩溃；返回空 features")
}

func TestSniffAnthropic_NilBody(t *testing.T) {
	f := SniffAnthropic(nil)
	require.NotNil(t, f)
	require.Empty(t, f.Required)
}

// --- ScoreFit ---

func TestScoreFit_RejectsHardReject(t *testing.T) {
	features := &RequestFeatures{Required: map[FeatureID]bool{FeatureDocument: true}}
	caps := &BridgeCapabilities{Levels: map[FeatureID]FeatureFit{
		FeatureDocument: FitRejected,
	}}
	require.False(t, ScoreFit(features, caps, nil))
}

func TestScoreFit_AllowsNative(t *testing.T) {
	features := &RequestFeatures{Required: map[FeatureID]bool{FeatureToolUse: true}}
	caps := &BridgeCapabilities{Levels: map[FeatureID]FeatureFit{
		FeatureToolUse: FitNative,
	}}
	require.True(t, ScoreFit(features, caps, nil))
}

func TestScoreFit_AllowsUnknownByDefault(t *testing.T) {
	// FitUnknown 默认乐观（让调度尝试）
	features := &RequestFeatures{Required: map[FeatureID]bool{FeatureToolUse: true}}
	caps := &BridgeCapabilities{} // 全空，所有特性都是 FitUnknown
	require.True(t, ScoreFit(features, caps, nil))
}

func TestScoreFit_DefaultRejectsDropped(t *testing.T) {
	features := &RequestFeatures{Required: map[FeatureID]bool{FeatureCacheControl: true}}
	caps := &BridgeCapabilities{Levels: map[FeatureID]FeatureFit{
		FeatureCacheControl: FitDropped,
	}}
	// 默认策略不允许 Dropped
	require.False(t, ScoreFit(features, caps, nil))
	// best_effort 策略允许
	require.True(t, ScoreFit(features, caps, &GroupFeaturePolicy{Strictness: "best_effort"}))
}

func TestScoreFit_StrictRejectsLossy(t *testing.T) {
	features := &RequestFeatures{Required: map[FeatureID]bool{FeatureExtendedThinking: true}}
	caps := &BridgeCapabilities{Levels: map[FeatureID]FeatureFit{
		FeatureExtendedThinking: FitLossy,
	}}
	require.True(t, ScoreFit(features, caps, nil)) // 默认允许 Lossy
	require.False(t, ScoreFit(features, caps, &GroupFeaturePolicy{Strictness: "strict"}))
}

func TestScoreFit_NativeOnlyFeaturesRejectLossy(t *testing.T) {
	features := &RequestFeatures{Required: map[FeatureID]bool{FeatureCitations: true}}
	caps := &BridgeCapabilities{Levels: map[FeatureID]FeatureFit{
		FeatureCitations: FitLossy,
	}}
	policy := &GroupFeaturePolicy{
		NativeOnlyFeatures: []FeatureID{FeatureCitations},
	}
	require.False(t, ScoreFit(features, caps, policy),
		"citations 在 NativeOnlyFeatures 中，即便策略允许 Lossy 也必须 Native")
}

func TestScoreFit_NilFeaturesAlwaysTrue(t *testing.T) {
	require.True(t, ScoreFit(nil, &BridgeCapabilities{}, nil))
}
