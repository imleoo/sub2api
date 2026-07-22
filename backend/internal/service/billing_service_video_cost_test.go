//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// newTestPricingServiceWithCatalog 构造一个仅含指定条目的 PricingService，供视频计费测试精确控制 catalog。
// cfg 非 nil 是必须的：GetCNYRate() 会读 s.cfg.Pricing.CNYRate 兜底，nil cfg 会直接 panic。
func newTestPricingServiceWithCatalog(entries ...*DBModelPricing) *PricingService {
	catalog := make(map[string]*DBModelPricing, len(entries))
	for _, e := range entries {
		catalog[e.ModelID] = e
	}
	return &PricingService{
		cfg:              &config.Config{},
		modelPricingRepo: &stubModelPricingRepo{items: entries},
		catalog:          catalog,
		aliasIdx:         buildAliasIndex(catalog),
	}
}

func floatPtr(v float64) *float64 { return &v }

// --- CalculatePerSecondVideoCost ---

func TestCalculatePerSecondVideoCost_UsesOutputCostPerImageAsPerSecondPrice(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:            "wanjie-video",
		OutputCostPerImage: floatPtr(0.1), // USD/秒
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("wanjie-video", 5, 1.0)

	require.InDelta(t, 0.5, cost.TotalCost, 1e-10)
	require.InDelta(t, 0.5, cost.ActualCost, 1e-10)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
}

func TestCalculatePerSecondVideoCost_CustomOutputCostTakesPriorityOverOutputCostPerImage(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:            "wanjie-video",
		OutputCostPerImage: floatPtr(0.1),
		CustomOutputCost:   floatPtr(0.5),
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("wanjie-video", 2, 1.0)

	// 自定义价优先：0.5 * 2 = 1.0，而不是 0.1 * 2 = 0.2
	require.InDelta(t, 1.0, cost.TotalCost, 1e-10)
}

func TestCalculatePerSecondVideoCost_FallsBackToOutputCostPerImageTokenWhenOthersMissing(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:                 "lingjing-seedance",
		OutputCostPerImageToken: floatPtr(0.05),
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("lingjing-seedance", 10, 1.0)

	require.InDelta(t, 0.5, cost.TotalCost, 1e-10)
}

func TestCalculatePerSecondVideoCost_AppliesDiscountRate(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:            "wanjie-video",
		OutputCostPerImage: floatPtr(1.0),
		DiscountRate:       floatPtr(0.5),
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("wanjie-video", 10, 1.0)

	// unitPrice(1.0) * seconds(10) * discount(0.5) = 5.0
	require.InDelta(t, 5.0, cost.TotalCost, 1e-10)
}

func TestCalculatePerSecondVideoCost_NoPricingDefaultsToZeroCostButKeepsBillingMode(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID: "wanjie-video-unpriced",
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("wanjie-video-unpriced", 5, 1.0)

	require.Equal(t, 0.0, cost.TotalCost)
	require.Equal(t, 0.0, cost.ActualCost)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
}

func TestCalculatePerSecondVideoCost_UnknownModelDefaultsToZero(t *testing.T) {
	ps := newTestPricingServiceWithCatalog()
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("does-not-exist", 5, 1.0)

	require.Equal(t, 0.0, cost.TotalCost)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
}

func TestCalculatePerSecondVideoCost_ZeroSecondsReturnsEmptyBreakdown(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:            "wanjie-video",
		OutputCostPerImage: floatPtr(0.1),
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("wanjie-video", 0, 1.0)

	require.Equal(t, &CostBreakdown{}, cost)
}

func TestCalculatePerSecondVideoCost_NegativeSecondsReturnsEmptyBreakdown(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:            "wanjie-video",
		OutputCostPerImage: floatPtr(0.1),
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("wanjie-video", -3, 1.0)

	require.Equal(t, &CostBreakdown{}, cost)
}

func TestCalculatePerSecondVideoCost_NegativeRateMultiplierClampedToZero(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:            "wanjie-video",
		OutputCostPerImage: floatPtr(0.1),
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculatePerSecondVideoCost("wanjie-video", 5, -2.0)

	require.InDelta(t, 0.5, cost.TotalCost, 1e-10)
	require.Equal(t, 0.0, cost.ActualCost)
}

// --- CalculateSeedanceVideoCost ---

func TestCalculateSeedanceVideoCost_TierHitBillsByTokenFormula(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID: "lingjing-seedance-pro",
		TierPricing: []VideoPriceTier{
			{Spec: "在线推理-720p-无声视频", CNYPerMToken: 68},
		},
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 15, false, 1.0)

	// tokens = 858(720p 每帧) * 24fps * 15s = 308,880；cost¥ = tokens/1e6 * 68；折 USD 用兜底汇率 6.8
	expectedUSD := (858.0 * 24 * 15 / 1_000_000) * 68 / DefaultWanjieCNYRate
	require.InDelta(t, expectedUSD, cost.TotalCost, 1e-9)
	require.InDelta(t, expectedUSD, cost.ActualCost, 1e-9)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
}

