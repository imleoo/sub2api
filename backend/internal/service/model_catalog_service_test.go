//go:build unit

// PR-7 单元测试：ModelCatalogService + ListEnabledCatalogModels。
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---- captureModelPricingRepo：记录调用路径 ----

type captureModelPricingRepo struct {
	stubModelPricingRepo
	upsertCalled bool
	seedCalled   bool
}

func (r *captureModelPricingRepo) UpsertBatch(_ context.Context, models []*DBModelPricing) error {
	r.upsertCalled = true
	r.items = append(r.items, models...)
	return nil
}

func (r *captureModelPricingRepo) SeedIfNotExists(_ context.Context, models []*DBModelPricing) error {
	r.seedCalled = true
	for _, m := range models {
		r.items = append(r.items, m)
	}
	return nil
}

func (r *captureModelPricingRepo) LoadAllEnabled(_ context.Context) ([]*DBModelPricing, error) {
	return r.items, nil
}

// ---- 辅助：newCatalogServiceForTest ----

func newCatalogServiceForTest(t *testing.T, preload []*DBModelPricing) (*ModelCatalogService, *captureModelPricingRepo, *PricingService) {
	t.Helper()
	repo := &captureModelPricingRepo{stubModelPricingRepo: stubModelPricingRepo{items: preload}}
	ps := &PricingService{
		modelPricingRepo: repo,
		catalog:          map[string]*DBModelPricing{},
		aliasIdx:         map[string]string{},
	}
	cs := NewModelCatalogService(repo, ps)
	return cs, repo, ps
}

// TestModelCatalogService_LiteLLMCallsUpsertBatch 验证 source=litellm 走 UpsertBatch。
func TestModelCatalogService_LiteLLMCallsUpsertBatch(t *testing.T) {
	cs, repo, _ := newCatalogServiceForTest(t, nil)

	_, err := cs.UpsertModels(context.Background(), []*DBModelPricing{
		{ModelID: "gpt-4o", Provider: "openai", Mode: "chat"},
	}, ModelPricingSourceLiteLLM)

	require.NoError(t, err)
	require.True(t, repo.upsertCalled, "litellm 应走 UpsertBatch")
	require.False(t, repo.seedCalled, "litellm 不应走 SeedIfNotExists")
}

// TestModelCatalogService_UpstreamSyncCallsSeedIfNotExists 验证 source=upstream_sync 走 SeedIfNotExists。
func TestModelCatalogService_UpstreamSyncCallsSeedIfNotExists(t *testing.T) {
	cs, repo, _ := newCatalogServiceForTest(t, nil)

	_, err := cs.UpsertModels(context.Background(), []*DBModelPricing{
		{ModelID: "my-model", Provider: "generic", Mode: "chat"},
	}, ModelPricingSourceUpstreamSync)

	require.NoError(t, err)
	require.False(t, repo.upsertCalled, "upstream_sync 不应走 UpsertBatch")
	require.True(t, repo.seedCalled, "upstream_sync 应走 SeedIfNotExists")
}

// TestModelCatalogService_SourceTagAutoFilled 验证 batch 里 Source 留空时由 UpsertModels 自动填充。
func TestModelCatalogService_SourceTagAutoFilled(t *testing.T) {
	cs, repo, _ := newCatalogServiceForTest(t, nil)

	batch := []*DBModelPricing{{ModelID: "m1"}}
	_, err := cs.UpsertModels(context.Background(), batch, ModelPricingSourceUpstreamSync)
	require.NoError(t, err)
	require.Equal(t, ModelPricingSourceUpstreamSync, repo.items[0].Source)
}

// TestModelCatalogService_EmptyBatchNoOp 空批次不报错也不调用 repo。
func TestModelCatalogService_EmptyBatchNoOp(t *testing.T) {
	cs, repo, _ := newCatalogServiceForTest(t, nil)

	n, err := cs.UpsertModels(context.Background(), nil, ModelPricingSourceLiteLLM)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.False(t, repo.upsertCalled)
	require.False(t, repo.seedCalled)
}

// TestListEnabledCatalogModels_FiltersDisabled 验证 ListEnabledCatalogModels 只返回 is_enabled=true。
func TestListEnabledCatalogModels_FiltersDisabled(t *testing.T) {
	ps := &PricingService{
		catalog: map[string]*DBModelPricing{
			"enabled-model": {
				ModelID:            "enabled-model",
				Provider:           "openai",
				Mode:               "chat",
				IsEnabled:          true,
				InputCostPerToken:  bootstrapFloatPtr(1e-6),
				OutputCostPerToken: bootstrapFloatPtr(2e-6),
			},
			"disabled-model": {
				ModelID:   "disabled-model",
				Provider:  "openai",
				Mode:      "chat",
				IsEnabled: false,
			},
		},
	}

	models := ps.ListEnabledCatalogModels()
	require.Len(t, models, 1)
	require.Equal(t, "enabled-model", models[0].ID)
	require.InDelta(t, 1e-6, models[0].InputCostPerToken, 1e-12)
	require.InDelta(t, 2e-6, models[0].OutputCostPerToken, 1e-12)
	require.InDelta(t, 1.0, models[0].DiscountRate, 1e-9, "默认折扣率应为 1.0")
}

// TestListEnabledCatalogModels_DiscountRate 自定义折扣率正确填充。
func TestListEnabledCatalogModels_DiscountRate(t *testing.T) {
	discount := 0.8
	ps := &PricingService{
		catalog: map[string]*DBModelPricing{
			"m": {ModelID: "m", IsEnabled: true, DiscountRate: &discount},
		},
	}

	models := ps.ListEnabledCatalogModels()
	require.Len(t, models, 1)
	require.InDelta(t, 0.8, models[0].DiscountRate, 1e-9)
}

// TestListEnabledCatalogModels_EmptyCatalog 空 catalog 返回空切片，不 panic。
func TestListEnabledCatalogModels_EmptyCatalog(t *testing.T) {
	ps := &PricingService{
		catalog:     map[string]*DBModelPricing{},
	}
	require.Empty(t, ps.ListEnabledCatalogModels())
}
