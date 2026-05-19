package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeProviderPricingRepo 是 service 层单测用的内存 stub。
//
// 不依赖 sqlite/ent，单测覆盖 Resolve 的纯函数行为；ent + repo 的命中查询行为
// 在 repository/provider_pricing_repository_unit_test.go 中独立覆盖（P0-3）。
type fakeProviderPricingRepo struct {
	pricing *DBProviderPricing
	err     error
}

func (f *fakeProviderPricingRepo) Create(_ context.Context, _ *DBProviderPricing) error {
	return errors.New("unused in tests")
}
func (f *fakeProviderPricingRepo) Update(_ context.Context, _ *DBProviderPricing) error {
	return errors.New("unused in tests")
}
func (f *fakeProviderPricingRepo) Delete(_ context.Context, _ int64) error {
	return errors.New("unused in tests")
}
func (f *fakeProviderPricingRepo) GetByID(_ context.Context, _ int64) (*DBProviderPricing, error) {
	return nil, errors.New("unused in tests")
}
func (f *fakeProviderPricingRepo) List(_ context.Context, _ ProviderPricingListFilter) ([]*DBProviderPricing, int, error) {
	return nil, 0, errors.New("unused in tests")
}
func (f *fakeProviderPricingRepo) FindEffective(_ context.Context, _, _ string, _ time.Time) (*DBProviderPricing, error) {
	return f.pricing, f.err
}

// TestUpstreamCostResolver_TokenBillingHit 命中 token 计费 → 返回 (cost, unitPrices, "provider_table")。
func TestUpstreamCostResolver_TokenBillingHit(t *testing.T) {
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

	tokens := UsageTokens{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 200,
		CacheReadTokens:     800,
	}
	cost, prices, source := resolver.Resolve(
		context.Background(),
		"anthropic", "claude-sonnet-4-6",
		tokens, time.Now(),
	)

	require.NotNil(t, cost)
	require.NotNil(t, prices)
	require.Equal(t, "provider_table", source)

	expected := 1000*3e-6 + 500*15e-6 + 200*3.75e-6 + 800*0.3e-6
	require.InDelta(t, expected, *cost, 1e-12)

	require.InDelta(t, 3e-6, prices.InputPrice, 1e-12)
	require.InDelta(t, 15e-6, prices.OutputPrice, 1e-12)
	require.InDelta(t, 3.75e-6, prices.CacheCreationPrice, 1e-12)
	require.InDelta(t, 0.3e-6, prices.CacheReadPrice, 1e-12)
}

// TestUpstreamCostResolver_PerRequestBillingHit 命中 per_request 计费（lingjing Seedance）。
func TestUpstreamCostResolver_PerRequestBillingHit(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider:    "lingjing",
			Model:       "doubao-seedance-1.5-pro-5s",
			BillingMode: "per_request",
			InputPrice:  0.42,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}

	// per_request 计费不看 token 数；即便给 ImageOutputTokens=1 也只按一次计费
	tokens := UsageTokens{ImageOutputTokens: 1}
	cost, prices, source := resolver.Resolve(
		context.Background(),
		"lingjing", "doubao-seedance-1.5-pro-5s",
		tokens, time.Now(),
	)

	require.NotNil(t, cost)
	require.Equal(t, "provider_table", source)
	require.InDelta(t, 0.42, *cost, 1e-12)
	require.InDelta(t, 0.42, prices.InputPrice, 1e-12)
}

// TestUpstreamCostResolver_ImageBillingHit 命中 image 计费（按张）。
func TestUpstreamCostResolver_ImageBillingHit(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider:    "lingjing",
			Model:       "doubao-seedream-3.0-image",
			BillingMode: "image",
			InputPrice:  0.04,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}

	// 4 张图
	tokens := UsageTokens{ImageOutputTokens: 4}
	cost, _, source := resolver.Resolve(
		context.Background(),
		"lingjing", "doubao-seedream-3.0-image",
		tokens, time.Now(),
	)

	require.NotNil(t, cost)
	require.Equal(t, "provider_table", source)
	require.InDelta(t, 0.16, *cost, 1e-12, "0.04 × 4 = 0.16")
}

