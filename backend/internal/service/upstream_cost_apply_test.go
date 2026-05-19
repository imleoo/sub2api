package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ApplyUpstreamCostSnapshot 装配点行为单测（Phase 0 P0-5）。
//
// 这 4 个用例覆盖 sprint-plan P0-5 验收：
//   - 命中（flag=true）：9 列全部填充
//   - 未命中（flag=true）：upstream_total_cost / pricing_source / 单价 / cost_finalized_at 保持 NULL，
//     但 Provider 仍写入（便于按 provider 维度做"无快照"统计）
//   - flag=false：所有 9 列保持 NULL（含 Provider，保持旧行为）
//   - 3 个装配点行为等价：同样 provider+model+tokens，输出 UsageLog 字段相同

// TestApplyUpstreamCostSnapshot_HitFillsAllNineFields 命中场景：9 列填充正确。
func TestApplyUpstreamCostSnapshot_HitFillsAllNineFields(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider:           "anthropic",
			Model:              "claude-sonnet-4-6",
			BillingMode:        "token",
			InputPrice:         3e-6,
			OutputPrice:        15e-6,
			CacheCreationPrice: 3.75e-6,
			CacheReadPrice:     0.3e-6,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}
	log := &UsageLog{}
	requestEnd := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	ApplyUpstreamCostSnapshot(
		context.Background(),
		log,
		resolver,
		"anthropic",
		"claude-sonnet-4-6",
		UsageTokens{InputTokens: 1000, OutputTokens: 500},
		requestEnd,
		true, // flag enabled
	)

	require.NotNil(t, log.UpstreamUnitPriceInput)
	require.NotNil(t, log.UpstreamUnitPriceOutput)
	require.NotNil(t, log.UpstreamUnitPriceCacheCreation)
	require.NotNil(t, log.UpstreamUnitPriceCacheRead)
	require.NotNil(t, log.UpstreamTotalCost)
	require.NotNil(t, log.PricingSource)
	require.NotNil(t, log.Provider)
	require.NotNil(t, log.CostFinalizedAt)

	require.InDelta(t, 3e-6, *log.UpstreamUnitPriceInput, 1e-12)
	require.InDelta(t, 1000*3e-6+500*15e-6, *log.UpstreamTotalCost, 1e-12)
	require.Equal(t, "provider_table", *log.PricingSource)
	require.Equal(t, "anthropic", *log.Provider)
	require.Equal(t, requestEnd, *log.CostFinalizedAt)
}

// TestApplyUpstreamCostSnapshot_MissKeepsCostFieldsNull 未命中：cost/unit_price/pricing_source/cost_finalized_at 保持 NULL；Provider 仍写入。
//
// 一致性约束（docs/upstream-cost-snapshot.md §3.1）：upstream_total_cost 与 pricing_source 同时 NULL。
func TestApplyUpstreamCostSnapshot_MissKeepsCostFieldsNull(t *testing.T) {
	repo := &fakeProviderPricingRepo{pricing: nil}
	resolver := &UpstreamCostResolver{repo: repo}
	log := &UsageLog{}

	ApplyUpstreamCostSnapshot(
		context.Background(),
		log,
		resolver,
		"unknown-provider",
		"any-model",
		UsageTokens{InputTokens: 1000},
		time.Now(),
		true,
	)

	// 9 列中只有 Provider 非空，其余保持 NULL
	require.Nil(t, log.UpstreamUnitPriceInput)
	require.Nil(t, log.UpstreamUnitPriceOutput)
	require.Nil(t, log.UpstreamUnitPriceCacheCreation)
	require.Nil(t, log.UpstreamUnitPriceCacheRead)
	require.Nil(t, log.UpstreamTotalCost)
	require.Nil(t, log.PricingSource, "未命中 pricing_source 必须为 NULL（与 upstream_total_cost 同步）")
	require.Nil(t, log.CostFinalizedAt)
	require.NotNil(t, log.Provider, "Provider 始终写入（便于'无快照'维度统计）")
	require.Equal(t, "unknown-provider", *log.Provider)
}

