package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// newProviderPricingRepoSQLite 构造一个内存 SQLite 后端的 repo + ent client，
// 范式参考 api_key_repo_last_used_unit_test.go。
func newProviderPricingRepoSQLite(t *testing.T) (*providerPricingRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:provider_pricing_repo?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &providerPricingRepository{client: client}, client
}

// seedProviderPricing 是测试辅助：插入一条单价记录并断言成功。
func seedProviderPricing(
	t *testing.T,
	ctx context.Context,
	repo *providerPricingRepository,
	provider, model, billingMode string,
	inputPrice float64,
	effectiveFrom time.Time,
	effectiveTo *time.Time,
) *service.DBProviderPricing {
	t.Helper()
	m := &service.DBProviderPricing{
		Provider:      provider,
		Model:         model,
		BillingMode:   billingMode,
		InputPrice:    inputPrice,
		Currency:      "USD",
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
		Source:        "manual",
	}
	require.NoError(t, repo.Create(ctx, m))
	require.NotZero(t, m.ID)
	return m
}

// TestProviderPricingRepository_FindEffective_ExactMatch 验证精确 model 匹配命中。
func TestProviderPricingRepository_FindEffective_ExactMatch(t *testing.T) {
	repo, _ := newProviderPricingRepoSQLite(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-time.Hour)
	to := now.Add(time.Hour)
	seedProviderPricing(t, ctx, repo, "anthropic", "claude-sonnet-4-6", "token", 3e-6, from, &to)

	got, err := repo.FindEffective(ctx, "anthropic", "claude-sonnet-4-6", now)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "anthropic", got.Provider)
	require.Equal(t, "claude-sonnet-4-6", got.Model)
	require.InDelta(t, 3e-6, got.InputPrice, 1e-12)
}

// TestProviderPricingRepository_FindEffective_WildcardFallback 验证 "*" 通配兜底。
func TestProviderPricingRepository_FindEffective_WildcardFallback(t *testing.T) {
	repo, _ := newProviderPricingRepoSQLite(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-time.Hour)
	// 只为 (deepseek, "*") 写一条通配单价，不写精确模型名
	seedProviderPricing(t, ctx, repo, "deepseek", "*", "token", 1e-7, from, nil)

	got, err := repo.FindEffective(ctx, "deepseek", "deepseek-chat", now)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "deepseek", got.Provider)
	require.Equal(t, "*", got.Model, "通配匹配应返回 model='*' 的行")
	require.InDelta(t, 1e-7, got.InputPrice, 1e-12)
}

// TestProviderPricingRepository_FindEffective_OutsideTimeWindow 验证生效时间窗口外不命中。
func TestProviderPricingRepository_FindEffective_OutsideTimeWindow(t *testing.T) {
	repo, _ := newProviderPricingRepoSQLite(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	// 一条已过期（effective_to 已过）
	{
		from := now.Add(-2 * time.Hour)
		to := now.Add(-time.Hour)
		seedProviderPricing(t, ctx, repo, "openai", "gpt-4o", "token", 5e-6, from, &to)
	}
	// 一条尚未生效（effective_from > now）
	{
		from := now.Add(time.Hour)
		seedProviderPricing(t, ctx, repo, "openai", "gpt-4o", "token", 6e-6, from, nil)
	}

	got, err := repo.FindEffective(ctx, "openai", "gpt-4o", now)
	require.NoError(t, err)
	require.Nil(t, got, "时间窗口外不应命中任何行")
}

// TestProviderPricingRepository_FindEffective_LatestEffectiveFromWins 验证多条生效时取最新 effective_from。
func TestProviderPricingRepository_FindEffective_LatestEffectiveFromWins(t *testing.T) {
	repo, _ := newProviderPricingRepoSQLite(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	// 旧价：3 小时前生效，仍可用
	seedProviderPricing(t, ctx, repo, "anthropic", "claude-sonnet-4-6", "token", 3e-6,
		now.Add(-3*time.Hour), nil)
	// 新价：1 小时前生效（调价后），仍可用
	seedProviderPricing(t, ctx, repo, "anthropic", "claude-sonnet-4-6", "token", 4e-6,
		now.Add(-time.Hour), nil)

	got, err := repo.FindEffective(ctx, "anthropic", "claude-sonnet-4-6", now)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.InDelta(t, 4e-6, got.InputPrice, 1e-12, "应取最新 effective_from 的行（新价）")
}

// TestProviderPricingRepository_FindEffective_NoMatch 验证未命中返回 nil（不报错）。
func TestProviderPricingRepository_FindEffective_NoMatch(t *testing.T) {
	repo, _ := newProviderPricingRepoSQLite(t)
	ctx := context.Background()

	// 写一条只为 anthropic 服务的单价
	seedProviderPricing(t, ctx, repo, "anthropic", "claude-sonnet-4-6", "token", 3e-6,
		time.Now().Add(-time.Hour), nil)

	got, err := repo.FindEffective(ctx, "unknown-provider", "any-model", time.Now())
	require.NoError(t, err, "未命中不应返回错误")
	require.Nil(t, got, "未命中应返回 nil")
}

// TestProviderPricingRepository_FindEffective_EmptyArgs 验证空参数直接返回 nil（不查询）。
func TestProviderPricingRepository_FindEffective_EmptyArgs(t *testing.T) {
	repo, _ := newProviderPricingRepoSQLite(t)
	ctx := context.Background()

	now := time.Now()

	got, err := repo.FindEffective(ctx, "", "model", now)
	require.NoError(t, err)
	require.Nil(t, got, "provider 为空应返回 nil")

	got, err = repo.FindEffective(ctx, "anthropic", "", now)
	require.NoError(t, err)
	require.Nil(t, got, "model 为空应返回 nil")
}

// TestProviderPricingRepository_FindEffective_ExactBeatsWildcard 验证精确匹配优先于通配。
func TestProviderPricingRepository_FindEffective_ExactBeatsWildcard(t *testing.T) {
	repo, _ := newProviderPricingRepoSQLite(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-time.Hour)
	// 同时存在精确单价和通配单价，价格不同
	seedProviderPricing(t, ctx, repo, "anthropic", "claude-sonnet-4-6", "token", 3e-6, from, nil)
	seedProviderPricing(t, ctx, repo, "anthropic", "*", "token", 1e-5, from, nil)

	got, err := repo.FindEffective(ctx, "anthropic", "claude-sonnet-4-6", now)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "claude-sonnet-4-6", got.Model, "精确匹配优先")
	require.InDelta(t, 3e-6, got.InputPrice, 1e-12)
}
