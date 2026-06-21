//go:build unit

// SSOT 重构 PR-3：Bootstrap seed 内容回归测试。
//
// 用途：固化 BootstrapPricingSeeds 的内容与 billing_service_golden_test.go 黄金值匹配；
// 任何 seed 字面价格漂移 → 这里失败 → 阻止合并。
package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBootstrapPricingSeeds_HasAllExpectedModels(t *testing.T) {
	seeds := BootstrapPricingSeeds()
	got := make(map[string]*DBModelPricing, len(seeds))
	for _, s := range seeds {
		got[s.ModelID] = s
	}

	// 通用化后：MaaS 平台价（含灵境豆包）不再内置，seed 仅保留 16 条 Claude/GPT/Gemini 基础兜底。
	expected := []string{
		// Anthropic Claude
		"claude-opus-4.5", "claude-opus-4.6", "claude-opus-4.7",
		"claude-sonnet-4", "claude-3-5-sonnet",
		"claude-3-5-haiku", "claude-3-opus", "claude-3-haiku",
		// Google Gemini
		"gemini-3.1-pro",
		// OpenAI
		"gpt-5.4", "gpt-5.5", "gpt-5.4-mini", "gpt-5.4-nano",
		"gpt-5.2", "gpt-5.3-codex", "gpt-5.3-codex-spark",
	}
	require.Len(t, seeds, len(expected), "seed 数量必须与基础兜底总和一致")
	for _, modelID := range expected {
		_, ok := got[modelID]
		require.True(t, ok, "seed 缺失 model_id=%s", modelID)
	}
}

