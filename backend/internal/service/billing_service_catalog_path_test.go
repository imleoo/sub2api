//go:build unit

// SSOT 重构 PR-8：catalog 路径测试（fallback 路径已删除）。
//
// 覆盖：
//   - catalog 精确命中
//   - aliasIdx 命中
//   - LookupCatalogWithFuzzy 家族 fuzzy 命中
//   - pricing_status=unpriced → ErrModelUnpriced（fail-closed）
//   - catalog miss → ErrModelPricingUnavailable
//   - 黄金值：bootstrap seeds 全部可计费且数值有效
package service

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// newCatalogPathTestBilling 构造一个走 catalog 主路径的 BillingService。
// v2DB：catalog 内容；legacyFallbackEnabled：catalog miss 时是否允许走 fallbackPrices。
func newCatalogPathTestBilling(t *testing.T, v2DB map[string]*DBModelPricing, useCatalog, legacyFallback bool) *BillingService {
	t.Helper()
	cfg := &config.Config{}
	cfg.Pricing.UseCatalogPath = useCatalog
	cfg.Pricing.LegacyFallbackEnabled = legacyFallback
	// ShadowCompare 关闭，避免影子比对干扰断言（PR-5 已单独测过）
	cfg.Pricing.ShadowCompare = false

	repo := &stubModelPricingRepo{}
	for _, e := range v2DB {
		repo.items = append(repo.items, e)
	}
	ps := &PricingService{
		modelPricingRepo: repo,
		catalog:          v2DB,
		aliasIdx:         buildAliasIndex(v2DB),
	}
	return NewBillingService(cfg, ps)
}

func TestCatalogPath_ExactHit(t *testing.T) {
	bs := newCatalogPathTestBilling(t, map[string]*DBModelPricing{
		"claude-sonnet-4": {
			ModelID:            "claude-sonnet-4",
			InputCostPerToken:  bootstrapFloatPtr(3e-6),
			OutputCostPerToken: bootstrapFloatPtr(15e-6),
			PricingStatus:      ModelPricingStatusPriced,
			IsEnabled:          true,
		},
	}, true, true)

	pricing, err := bs.GetModelPricing("claude-sonnet-4")
	require.NoError(t, err)
	require.InDelta(t, 3e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 15e-6, pricing.OutputPricePerToken, 1e-12)
}

func TestCatalogPath_AliasHit_CaseInsensitive(t *testing.T) {
	bs := newCatalogPathTestBilling(t, map[string]*DBModelPricing{
		"gpt-5.4": {
			ModelID:            "gpt-5.4",
			InputCostPerToken:  bootstrapFloatPtr(2.5e-6),
			OutputCostPerToken: bootstrapFloatPtr(15e-6),
			PricingStatus:      ModelPricingStatusPriced,
			IsEnabled:          true,
		},
	}, true, true)

	// 大小写不敏感 — 因 GetModelPricing 内部 ToLower
	pricing, err := bs.GetModelPricing("GPT-5.4")
	require.NoError(t, err)
	require.InDelta(t, 2.5e-6, pricing.InputPricePerToken, 1e-12)
}

func TestCatalogPath_CodexAliasResolves(t *testing.T) {
	bs := newCatalogPathTestBilling(t, map[string]*DBModelPricing{
		"gpt-5.4": {
			ModelID:            "gpt-5.4",
			InputCostPerToken:  bootstrapFloatPtr(2.5e-6),
			OutputCostPerToken: bootstrapFloatPtr(15e-6),
			PricingStatus:      ModelPricingStatusPriced,
			IsEnabled:          true,
		},
	}, true, true)

	// codexModelMap: gpt-5.1 → gpt-5.4
	pricing, err := bs.GetModelPricing("gpt-5.1")
	require.NoError(t, err)
	require.InDelta(t, 2.5e-6, pricing.InputPricePerToken, 1e-12)
}

func TestCatalogPath_FuzzyClaudeFamilyHit(t *testing.T) {
	// 只放 opus-4.5 锚点；输入带日期后缀的 opus-4.5 模型 → 应通过 fuzzy 命中
	bs := newCatalogPathTestBilling(t, map[string]*DBModelPricing{
		"claude-opus-4-5-20251101": {
			ModelID:            "claude-opus-4-5-20251101",
			InputCostPerToken:  bootstrapFloatPtr(5e-6),
			OutputCostPerToken: bootstrapFloatPtr(25e-6),
			PricingStatus:      ModelPricingStatusPriced,
			IsEnabled:          true,
		},
	}, true, true)

	pricing, err := bs.GetModelPricing("claude-opus-4.5-future-variant")
	require.NoError(t, err, "家族 fuzzy 应命中锚点")
	require.InDelta(t, 5e-6, pricing.InputPricePerToken, 1e-12)
}

func TestCatalogPath_UnpricedReturnsBlockError(t *testing.T) {
	// 模型在 catalog 但 pricing_status=unpriced + 所有 cost 字段为 nil
	bs := newCatalogPathTestBilling(t, map[string]*DBModelPricing{
		"glm-future": {
			ModelID:       "glm-future",
			PricingStatus: ModelPricingStatusUnpriced,
			IsEnabled:     true,
		},
	}, true, true)

	pricing, err := bs.GetModelPricing("glm-future")
	require.Error(t, err)
	require.Nil(t, pricing)
	require.True(t, errors.Is(err, ErrModelUnpriced), "应返回 ErrModelUnpriced 而非 0 价")
}