// TestApplyUpstreamCostSnapshot_FlagDisabledKeepsAllNull flag=false：全部 9 列保持 NULL（含 Provider，保留旧行为）。
func TestApplyUpstreamCostSnapshot_FlagDisabledKeepsAllNull(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider: "anthropic", Model: "claude-sonnet-4-6",
			BillingMode: "token", InputPrice: 3e-6,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}
	log := &UsageLog{}

	ApplyUpstreamCostSnapshot(
		context.Background(),
		log,
		resolver,
		"anthropic",
		"claude-sonnet-4-6",
		UsageTokens{InputTokens: 1000},
		time.Now(),
		false, // flag disabled
	)

	// flag=false 时所有 9 列保持 NULL（不写 Provider），等同旧行为
	require.Nil(t, log.UpstreamUnitPriceInput)
	require.Nil(t, log.UpstreamTotalCost)
	require.Nil(t, log.PricingSource)
	require.Nil(t, log.Provider, "flag=false 时连 Provider 也不写，确保可热切回滚到完全旧行为")
	require.Nil(t, log.CostFinalizedAt)
}

// TestApplyUpstreamCostSnapshot_ThreeBuilderEquivalence 3 个装配点行为等价（验收用例 D）。
//
// 同样 provider+model+tokens+resolver+flag 三次独立调用 helper，输出 UsageLog 字段必须相同。
// 模拟 gateway_service / openai_gateway_service / usage_service 三处装配点。
func TestApplyUpstreamCostSnapshot_ThreeBuilderEquivalence(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider:           "anthropic",
			Model:              "claude-sonnet-4-6",
			BillingMode:        "token",
			InputPrice:         3e-6,
			OutputPrice:        15e-6,
			CacheCreationPrice: 3.75e-6,
			CacheReadPrice:     0.3e-6,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 20, CacheReadTokens: 80}
	requestEnd := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	gatewayLog := &UsageLog{}
	openaiLog := &UsageLog{}
	usageLog := &UsageLog{}

	for _, log := range []*UsageLog{gatewayLog, openaiLog, usageLog} {
		ApplyUpstreamCostSnapshot(
			context.Background(),
			log,
			resolver,
			"anthropic", "claude-sonnet-4-6",
			tokens, requestEnd, true,
		)
	}

	// 3 处装配点的 9 列输出必须完全相同
	require.Equal(t, *gatewayLog.UpstreamTotalCost, *openaiLog.UpstreamTotalCost, "gateway vs openai")
	require.Equal(t, *openaiLog.UpstreamTotalCost, *usageLog.UpstreamTotalCost, "openai vs usage")

	require.Equal(t, *gatewayLog.UpstreamUnitPriceInput, *openaiLog.UpstreamUnitPriceInput)
	require.Equal(t, *gatewayLog.UpstreamUnitPriceOutput, *openaiLog.UpstreamUnitPriceOutput)
	require.Equal(t, *gatewayLog.PricingSource, *openaiLog.PricingSource)
	require.Equal(t, *gatewayLog.Provider, *openaiLog.Provider)
	require.Equal(t, gatewayLog.CostFinalizedAt.Unix(), openaiLog.CostFinalizedAt.Unix())

	// 验证一致性约束：upstream_total_cost 与 pricing_source 同时非空
	require.NotNil(t, gatewayLog.UpstreamTotalCost)
	require.NotNil(t, gatewayLog.PricingSource)
}

// TestApplyUpstreamCostSnapshot_NilSafe nil log / nil resolver 都安全返回。
func TestApplyUpstreamCostSnapshot_NilSafe(t *testing.T) {
	// nil log
	ApplyUpstreamCostSnapshot(context.Background(), nil, &UpstreamCostResolver{}, "p", "m", UsageTokens{}, time.Now(), true)

	// nil resolver
	log := &UsageLog{}
	ApplyUpstreamCostSnapshot(context.Background(), log, nil, "p", "m", UsageTokens{}, time.Now(), true)
	require.Nil(t, log.UpstreamTotalCost, "nil resolver 时不写入任何字段")
}