func TestCalculateSeedanceVideoCost_NoTierPricingDefaultsToZeroCost(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID: "lingjing-seedance-pro",
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 15, false, 1.0)

	require.Equal(t, 0.0, cost.TotalCost)
	require.Equal(t, 0.0, cost.ActualCost)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
}

func TestCalculateSeedanceVideoCost_UnknownModelDefaultsToZeroCost(t *testing.T) {
	ps := newTestPricingServiceWithCatalog()
	svc := NewBillingService(nil, ps)

	cost := svc.CalculateSeedanceVideoCost("does-not-exist", "720p", 15, false, 1.0)

	require.Equal(t, 0.0, cost.TotalCost)
}

func TestCalculateSeedanceVideoCost_ZeroSecondsReturnsEmptyBreakdown(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID: "lingjing-seedance-pro",
		TierPricing: []VideoPriceTier{
			{Spec: "在线推理-720p-无声视频", CNYPerMToken: 68},
		},
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 0, false, 1.0)

	require.Equal(t, &CostBreakdown{}, cost)
}

func TestCalculateSeedanceVideoCost_NegativeSecondsReturnsEmptyBreakdown(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID: "lingjing-seedance-pro",
		TierPricing: []VideoPriceTier{
			{Spec: "在线推理-720p-无声视频", CNYPerMToken: 68},
		},
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", -1, false, 1.0)

	require.Equal(t, &CostBreakdown{}, cost)
}

// 跨多个价格档位：selectVideoTier 优先精确匹配「在线 + 有声/无声」档，
// 命中不同 generateAudio 取值应选中不同档位单价，验证多档穿越时选档不串。
func TestCalculateSeedanceVideoCost_SelectsCorrectTierAmongMultiple(t *testing.T) {
	tiers := []VideoPriceTier{
		{Spec: "离线推理-720p-无声视频", CNYPerMToken: 40}, // 离线档，应被跳过
		{Spec: "在线推理-720p-无声视频", CNYPerMToken: 68},
		{Spec: "在线推理-720p-有声视频", CNYPerMToken: 96},
	}
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:     "lingjing-seedance-pro",
		TierPricing: tiers,
	})
	svc := NewBillingService(nil, ps)

	silentCost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 10, false, 1.0)
	audioCost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 10, true, 1.0)

	require.Less(t, silentCost.TotalCost, audioCost.TotalCost)

	expectedSilentUSD := (858.0 * 24 * 10 / 1_000_000) * 68 / DefaultWanjieCNYRate
	expectedAudioUSD := (858.0 * 24 * 10 / 1_000_000) * 96 / DefaultWanjieCNYRate
	require.InDelta(t, expectedSilentUSD, silentCost.TotalCost, 1e-9)
	require.InDelta(t, expectedAudioUSD, audioCost.TotalCost, 1e-9)
}

func TestCalculateSeedanceVideoCost_DegradesToFirstTierWhenNoOnlineMatch(t *testing.T) {
	tiers := []VideoPriceTier{
		{Spec: "离线推理-720p-无声视频", CNYPerMToken: 40},
	}
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID:     "lingjing-seedance-pro",
		TierPricing: tiers,
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 10, false, 1.0)

	expectedUSD := (858.0 * 24 * 10 / 1_000_000) * 40 / DefaultWanjieCNYRate
	require.InDelta(t, expectedUSD, cost.TotalCost, 1e-9)
}

func TestCalculateSeedanceVideoCost_AppliesDiscountRate(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID: "lingjing-seedance-pro",
		TierPricing: []VideoPriceTier{
			{Spec: "在线推理-720p-无声视频", CNYPerMToken: 68},
		},
		DiscountRate: floatPtr(0.5),
	})
	svc := NewBillingService(nil, ps)

	full := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 10, false, 1.0)

	baseUSD := (858.0 * 24 * 10 / 1_000_000) * 68 / DefaultWanjieCNYRate
	require.InDelta(t, baseUSD*0.5, full.TotalCost, 1e-9)
}

func TestCalculateSeedanceVideoCost_NegativeRateMultiplierClampedToZero(t *testing.T) {
	ps := newTestPricingServiceWithCatalog(&DBModelPricing{
		ModelID: "lingjing-seedance-pro",
		TierPricing: []VideoPriceTier{
			{Spec: "在线推理-720p-无声视频", CNYPerMToken: 68},
		},
	})
	svc := NewBillingService(nil, ps)

	cost := svc.CalculateSeedanceVideoCost("lingjing-seedance-pro", "720p", 10, false, -1.0)

	require.Greater(t, cost.TotalCost, 0.0)
	require.Equal(t, 0.0, cost.ActualCost)
}