// TestBootstrapPricingSeeds_PricingMatchesBillingFallback 验证 seed 价格与
// billing_service.go fallbackPrices 字面值 1:1 一致 —— 这是 PR-6 切流后
// catalog 路径与现有 fallback 路径计费数字保持不变的硬保证。
func TestBootstrapPricingSeeds_PricingMatchesBillingFallback(t *testing.T) {
	seeds := BootstrapPricingSeeds()
	byID := make(map[string]*DBModelPricing, len(seeds))
	for _, s := range seeds {
		byID[s.ModelID] = s
	}

	cases := []struct {
		modelID                        string
		wantInput, wantOutput          float64
		wantCacheCreate, wantCacheRead float64
		// optional priority / long-context
		wantInputPrio        *float64
		wantOutputPrio       *float64
		wantCacheReadPrio    *float64
		wantLongCtxThreshold *int64
		wantLongCtxInputMul  *float64
		wantLongCtxOutputMul *float64
	}{
		// 与 billing_service.go:142-260 字面值一致
		{modelID: "claude-opus-4.5", wantInput: 5e-6, wantOutput: 25e-6, wantCacheCreate: 6.25e-6, wantCacheRead: 0.5e-6},
		{modelID: "claude-sonnet-4", wantInput: 3e-6, wantOutput: 15e-6, wantCacheCreate: 3.75e-6, wantCacheRead: 0.3e-6},
		{modelID: "claude-3-5-sonnet", wantInput: 3e-6, wantOutput: 15e-6, wantCacheCreate: 3.75e-6, wantCacheRead: 0.3e-6},
		{modelID: "claude-3-5-haiku", wantInput: 1e-6, wantOutput: 5e-6, wantCacheCreate: 1.25e-6, wantCacheRead: 0.1e-6},
		{modelID: "claude-3-opus", wantInput: 15e-6, wantOutput: 75e-6, wantCacheCreate: 18.75e-6, wantCacheRead: 1.5e-6},
		{modelID: "claude-3-haiku", wantInput: 0.25e-6, wantOutput: 1.25e-6, wantCacheCreate: 0.3e-6, wantCacheRead: 0.03e-6},
		{modelID: "claude-opus-4.6", wantInput: 5e-6, wantOutput: 25e-6, wantCacheCreate: 6.25e-6, wantCacheRead: 0.5e-6},
		{modelID: "claude-opus-4.7", wantInput: 5e-6, wantOutput: 25e-6, wantCacheCreate: 6.25e-6, wantCacheRead: 0.5e-6},
		{modelID: "gemini-3.1-pro", wantInput: 2e-6, wantOutput: 12e-6, wantCacheCreate: 2e-6, wantCacheRead: 0.2e-6},
		{
			modelID: "gpt-5.4", wantInput: 2.5e-6, wantOutput: 15e-6,
			wantCacheCreate: 2.5e-6, wantCacheRead: 0.25e-6,
			wantInputPrio: bootstrapFloatPtr(5e-6), wantOutputPrio: bootstrapFloatPtr(30e-6),
			wantCacheReadPrio:    bootstrapFloatPtr(0.5e-6),
			wantLongCtxThreshold: bootstrapInt64Ptr(272000),
			wantLongCtxInputMul:  bootstrapFloatPtr(2.0),
			wantLongCtxOutputMul: bootstrapFloatPtr(1.5),
		},
		{
			modelID: "gpt-5.5", wantInput: 2.5e-6, wantOutput: 15e-6,
			wantCacheCreate: 2.5e-6, wantCacheRead: 0.25e-6,
			wantInputPrio: bootstrapFloatPtr(5e-6), wantOutputPrio: bootstrapFloatPtr(30e-6),
			wantCacheReadPrio:    bootstrapFloatPtr(0.5e-6),
			wantLongCtxThreshold: bootstrapInt64Ptr(272000),
			wantLongCtxInputMul:  bootstrapFloatPtr(2.0),
			wantLongCtxOutputMul: bootstrapFloatPtr(1.5),
		},
		{modelID: "gpt-5.4-mini", wantInput: 7.5e-7, wantOutput: 4.5e-6, wantCacheRead: 7.5e-8},
		{modelID: "gpt-5.4-nano", wantInput: 2e-7, wantOutput: 1.25e-6, wantCacheRead: 2e-8},
		{
			modelID: "gpt-5.2", wantInput: 1.75e-6, wantOutput: 14e-6,
			wantCacheCreate: 1.75e-6, wantCacheRead: 0.175e-6,
			wantInputPrio: bootstrapFloatPtr(3.5e-6), wantOutputPrio: bootstrapFloatPtr(28e-6),
			wantCacheReadPrio: bootstrapFloatPtr(0.35e-6),
		},
		{
			modelID: "gpt-5.3-codex", wantInput: 1.5e-6, wantOutput: 12e-6,
			wantCacheCreate: 1.5e-6, wantCacheRead: 0.15e-6,
			wantInputPrio: bootstrapFloatPtr(3e-6), wantOutputPrio: bootstrapFloatPtr(24e-6),
			wantCacheReadPrio: bootstrapFloatPtr(0.3e-6),
		},
	}

	for _, tc := range cases {
		t.Run(tc.modelID, func(t *testing.T) {
			m, ok := byID[tc.modelID]
			require.True(t, ok, "seed missing %s", tc.modelID)
			require.NotNil(t, m.InputCostPerToken)
			require.InDelta(t, tc.wantInput, *m.InputCostPerToken, 1e-12, "InputCost")
			require.NotNil(t, m.OutputCostPerToken)
			require.InDelta(t, tc.wantOutput, *m.OutputCostPerToken, 1e-12, "OutputCost")
			if tc.wantCacheCreate > 0 {
				require.NotNil(t, m.CacheCreationInputTokenCost)
				require.InDelta(t, tc.wantCacheCreate, *m.CacheCreationInputTokenCost, 1e-12, "CacheCreation")
			}
			if tc.wantCacheRead > 0 {
				require.NotNil(t, m.CacheReadInputTokenCost)
				require.InDelta(t, tc.wantCacheRead, *m.CacheReadInputTokenCost, 1e-12, "CacheRead")
			}
			if tc.wantInputPrio != nil {
				require.NotNil(t, m.InputCostPerTokenPriority)
				require.InDelta(t, *tc.wantInputPrio, *m.InputCostPerTokenPriority, 1e-12, "InputPriority")
			}
			if tc.wantOutputPrio != nil {
				require.NotNil(t, m.OutputCostPerTokenPriority)
				require.InDelta(t, *tc.wantOutputPrio, *m.OutputCostPerTokenPriority, 1e-12, "OutputPriority")
			}
			if tc.wantCacheReadPrio != nil {
				require.NotNil(t, m.CacheReadInputTokenCostPriority)
				require.InDelta(t, *tc.wantCacheReadPrio, *m.CacheReadInputTokenCostPriority, 1e-12, "CacheReadPriority")
			}
			if tc.wantLongCtxThreshold != nil {
				require.NotNil(t, m.LongContextInputTokenThreshold)
				require.Equal(t, *tc.wantLongCtxThreshold, *m.LongContextInputTokenThreshold, "LongCtxThreshold")
			}
			if tc.wantLongCtxInputMul != nil {
				require.NotNil(t, m.LongContextInputCostMultiplier)
				require.InDelta(t, *tc.wantLongCtxInputMul, *m.LongContextInputCostMultiplier, 1e-12, "LongCtxInputMul")
			}
			if tc.wantLongCtxOutputMul != nil {
				require.NotNil(t, m.LongContextOutputCostMultiplier)
				require.InDelta(t, *tc.wantLongCtxOutputMul, *m.LongContextOutputCostMultiplier, 1e-12, "LongCtxOutputMul")
			}

			require.Equal(t, ModelPricingStatusPriced, m.PricingStatus, "pricing_status")
			require.True(t, m.IsEnabled, "is_enabled")
			if m.Provider == "lingjing" {
				require.Equal(t, ModelPricingSourceLingjing, m.Source, "source=lingjing")
			} else {
				require.Equal(t, bootstrapSourceLabel, m.Source, "source=bootstrap")
			}
		})
	}
}

// TestBootstrapPricingSeeds_LingjingFields 灵境模型仅设 image / image_token 价格，
// 不应误填 input/output token 价格。
func TestBootstrapPricingSeeds_LingjingFields(t *testing.T) {
	seeds := BootstrapPricingSeeds()
	for _, s := range seeds {
		if s.Provider != "lingjing" {
			continue
		}
		require.Nil(t, s.InputCostPerToken, "灵境 %s 不应有 input token 价格", s.ModelID)
		require.Nil(t, s.OutputCostPerToken, "灵境 %s 不应有 output token 价格", s.ModelID)
		if s.Mode == "image_generation" {
			require.NotNil(t, s.OutputCostPerImage, "灵境 %s image_generation 必须有 image 价格", s.ModelID)
		}
		if s.Mode == "video_generation" {
			// seedance 已迁移到按秒计费：单价存 output_cost_per_image（schema 允许 per image / per second 复用）。
			require.NotNil(t, s.OutputCostPerImage, "灵境 %s video_generation 必须有 per-second 价格", s.ModelID)
		}
	}
}
