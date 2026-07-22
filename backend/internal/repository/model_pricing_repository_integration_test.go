//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newTestModelPricingRepo(t *testing.T) (*modelPricingRepository, context.Context) {
	t.Helper()
	tx := testEntTx(t)
	return &modelPricingRepository{client: tx.Client()}, context.Background()
}

// newTestModelPricingRepoGlobal 用于测试 BulkUpsertMaas：该方法内部自行开启
// r.client.Tx(ctx)，若 client 本身已是 testEntTx 包出的事务 client 会报
// "ent: cannot start a transaction within a transaction"。改用全局 client（不
// 自动回滚），配合 uniqueModelID 生成的随机 model_id 避免跨用例数据冲突；
// 底层 postgres 由 testcontainers 按整个测试进程生命周期管理，进程结束即销毁，
// 不会有真实的数据残留问题。
func newTestModelPricingRepoGlobal(t *testing.T) (*modelPricingRepository, context.Context) {
	t.Helper()
	return &modelPricingRepository{client: testEntClient(t)}, context.Background()
}

func uniqueModelID(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func float64Ptr(v float64) *float64 { return &v }

// TestModelPricingRepo_UpsertBatch_SkipsCustomRecords 覆盖 UpsertBatch 的核心承诺：
// is_custom=true 的记录不会被远端同步覆盖。
func TestModelPricingRepo_UpsertBatch_SkipsCustomRecords(t *testing.T) {
	repo, ctx := newTestModelPricingRepo(t)

	customID := uniqueModelID(t, "custom-model")
	syncedID := uniqueModelID(t, "synced-model")

	// 先手动创建一条 is_custom=true 的记录，价格 1.0
	require.NoError(t, repo.Create(ctx, &service.DBModelPricing{
		ModelID:           customID,
		Provider:          "openai",
		Mode:              "chat",
		PricingUnit:       service.ModelPricingUnitToken,
		IsCustom:          true,
		IsEnabled:         true,
		InputCostPerToken: float64Ptr(1.0),
	}))

	// 远端同步批次尝试覆盖 custom 记录为 9.99，并新增一条 synced 记录
	err := repo.UpsertBatch(ctx, []*service.DBModelPricing{
		{
			ModelID:           customID,
			Provider:          "openai",
			Mode:              "chat",
			PricingUnit:       service.ModelPricingUnitToken,
			IsEnabled:         true,
			InputCostPerToken: float64Ptr(9.99),
		},
		{
			ModelID:           syncedID,
			Provider:          "anthropic",
			Mode:              "chat",
			PricingUnit:       service.ModelPricingUnitToken,
			IsEnabled:         true,
			InputCostPerToken: float64Ptr(2.5),
		},
	})
	require.NoError(t, err)

	custom, err := repo.GetByModelID(ctx, customID)
	require.NoError(t, err)
	require.NotNil(t, custom.InputCostPerToken)
	require.Equal(t, 1.0, *custom.InputCostPerToken, "is_custom=true 的记录不应被远端同步覆盖")

	synced, err := repo.GetByModelID(ctx, syncedID)
	require.NoError(t, err)
	require.NotNil(t, synced.InputCostPerToken)
	require.Equal(t, 2.5, *synced.InputCostPerToken)
	require.False(t, synced.IsCustom)
}

// TestModelPricingRepo_UpsertBatch_UpdatesExistingSyncedRecord 覆盖非 custom 记录的正常更新路径。
func TestModelPricingRepo_UpsertBatch_UpdatesExistingSyncedRecord(t *testing.T) {
	repo, ctx := newTestModelPricingRepo(t)
	modelID := uniqueModelID(t, "resync-model")

	require.NoError(t, repo.UpsertBatch(ctx, []*service.DBModelPricing{
		{ModelID: modelID, Provider: "openai", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, IsEnabled: true, InputCostPerToken: float64Ptr(1.0)},
	}))
	require.NoError(t, repo.UpsertBatch(ctx, []*service.DBModelPricing{
		{ModelID: modelID, Provider: "openai", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, IsEnabled: true, InputCostPerToken: float64Ptr(3.0)},
	}))

	got, err := repo.GetByModelID(ctx, modelID)
	require.NoError(t, err)
	require.NotNil(t, got.InputCostPerToken)
	require.Equal(t, 3.0, *got.InputCostPerToken)
}

// TestModelPricingRepo_BulkUpsertMaas 覆盖三条分支：不存在则新建（is_custom=true）、
// is_custom=true 已存在则更新（provider 仅在原记录为空时补全）、is_custom=false（LiteLLM）已存在则跳过。
func TestModelPricingRepo_BulkUpsertMaas(t *testing.T) {
	repo, ctx := newTestModelPricingRepoGlobal(t)

	newModelID := uniqueModelID(t, "maas-new")
	customModelID := uniqueModelID(t, "maas-custom")
	litellmModelID := uniqueModelID(t, "maas-litellm")

	// 预置一条 is_custom=true 的记录（provider 为空，等待 MaaS 同步补全）与一条 LiteLLM 记录。
	require.NoError(t, repo.Create(ctx, &service.DBModelPricing{
		ModelID: customModelID, Provider: "", Mode: "chat",
		PricingUnit: service.ModelPricingUnitToken, IsCustom: true, IsEnabled: true,
		InputCostPerToken: float64Ptr(1.0),
	}))
	require.NoError(t, repo.UpsertBatch(ctx, []*service.DBModelPricing{
		{ModelID: litellmModelID, Provider: "openai", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, IsEnabled: true, InputCostPerToken: float64Ptr(1.0)},
	}))

	err := repo.BulkUpsertMaas(ctx, []*service.DBModelPricing{
		{ModelID: newModelID, Provider: "wanjie", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, Source: "wanjie", PricingStatus: service.ModelPricingStatusPriced, InputCostPerToken: float64Ptr(4.0)},
		{ModelID: customModelID, Provider: "wanjie", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, Source: "wanjie", PricingStatus: service.ModelPricingStatusPriced, InputCostPerToken: float64Ptr(5.0)},
		{ModelID: litellmModelID, Provider: "wanjie", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, Source: "wanjie", PricingStatus: service.ModelPricingStatusPriced, InputCostPerToken: float64Ptr(6.0)},
	})
	require.NoError(t, err)

	created, err := repo.GetByModelID(ctx, newModelID)
	require.NoError(t, err)
	require.True(t, created.IsCustom, "不存在的记录应以 is_custom=true 新建")
	require.Equal(t, 4.0, *created.InputCostPerToken)

	updated, err := repo.GetByModelID(ctx, customModelID)
	require.NoError(t, err)
	require.Equal(t, "wanjie", updated.Provider, "原 provider 为空时应被 MaaS 数据补全")
	require.Equal(t, 5.0, *updated.InputCostPerToken)

	untouched, err := repo.GetByModelID(ctx, litellmModelID)
	require.NoError(t, err)
	require.Equal(t, 1.0, *untouched.InputCostPerToken, "is_custom=false（LiteLLM 来源）记录必须原样跳过")
}

// TestModelPricingRepo_BulkUpsertMaas_TierPricingRoundTrip 覆盖视频分档单价的写入/清空。
func TestModelPricingRepo_BulkUpsertMaas_TierPricingRoundTrip(t *testing.T) {
	repo, ctx := newTestModelPricingRepoGlobal(t)
	modelID := uniqueModelID(t, "maas-tier")

	tiers := []service.VideoPriceTier{{Spec: "在线推理-1080p", CNYPerMToken: 20}}
	require.NoError(t, repo.BulkUpsertMaas(ctx, []*service.DBModelPricing{
		{ModelID: modelID, Provider: "wanjie", Mode: "video_generation", PricingUnit: service.ModelPricingUnitVideo, Source: "wanjie", PricingStatus: service.ModelPricingStatusPriced, TierPricing: tiers},
	}))

	got, err := repo.GetByModelID(ctx, modelID)
	require.NoError(t, err)
	require.Equal(t, tiers, got.TierPricing)

	// 二次同步不带 tier_pricing，应被清空。
	require.NoError(t, repo.BulkUpsertMaas(ctx, []*service.DBModelPricing{
		{ModelID: modelID, Provider: "wanjie", Mode: "video_generation", PricingUnit: service.ModelPricingUnitVideo, Source: "wanjie", PricingStatus: service.ModelPricingStatusPriced},
	}))
	got2, err := repo.GetByModelID(ctx, modelID)
	require.NoError(t, err)
	require.Nil(t, got2.TierPricing)
}

// TestModelPricingRepo_BulkUpdateDiscountRates_And_ClearAll 覆盖折扣批量设置与清空的影响范围。
func TestModelPricingRepo_BulkUpdateDiscountRates_And_ClearAll(t *testing.T) {
	repo, ctx := newTestModelPricingRepo(t)
	modelA := uniqueModelID(t, "discount-a")
	modelB := uniqueModelID(t, "discount-b")

	require.NoError(t, repo.UpsertBatch(ctx, []*service.DBModelPricing{
		{ModelID: modelA, Provider: "openai", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, IsEnabled: true},
		{ModelID: modelB, Provider: "openai", Mode: "chat", PricingUnit: service.ModelPricingUnitToken, IsEnabled: true},
	}))

	require.NoError(t, repo.BulkUpdateDiscountRates(ctx, map[string]float64{
		modelA: 0.8,
		modelB: 0.5,
	}))

	gotA, err := repo.GetByModelID(ctx, modelA)
	require.NoError(t, err)
	require.NotNil(t, gotA.DiscountRate)
	require.Equal(t, 0.8, *gotA.DiscountRate)

	require.NoError(t, repo.ClearAllDiscountRates(ctx))

	gotA2, err := repo.GetByModelID(ctx, modelA)
	require.NoError(t, err)
	require.Nil(t, gotA2.DiscountRate, "ClearAllDiscountRates 应清空所有记录的折扣率")

	gotB2, err := repo.GetByModelID(ctx, modelB)
	require.NoError(t, err)
	require.Nil(t, gotB2.DiscountRate)
}

// TestModelPricingRepo_BulkUpdateDiscountRates_EmptyMapNoop 空 map 不应报错也不产生任何写操作。
func TestModelPricingRepo_BulkUpdateDiscountRates_EmptyMapNoop(t *testing.T) {
	repo, ctx := newTestModelPricingRepo(t)
	require.NoError(t, repo.BulkUpdateDiscountRates(ctx, map[string]float64{}))
	require.NoError(t, repo.BulkUpdateDiscountRates(ctx, nil))
}
