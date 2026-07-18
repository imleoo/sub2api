//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/statement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stmtFakeRepo 可编程的 StatementRepository fake。
type stmtFakeRepo struct {
	deposits    []StatementCashflow
	withdrawals []StatementCashflow
	credits     []StatementCashflow
	daily       []StatementDailyUsage
	monthNet    *StatementNetChange
	tailNet     *StatementNetChange
	monthStart  time.Time
	monthEnd    time.Time
}

func (f *stmtFakeRepo) ListDeposits(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error) {
	return f.deposits, nil
}

func (f *stmtFakeRepo) ListWithdrawals(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error) {
	return f.withdrawals, nil
}

func (f *stmtFakeRepo) ListCredits(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error) {
	return f.credits, nil
}

func (f *stmtFakeRepo) GetDailyUtilisation(ctx context.Context, userID int64, start, end time.Time, tzName string) ([]StatementDailyUsage, error) {
	return f.daily, nil
}

func (f *stmtFakeRepo) SumNetChange(ctx context.Context, userID int64, start, end time.Time) (*StatementNetChange, error) {
	if start.Equal(f.monthStart) && end.Equal(f.monthEnd) {
		if f.monthNet != nil {
			return f.monthNet, nil
		}
		return &StatementNetChange{}, nil
	}
	if f.tailNet != nil {
		return f.tailNet, nil
	}
	return &StatementNetChange{}, nil
}

func newStatementTestService(repo *stmtFakeRepo, snapRepo *fakeBalanceSnapshotRepo, balance float64, createdAt time.Time) *StatementService {
	userRepo := &snapshotUserRepoStub{user: &User{ID: 1, Email: "u@test.com", Balance: balance, CreatedAt: createdAt}}
	return NewStatementService(repo, snapRepo, nil, userRepo)
}

func TestGetStatement_ClosedMonthComputed(t *testing.T) {
	loc := time.UTC
	// 用 UTC 时区参数，月界即 UTC 自然月。
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	repo := &stmtFakeRepo{
		monthStart: monthStart,
		monthEnd:   monthEnd,
		deposits: []StatementCashflow{
			{Time: monthStart.AddDate(0, 0, 0), Amount: 5000, Note: "order A", Source: "payment_order"},
		},
		withdrawals: []StatementCashflow{
			{Time: monthStart.AddDate(0, 0, 3), Amount: 1000, Note: "refund A", Source: "refund"},
		},
		credits: []StatementCashflow{
			{Time: monthStart.AddDate(0, 0, 5), Amount: 10, Note: "redeem X", Source: "redeem"},
		},
		daily: []StatementDailyUsage{
			{Date: "2026-06-02", TotalTokens: 20000, CostBefore: 1000, CostAfter: 600},
		},
		monthNet: &StatementNetChange{Deposit: 5000, Withdraw: 1000, Credit: 10, UtilisationGross: 1000, Utilisation: 600},
		tailNet:  &StatementNetChange{Utilisation: 100}, // 月末至今又消费 100
	}
	svc := newStatementTestService(repo, newFakeBalanceSnapshotRepo(), 4310, monthStart.AddDate(0, -2, 0))

	st, err := svc.GetStatement(context.Background(), 1, "2026-06", "UTC")
	require.NoError(t, err)

	assert.Equal(t, "computed", st.Source)
	assert.True(t, st.Closed)
	// closing = 4310 − (−100) = 4410；opening = 4410 − (5000−1000+10−600) = 1000。
	assert.InDelta(t, 4410.0, st.ClosingBalance, 1e-9)
	assert.InDelta(t, 1000.0, st.OpeningBalance, 1e-9)

	// 行：opening + deposit + utilisation + withdraw + credit + closing = 6。
	require.Len(t, st.Rows, 6)
	assert.Equal(t, statement.NatureOpening, st.Rows[0].Nature)
	assert.Equal(t, statement.NatureDeposit, st.Rows[1].Nature)
	assert.Equal(t, statement.NatureUtilisation, st.Rows[2].Nature)
	assert.Equal(t, statement.NatureWithdraw, st.Rows[3].Nature)
	assert.Equal(t, statement.NatureCredit, st.Rows[4].Nature)
	assert.Equal(t, statement.NatureClosing, st.Rows[5].Nature)

	// Utilisation 反算列：Qty 负数、单价、折扣率。
	u := st.Rows[2]
	assert.Equal(t, int64(-20000), u.Qty)
	assert.InDelta(t, 0.05, u.UnitPrice, 1e-9)
	assert.InDelta(t, 0.4, u.DiscountRate, 1e-9)

	// 滚动余额：1000 → 6000 → 5400 → 4400 → 4410。
	assert.InDelta(t, 6000.0, st.Rows[1].RunningTotal, 1e-9)
	assert.InDelta(t, 5400.0, st.Rows[2].RunningTotal, 1e-9)
	assert.InDelta(t, 4400.0, st.Rows[3].RunningTotal, 1e-9)
	assert.InDelta(t, 4410.0, st.Rows[4].RunningTotal, 1e-9)

	// 合计与恒等式（闭合，gap=0）。
	assert.InDelta(t, 5000.0, st.Totals.Deposit, 1e-9)
	assert.InDelta(t, 1000.0, st.Totals.Withdraw, 1e-9)
	assert.InDelta(t, 10.0, st.Totals.Credit, 1e-9)
	assert.InDelta(t, 600.0, st.Totals.UtilisationAfter, 1e-9)
	assert.InDelta(t, 0.0, st.Totals.IdentityGap, 1e-9)
}

