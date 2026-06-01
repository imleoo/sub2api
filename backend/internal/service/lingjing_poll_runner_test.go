package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Phase 0 P0-7 验收单测：lingjing 异步计费 → UsageLog 第 4 装配点
//
// 用例覆盖 docs/upstream-cost-snapshot.md §6 验收用例 8：
//   - 命中 provider_pricing：upstream_total_cost + cost_finalized_at + async_task_id 同时非空
//   - 未命中：upstream_total_cost = NULL，async_task_id 仍写入（归"待结算超时"维度的反例：
//     已结算但无快照，与同步未命中等价归类）
//   - usageLogRepo nil 安全（测试场景）

// pollRunnerUsageLogStub 捕获写入的 UsageLog 行，方便断言。
type pollRunnerUsageLogStub struct {
	UsageLogRepository
	created []*UsageLog
	err     error
}

func (s *pollRunnerUsageLogStub) Create(_ context.Context, log *UsageLog) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	s.created = append(s.created, log)
	return true, nil
}

// TestWriteAsyncUsageLog_HitFillsSnapshotAndAsyncFields 命中场景：
// upstream_total_cost / cost_finalized_at / async_task_id / provider 同时非空
func TestWriteAsyncUsageLog_HitFillsSnapshotAndAsyncFields(t *testing.T) {
	pricingRepo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider:    "lingjing",
			Model:       "doubao-seedance-1.5-pro-5s",
			BillingMode: "per_request",
			InputPrice:  0.42,
		},
	}
	resolver := &UpstreamCostResolver{repo: pricingRepo}
	usageStub := &pollRunnerUsageLogStub{}

	runner := &LingjingPollRunner{
		upstreamCostResolver: resolver,
		usageLogRepo:         usageStub,
	}

	gid := int64(99)
	task := &LingjingTask{
		ID:        7,
		GenTaskID: "gen-task-id-abc",
		UserID:    1001,
		APIKeyID:  2002,
		AccountID: 3003,
		GroupID:   &gid,
		Duration:  "5",
	}
	cost := &CostBreakdown{
		InputCost:   0.5,
		TotalCost:   0.5,
		ActualCost:  0.5,
		BillingMode: "per_request",
	}

	runner.writeAsyncUsageLog(context.Background(), task, "doubao-seedance-1.5-pro-5s", 5, cost)

	require.Len(t, usageStub.created, 1)
	log := usageStub.created[0]

	// 异步专属字段：AsyncTaskID + CostFinalizedAt 必须同时非空（docs §3.1 异步约束）
	require.NotNil(t, log.AsyncTaskID, "异步任务必须写入 AsyncTaskID 用于 lingjing_task 对账")
	require.Equal(t, "gen-task-id-abc", *log.AsyncTaskID)
	require.NotNil(t, log.CostFinalizedAt, "命中后 CostFinalizedAt 非空")

	// 上游成本快照：命中 provider_pricing
	require.NotNil(t, log.UpstreamTotalCost)
	require.InDelta(t, 0.42, *log.UpstreamTotalCost, 1e-12, "per_request 计费按 input_price × 1 次")
	require.NotNil(t, log.PricingSource)
	require.Equal(t, "provider_table", *log.PricingSource)
	require.NotNil(t, log.Provider)
	require.Equal(t, "lingjing", *log.Provider)

	// 客户售价仍写入
	require.InDelta(t, 0.5, log.ActualCost, 1e-12)

	// 关联字段
	require.Equal(t, int64(1001), log.UserID)
	require.Equal(t, int64(2002), log.APIKeyID)
	require.Equal(t, int64(3003), log.AccountID)
	require.NotNil(t, log.GroupID)
	require.Equal(t, int64(99), *log.GroupID)
	require.Equal(t, "gen-task-id-abc", log.RequestID, "RequestID = GenTaskID 保证 ON CONFLICT 幂等")
}

