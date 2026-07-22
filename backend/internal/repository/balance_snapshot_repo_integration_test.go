//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newTestBalanceSnapshotRepo(t *testing.T) (*balanceSnapshotRepository, context.Context) {
	t.Helper()
	tx := testEntTx(t)
	return &balanceSnapshotRepository{client: tx.Client()}, context.Background()
}

// TestBalanceSnapshotRepo_GetByUserPeriod_NotFoundReturnsNilNil 覆盖接口注释承诺的
// "未找到返回 (nil, nil)"（而不是返回 sql.ErrNoRows 之类的错误）。
func TestBalanceSnapshotRepo_GetByUserPeriod_NotFoundReturnsNilNil(t *testing.T) {
	repo, ctx := newTestBalanceSnapshotRepo(t)

	got, err := repo.GetByUserPeriod(ctx, 999999, "2026-01")
	require.NoError(t, err)
	require.Nil(t, got)
}

// TestBalanceSnapshotRepo_Upsert_CreatesThenReads 覆盖首次写入 + 读回的字段完整性。
func TestBalanceSnapshotRepo_Upsert_CreatesThenReads(t *testing.T) {
	repo, ctx := newTestBalanceSnapshotRepo(t)
	userID := time.Now().UnixNano()

	snap := &service.BalanceSnapshotRecord{
		UserID:                userID,
		Period:                "2026-06",
		OpeningBalance:        100,
		ClosingBalance:        80,
		DepositTotal:          50,
		WithdrawTotal:         10,
		CreditTotal:           5,
		UtilisationGrossTotal: 70,
		UtilisationTotal:      65,
	}
	require.NoError(t, repo.Upsert(ctx, snap))

	got, err := repo.GetByUserPeriod(ctx, userID, "2026-06")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, userID, got.UserID)
	require.Equal(t, "2026-06", got.Period)
	require.Equal(t, 100.0, got.OpeningBalance)
	require.Equal(t, 80.0, got.ClosingBalance)
	require.Equal(t, 50.0, got.DepositTotal)
	require.Equal(t, 10.0, got.WithdrawTotal)
	require.Equal(t, 5.0, got.CreditTotal)
	require.Equal(t, 70.0, got.UtilisationGrossTotal)
	require.Equal(t, 65.0, got.UtilisationTotal)
}

// TestBalanceSnapshotRepo_Upsert_IsIdempotentOnUserPeriod 覆盖 UNIQUE(user_id, period) 下的
// upsert 幂等性：同一 (user_id, period) 重复写入应更新而非报唯一键冲突错误，且新值完全覆盖旧值。
func TestBalanceSnapshotRepo_Upsert_IsIdempotentOnUserPeriod(t *testing.T) {
	repo, ctx := newTestBalanceSnapshotRepo(t)
	userID := time.Now().UnixNano()

	first := &service.BalanceSnapshotRecord{
		UserID: userID, Period: "2026-07",
		OpeningBalance: 100, ClosingBalance: 90,
		DepositTotal: 10, WithdrawTotal: 0, CreditTotal: 0,
		UtilisationGrossTotal: 10, UtilisationTotal: 10,
	}
	require.NoError(t, repo.Upsert(ctx, first))

	second := &service.BalanceSnapshotRecord{
		UserID: userID, Period: "2026-07",
		OpeningBalance: 90, ClosingBalance: 40,
		DepositTotal: 0, WithdrawTotal: 20, CreditTotal: 0,
		UtilisationGrossTotal: 30, UtilisationTotal: 30,
	}
	require.NoError(t, repo.Upsert(ctx, second))

	got, err := repo.GetByUserPeriod(ctx, userID, "2026-07")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 90.0, got.OpeningBalance, "重复 upsert 应以最新值覆盖，而不是保留首次写入值")
	require.Equal(t, 40.0, got.ClosingBalance)
	require.Equal(t, 20.0, got.WithdrawTotal)

	periods, err := repo.ListPeriodsByUser(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-07"}, periods, "幂等 upsert 不应产生第二行")
}

// TestBalanceSnapshotRepo_ListPeriodsByUser_OrderedAscending 覆盖跨月排序（按字典序即时间序）。
func TestBalanceSnapshotRepo_ListPeriodsByUser_OrderedAscending(t *testing.T) {
	repo, ctx := newTestBalanceSnapshotRepo(t)
	userID := time.Now().UnixNano()

	for _, period := range []string{"2026-03", "2025-12", "2026-01"} {
		require.NoError(t, repo.Upsert(ctx, &service.BalanceSnapshotRecord{
			UserID: userID, Period: period,
		}))
	}

	periods, err := repo.ListPeriodsByUser(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []string{"2025-12", "2026-01", "2026-03"}, periods)
}