func TestGetStatement_SnapshotPreferredForClosedMonth(t *testing.T) {
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	repo := &stmtFakeRepo{monthStart: monthStart, monthEnd: monthStart.AddDate(0, 1, 0)}
	snapRepo := newFakeBalanceSnapshotRepo()
	snapRepo.snaps["2026-06"] = &BalanceSnapshotRecord{
		Period: "2026-06", OpeningBalance: 111, ClosingBalance: 222,
	}
	svc := newStatementTestService(repo, snapRepo, 999, monthStart.AddDate(0, -2, 0))

	st, err := svc.GetStatement(context.Background(), 1, "2026-06", "UTC")
	require.NoError(t, err)
	assert.Equal(t, "snapshot", st.Source)
	assert.InDelta(t, 111.0, st.OpeningBalance, 1e-9)
	assert.InDelta(t, 222.0, st.ClosingBalance, 1e-9)
}

func TestGetStatement_CurrentMonthNotClosed(t *testing.T) {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	repo := &stmtFakeRepo{monthStart: monthStart, monthEnd: monthStart.AddDate(0, 1, 0)}
	// 即使当月存在快照（不应存在），也必须走反推。
	snapRepo := newFakeBalanceSnapshotRepo()
	snapRepo.snaps[monthStart.Format("2006-01")] = &BalanceSnapshotRecord{OpeningBalance: -1, ClosingBalance: -1}
	svc := newStatementTestService(repo, snapRepo, 88, monthStart.AddDate(0, -2, 0))

	st, err := svc.GetStatement(context.Background(), 1, monthStart.Format("2006-01"), "UTC")
	require.NoError(t, err)
	assert.False(t, st.Closed)
	assert.Equal(t, "computed", st.Source)
	// 当月期末 = 当前余额（tail 区间为空）。
	assert.InDelta(t, 88.0, st.ClosingBalance, 1e-9)
}

func TestGetStatement_IdentityGapExposed(t *testing.T) {
	// 快照期初 500，但流水只有 +100，期末快照 700 → gap = 700 − 600 = 100。
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	repo := &stmtFakeRepo{
		monthStart: monthStart,
		monthEnd:   monthStart.AddDate(0, 1, 0),
		deposits:   []StatementCashflow{{Time: monthStart, Amount: 100}},
	}
	snapRepo := newFakeBalanceSnapshotRepo()
	snapRepo.snaps["2026-06"] = &BalanceSnapshotRecord{OpeningBalance: 500, ClosingBalance: 700}
	svc := newStatementTestService(repo, snapRepo, 0, monthStart.AddDate(0, -1, 0))

	st, err := svc.GetStatement(context.Background(), 1, "2026-06", "UTC")
	require.NoError(t, err)
	assert.InDelta(t, 100.0, st.Totals.IdentityGap, 1e-9)
}

func TestGetStatement_InvalidInput(t *testing.T) {
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	repo := &stmtFakeRepo{monthStart: monthStart, monthEnd: monthStart.AddDate(0, 1, 0)}
	svc := newStatementTestService(repo, newFakeBalanceSnapshotRepo(), 0, monthStart)

	_, err := svc.GetStatement(context.Background(), 1, "2026/06", "UTC")
	assert.Error(t, err)
	_, err = svc.GetStatement(context.Background(), 1, "2999-01", "UTC")
	assert.Error(t, err, "future month rejected")
	_, err = svc.GetStatement(context.Background(), 1, "2026-06", "Not/AZone")
	assert.Error(t, err)
}

func TestListAvailableMonths(t *testing.T) {
	repo := &stmtFakeRepo{}
	// 注册于 3 个月前 → 应返回 4 个月（注册月起含当月），倒序。
	created := time.Now().AddDate(0, -3, 0)
	svc := newStatementTestService(repo, newFakeBalanceSnapshotRepo(), 0, created)

	months, err := svc.ListAvailableMonths(context.Background(), 1)
	require.NoError(t, err)
	require.NotEmpty(t, months)
	assert.Equal(t, time.Now().Format("2006-01"), months[0], "最近月份在前")
	for i := 1; i < len(months); i++ {
		assert.Greater(t, months[i-1], months[i], "严格倒序")
	}
}