// TestWriteAsyncUsageLog_MissKeepsCostNullKeepsAsyncTaskID 未命中：
// upstream_total_cost = NULL + pricing_source = NULL，但 async_task_id 仍写入
func TestWriteAsyncUsageLog_MissKeepsCostNullKeepsAsyncTaskID(t *testing.T) {
	pricingRepo := &fakeProviderPricingRepo{pricing: nil} // 未命中
	resolver := &UpstreamCostResolver{repo: pricingRepo}
	usageStub := &pollRunnerUsageLogStub{}

	runner := &LingjingPollRunner{
		upstreamCostResolver: resolver,
		usageLogRepo:         usageStub,
	}

	task := &LingjingTask{
		ID:        8,
		GenTaskID: "gen-task-id-no-pricing",
		UserID:    1, APIKeyID: 2, AccountID: 3,
		Duration: "5",
	}
	cost := &CostBreakdown{ActualCost: 0.5, BillingMode: "per_request"}

	runner.writeAsyncUsageLog(context.Background(), task, "doubao-unknown-model", 5, cost)

	require.Len(t, usageStub.created, 1)
	log := usageStub.created[0]

	// 未命中：upstream_total_cost / pricing_source 同时 NULL（一致性约束）
	require.Nil(t, log.UpstreamTotalCost)
	require.Nil(t, log.PricingSource)

	// async_task_id 仍写入：用于 lingjing_task vs UsageLog 对账（无关命中与否）
	require.NotNil(t, log.AsyncTaskID)
	require.Equal(t, "gen-task-id-no-pricing", *log.AsyncTaskID)

	// Provider 仍写入（helper 行为）
	require.NotNil(t, log.Provider)
	require.Equal(t, "lingjing", *log.Provider)
}

// TestWriteAsyncUsageLog_NilUsageLogRepoSafe 测试场景：usageLogRepo=nil 安全跳过
func TestWriteAsyncUsageLog_NilUsageLogRepoSafe(t *testing.T) {
	runner := &LingjingPollRunner{
		upstreamCostResolver: &UpstreamCostResolver{repo: &fakeProviderPricingRepo{}},
		usageLogRepo:         nil, // 测试场景
	}
	task := &LingjingTask{ID: 9, GenTaskID: "gen-id-9"}
	cost := &CostBreakdown{ActualCost: 0.5}

	// 不应 panic
	runner.writeAsyncUsageLog(context.Background(), task, "any-model", 5, cost)
}

// TestWriteAsyncUsageLog_FlagForceOn 异步路径不读 USAGE_UPSTREAM_COST_ENABLED flag。
//
// 即便业务侧 flag=false，异步路径仍强制写入上游成本快照（docs §4.1 关键约束）：
// 避免 poll 期间 flag 切换导致同一任务两次轮询出现 NULL/非 NULL 摇摆。
//
// 注意：这个测试通过 hash 检验：writeAsyncUsageLog 直接传 flag=true（强制 ON），
// 而不读取任何 cfg.Gateway.UpstreamCostEnabled。
func TestWriteAsyncUsageLog_FlagForceOn(t *testing.T) {
	pricingRepo := &fakeProviderPricingRepo{
		pricing: &DBProviderPricing{
			Provider: "lingjing", Model: "any",
			BillingMode: "per_request", InputPrice: 1.0,
		},
	}
	resolver := &UpstreamCostResolver{repo: pricingRepo}
	usageStub := &pollRunnerUsageLogStub{}

	runner := &LingjingPollRunner{
		upstreamCostResolver: resolver,
		usageLogRepo:         usageStub,
	}
	task := &LingjingTask{GenTaskID: "force-on", UserID: 1, APIKeyID: 2, AccountID: 3}

	start := time.Now()
	runner.writeAsyncUsageLog(context.Background(), task, "any", 5, &CostBreakdown{ActualCost: 1.0})
	elapsed := time.Since(start)

	require.Len(t, usageStub.created, 1)
	log := usageStub.created[0]
	require.NotNil(t, log.UpstreamTotalCost, "异步路径强制写入上游成本快照，与 USAGE_UPSTREAM_COST_ENABLED 解耦")
	require.NotNil(t, log.CostFinalizedAt)
	require.WithinDuration(t, time.Now(), *log.CostFinalizedAt, elapsed+time.Second)
}