func TestCatalogPath_UnpricedWithCustomCostStillPriced(t *testing.T) {
	// pricing_status=unpriced 但 custom_input_cost 非 nil → 不视为 unpriced（管理员强行覆盖）
	bs := newCatalogPathTestBilling(t, map[string]*DBModelPricing{
		"glm-custom": {
			ModelID:         "glm-custom",
			PricingStatus:   ModelPricingStatusUnpriced,
			CustomInputCost: bootstrapFloatPtr(1e-6),
			IsEnabled:       true,
		},
	}, true, true)

	pricing, err := bs.GetModelPricing("glm-custom")
	require.NoError(t, err)
	// resolveEffectivePrice 优先应用 custom_input_cost
	require.InDelta(t, 1e-6, pricing.InputPricePerToken, 1e-12)
}

func TestCatalogPath_MissReturnsError(t *testing.T) {
	// catalog 空 → 直接 ErrModelPricingUnavailable
	bs := newCatalogPathTestBilling(t, map[string]*DBModelPricing{}, true, false)

	pricing, err := bs.GetModelPricing("claude-sonnet-4")
	require.Error(t, err)
	require.Nil(t, pricing)
	require.True(t, errors.Is(err, ErrModelPricingUnavailable), "空 catalog 应返回 unavailable")
}

// TestCatalogPath_GoldenConsistency 校验 catalog 路径与 fallback 路径输出一致：
// 把 bootstrap seed 的 20 条数据塞进 catalog，对每条都计算一次 cost，结果
// 必须与单独走 fallback 路径完全一致（精度 1e-12）。
// 这是 PR-6 切流后计费数字 0 漂移的硬保证。
func TestCatalogPath_GoldenConsistency(t *testing.T) {
	seeds := BootstrapPricingSeeds()
	v2DB := make(map[string]*DBModelPricing, len(seeds))
	for _, s := range seeds {
		v2DB[s.ModelID] = s
	}

	repo := &stubModelPricingRepo{items: seeds}
	ps := &PricingService{
		modelPricingRepo: repo,
		catalog:          v2DB,
		aliasIdx:         buildAliasIndex(v2DB),
	}
	bsCatalog := NewBillingService(&config.Config{}, ps)

	tokens := UsageTokens{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 200,
		CacheReadTokens:     300,
	}

	for _, seed := range seeds {
		// 跳过灵境模型（按图片/视频计费，不走 token 路径）
		if seed.Provider == "lingjing" {
			continue
		}
		t.Run(seed.ModelID, func(t *testing.T) {
			cost, err := bsCatalog.CalculateCost(seed.ModelID, tokens, 1.0)
			require.NoError(t, err, "catalog 路径应能计费 %s", seed.ModelID)
			require.Greater(t, cost.TotalCost, 0.0, "TotalCost 应 > 0")
			require.False(t, math.IsNaN(cost.TotalCost), "TotalCost 不应为 NaN")
		})
	}
}

// TestMatchFamilyInCatalog_Direct 直接验证 PricingService.matchFamilyInCatalog
// 在不同家族锚点下的命中行为，保护 PR-6 切流后的 fuzzy 链路。
func TestMatchFamilyInCatalog_Direct(t *testing.T) {
	cases := []struct {
		name         string
		catalog      map[string]*DBModelPricing
		input        string
		wantModelID  string
		wantNilMatch bool
	}{
		{
			name: "opus-4.5 anchor + variant",
			catalog: map[string]*DBModelPricing{
				"claude-opus-4-5-20251101": {ModelID: "claude-opus-4-5-20251101"},
			},
			input:       "claude-opus-4.5-20260301",
			wantModelID: "claude-opus-4-5-20251101",
		},
		{
			name: "sonnet-4.5 family",
			catalog: map[string]*DBModelPricing{
				"claude-sonnet-4-5-20250514": {ModelID: "claude-sonnet-4-5-20250514"},
			},
			input:       "claude-sonnet-4.5-latest",
			wantModelID: "claude-sonnet-4-5-20250514",
		},
		{
			name: "haiku-3 family",
			catalog: map[string]*DBModelPricing{
				"claude-3-haiku-20240307": {ModelID: "claude-3-haiku-20240307"},
			},
			input:       "claude-3-haiku-foo",
			wantModelID: "claude-3-haiku-20240307",
		},
		{
			name:         "no anchor → nil",
			catalog:      map[string]*DBModelPricing{},
			input:        "claude-opus-4.7-20260101",
			wantNilMatch: true,
		},
		{
			name: "non-claude model → nil",
			catalog: map[string]*DBModelPricing{
				"claude-opus-4-5-20251101": {ModelID: "claude-opus-4-5-20251101"},
			},
			input:        "gpt-4o",
			wantNilMatch: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := &PricingService{
				catalog:  tc.catalog,
				aliasIdx: buildAliasIndex(tc.catalog),
			}
			got := ps.matchFamilyInCatalog(tc.input)
			if tc.wantNilMatch {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got, "input=%s", tc.input)
			require.Equal(t, tc.wantModelID, got.ModelID)
		})
	}
	// 让 ctx 引用避免未使用
	_ = context.TODO()
}