// TestUpstreamCostResolver_ImageBillingDefaultsToOne ImageOutputTokens=0 时按 1 张计费（兼容虚拟 token）。
func TestUpstreamCostResolver_ImageBillingDefaultsToOne(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider:    "lingjing",
			Model:       "doubao-seedream-3.0-image",
			BillingMode: "image",
			InputPrice:  0.04,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}

	tokens := UsageTokens{ImageOutputTokens: 0}
	cost, _, source := resolver.Resolve(
		context.Background(),
		"lingjing", "doubao-seedream-3.0-image",
		tokens, time.Now(),
	)

	require.NotNil(t, cost)
	require.Equal(t, "provider_table", source)
	require.InDelta(t, 0.04, *cost, 1e-12, "ImageOutputTokens=0 兜底为 1 张")
}

// TestUpstreamCostResolver_Miss 未命中 → (nil, nil, "")。
func TestUpstreamCostResolver_Miss(t *testing.T) {
	repo := &fakeProviderPricingRepo{pricing: nil}
	resolver := &UpstreamCostResolver{repo: repo}

	cost, prices, source := resolver.Resolve(
		context.Background(),
		"unknown-provider", "any-model",
		UsageTokens{InputTokens: 100}, time.Now(),
	)

	require.Nil(t, cost)
	require.Nil(t, prices)
	require.Equal(t, "", source, "未命中 source 必须为空串，与 upstream_total_cost=NULL 同步")
}

// TestUpstreamCostResolver_RepoError SQL 错误退化为未命中。
func TestUpstreamCostResolver_RepoError(t *testing.T) {
	repo := &fakeProviderPricingRepo{err: errors.New("db down")}
	resolver := &UpstreamCostResolver{repo: repo}

	cost, prices, source := resolver.Resolve(
		context.Background(),
		"anthropic", "claude-sonnet-4-6",
		UsageTokens{InputTokens: 100}, time.Now(),
	)

	require.Nil(t, cost, "SQL 错误不阻塞 UsageLog 写入热路径")
	require.Nil(t, prices)
	require.Equal(t, "", source)
}

// TestUpstreamCostResolver_EmptyArgs provider / model 为空 → 立即返回 (nil, nil, "")。
func TestUpstreamCostResolver_EmptyArgs(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider: "anthropic", Model: "claude-sonnet-4-6",
			BillingMode: "token", InputPrice: 3e-6,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}

	cost, _, source := resolver.Resolve(context.Background(), "", "claude-sonnet-4-6", UsageTokens{InputTokens: 100}, time.Now())
	require.Nil(t, cost)
	require.Equal(t, "", source)

	cost, _, source = resolver.Resolve(context.Background(), "anthropic", "", UsageTokens{InputTokens: 100}, time.Now())
	require.Nil(t, cost)
	require.Equal(t, "", source)
}

// TestUpstreamCostResolver_ZeroCostMiss 命中但所有 token 数为 0 → cost=0，按"未命中"处理。
//
// 设计意图：avoid 写入 (upstream_total_cost=0, pricing_source="provider_table") 行级数据，
// 与 §3.1 一致性约束保持兼容（pricing_source 必须与 cost 同时 NULL 或同时非空，
// 0 与 NULL 等价处理）。
func TestUpstreamCostResolver_ZeroCostMiss(t *testing.T) {
	repo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider: "anthropic", Model: "claude-sonnet-4-6",
			BillingMode: "token", InputPrice: 3e-6, OutputPrice: 15e-6,
		},
	}
	resolver := &UpstreamCostResolver{repo: repo}

	cost, prices, source := resolver.Resolve(
		context.Background(),
		"anthropic", "claude-sonnet-4-6",
		UsageTokens{}, // 全 0
		time.Now(),
	)

	require.Nil(t, cost, "cost=0 时不写入快照，保持 NULL")
	require.Nil(t, prices)
	require.Equal(t, "", source)
}

// TestUpstreamCostResolver_NilResolver nil resolver 安全返回（防御性测试）。
func TestUpstreamCostResolver_NilResolver(t *testing.T) {
	var resolver *UpstreamCostResolver
	cost, prices, source := resolver.Resolve(context.Background(), "anthropic", "claude-sonnet-4-6", UsageTokens{InputTokens: 100}, time.Now())
	require.Nil(t, cost)
	require.Nil(t, prices)
	require.Equal(t, "", source)
}
