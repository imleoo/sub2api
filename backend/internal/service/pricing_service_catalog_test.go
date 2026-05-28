//go:build unit

// SSOT 重构 PR-4：catalog + aliasIdx 影子层测试。
//
// 验证：
//  1. buildAliasIndex 自身 + Codex 别名 + 归一化形态的三路合并
//  2. LookupCatalog 命中：精确 / 小写 / Codex 别名 / normalize 形态
//  3. miss 返回 nil（不调用 matchByModelFamily，留给 PR-6）
//  4. buildCatalogAndAliasIndex 失败时保留旧 catalog（fail-open）
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubModelPricingRepo 用于 catalog 测试的最小 ModelPricingRepository。
type stubModelPricingRepo struct {
	items   []*DBModelPricing
	loadErr error
}

func (s *stubModelPricingRepo) UpsertBatch(ctx context.Context, models []*DBModelPricing) error {
	return nil
}
func (s *stubModelPricingRepo) Create(ctx context.Context, m *DBModelPricing) error { return nil }
func (s *stubModelPricingRepo) GetByID(ctx context.Context, id int64) (*DBModelPricing, error) {
	return nil, nil
}
func (s *stubModelPricingRepo) GetByModelID(ctx context.Context, modelID string) (*DBModelPricing, error) {
	return nil, nil
}
func (s *stubModelPricingRepo) List(ctx context.Context, filter ModelPricingListFilter) ([]*DBModelPricing, int, error) {
	return nil, 0, nil
}
func (s *stubModelPricingRepo) Update(ctx context.Context, m *DBModelPricing) error { return nil }
func (s *stubModelPricingRepo) Delete(ctx context.Context, id int64) error          { return nil }
func (s *stubModelPricingRepo) LoadAllEnabled(ctx context.Context) ([]*DBModelPricing, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.items, nil
}
func (s *stubModelPricingRepo) BulkUpdateDiscountRates(ctx context.Context, rates map[string]float64) error {
	return nil
}
func (s *stubModelPricingRepo) SeedIfNotExists(ctx context.Context, models []*DBModelPricing) error {
	return nil
}

func newCatalogTestService(items []*DBModelPricing) (*PricingService, *stubModelPricingRepo) {
	repo := &stubModelPricingRepo{items: items}
	svc := &PricingService{modelPricingRepo: repo}
	svc.buildCatalogAndAliasIndex(context.Background())
	return svc, repo
}

func TestBuildAliasIndex_SelfAndNormalized(t *testing.T) {
	catalog := map[string]*DBModelPricing{
		"GPT-5.4":         {ModelID: "GPT-5.4"},
		"claude-sonnet-4": {ModelID: "claude-sonnet-4"},
		"models/foo":      {ModelID: "models/foo"},
	}
	idx := buildAliasIndex(catalog)

	// catalog 自身 model_id 的小写映射到自己（保留 canonical 大小写）
	require.Equal(t, "GPT-5.4", idx["gpt-5.4"])
	require.Equal(t, "claude-sonnet-4", idx["claude-sonnet-4"])

	// normalize 形态：models/foo → foo
	require.Equal(t, "models/foo", idx["foo"])
}

func TestBuildAliasIndex_CodexVariantsResolveToCatalog(t *testing.T) {
	catalog := map[string]*DBModelPricing{
		"gpt-5.4":       {ModelID: "gpt-5.4"},
		"gpt-5.3-codex": {ModelID: "gpt-5.3-codex"},
	}
	idx := buildAliasIndex(catalog)

	// codexModelMap 中 "gpt-5.1" → "gpt-5.4"
	require.Equal(t, "gpt-5.4", idx["gpt-5.1"])
	// "codex-mini-latest" → "gpt-5.3-codex"
	require.Equal(t, "gpt-5.3-codex", idx["codex-mini-latest"])
	// "gpt-5-codex" → "gpt-5.3-codex"
	require.Equal(t, "gpt-5.3-codex", idx["gpt-5-codex"])
}

func TestBuildAliasIndex_CodexTargetMissingFromCatalogSkipped(t *testing.T) {
	// 只放 gpt-5.4，不放 gpt-5.3-codex
	catalog := map[string]*DBModelPricing{
		"gpt-5.4": {ModelID: "gpt-5.4"},
	}
	idx := buildAliasIndex(catalog)

	// "codex-mini-latest" 在 codexModelMap 中指向 "gpt-5.3-codex"，但 catalog 缺这条 → 跳过
	_, ok := idx["codex-mini-latest"]
	require.False(t, ok, "目标不在 catalog 时应跳过避免悬空别名")

	// "gpt-5.1" 指向 "gpt-5.4"（在 catalog 里）→ 应注入
	require.Equal(t, "gpt-5.4", idx["gpt-5.1"])
}

func TestLookupCatalog_HitPaths(t *testing.T) {
	svc, _ := newCatalogTestService([]*DBModelPricing{
		{ModelID: "gpt-5.4", IsEnabled: true,
			InputCostPerToken:  bootstrapFloatPtr(2.5e-6),
			OutputCostPerToken: bootstrapFloatPtr(15e-6)},
		{ModelID: "claude-opus-4.5", IsEnabled: true,
			InputCostPerToken: bootstrapFloatPtr(5e-6)},
	})

	cases := []struct {
		name      string
		input     string
		wantModel string
	}{
		{"exact", "gpt-5.4", "gpt-5.4"},
		{"lowercase", "GPT-5.4", "gpt-5.4"},
		{"codex variant gpt-5.1 → gpt-5.4", "gpt-5.1", "gpt-5.4"},
		{"codex variant gpt-5-mini → gpt-5.4", "gpt-5-mini", "gpt-5.4"},
		{"exact claude opus 4.5", "claude-opus-4.5", "claude-opus-4.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.LookupCatalog(tc.input)
			require.NotNil(t, got, "input=%s", tc.input)
			require.Equal(t, tc.wantModel, got.ModelID)
		})
	}
}

func TestLookupCatalog_MissReturnsNil(t *testing.T) {
	svc, _ := newCatalogTestService([]*DBModelPricing{
		{ModelID: "gpt-5.4", IsEnabled: true},
	})

	require.Nil(t, svc.LookupCatalog(""))
	require.Nil(t, svc.LookupCatalog("totally-unknown-model"))
	// 家族 fuzzy 不在 catalog lookup 范围内（PR-6 才接入）
	require.Nil(t, svc.LookupCatalog("claude-opus-4.7-20260301"))
}

func TestBuildCatalogAndAliasIndex_FailureKeepsOldCatalog(t *testing.T) {
	repo := &stubModelPricingRepo{items: []*DBModelPricing{
		{ModelID: "gpt-5.4", IsEnabled: true},
	}}
	svc := &PricingService{modelPricingRepo: repo}
	svc.buildCatalogAndAliasIndex(context.Background())
	require.Equal(t, 1, svc.CatalogSize())
	require.Greater(t, svc.AliasIndexSize(), 0)

	// 模拟 DB 不可用
	repo.loadErr = errors.New("db down")
	svc.buildCatalogAndAliasIndex(context.Background())

	// 旧 catalog 保留，错误计数 +1
	require.Equal(t, 1, svc.CatalogSize(), "失败时不应清空 catalog")
	require.NotNil(t, svc.LookupCatalog("gpt-5.4"), "旧 catalog 模型仍可命中")
	require.EqualValues(t, 1, svc.catalogLoadErrors)
}

func TestReloadFromDB_RebuildsCatalog(t *testing.T) {
	repo := &stubModelPricingRepo{items: []*DBModelPricing{
		{ModelID: "gpt-5.4", IsEnabled: true},
	}}
	svc := &PricingService{
		modelPricingRepo: repo,
	}
	svc.ReloadFromDB(context.Background())
	require.Equal(t, 1, svc.CatalogSize())

	// 加新条目，调 ReloadFromDB → catalog 应包含新模型
	repo.items = append(repo.items, &DBModelPricing{ModelID: "claude-opus-4.5", IsEnabled: true})
	svc.ReloadFromDB(context.Background())
	require.Equal(t, 2, svc.CatalogSize())
	require.NotNil(t, svc.LookupCatalog("claude-opus-4.5"))
}

func TestCodexAliasPairs_NonEmpty(t *testing.T) {
	pairs := CodexAliasPairs()
	require.Greater(t, len(pairs), 40, "codexModelMap 至少有 41 条变体映射 + 7 条 prefix 映射")
	// 抽样几个关键的
	seen := make(map[string]string, len(pairs))
	for _, p := range pairs {
		if _, ok := seen[p[0]]; ok {
			continue
		}
		seen[p[0]] = p[1]
	}
	require.Equal(t, "gpt-5.4", seen["gpt-5.1"])
	require.Equal(t, "gpt-5.3-codex", seen["codex-mini-latest"])
	require.Equal(t, "gpt-5.2", seen["gpt-5.2-codex"])
}
