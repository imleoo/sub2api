//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStatementRepo 以 [start,end) 区间为 key 返回预置净变动。
type fakeStatementRepo struct {
	netByRange map[string]*StatementNetChange
}

func rangeKey(start, end time.Time) string {
	return start.UTC().Format(time.RFC3339) + "|" + end.UTC().Format(time.RFC3339)
}

func (f *fakeStatementRepo) ListDeposits(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error) {
	return nil, nil
}

func (f *fakeStatementRepo) ListWithdrawals(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error) {
	return nil, nil
}

func (f *fakeStatementRepo) ListCredits(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error) {
	return nil, nil
}

func (f *fakeStatementRepo) GetDailyUtilisation(ctx context.Context, userID int64, start, end time.Time, tzName string) ([]StatementDailyUsage, error) {
	return nil, nil
}

func (f *fakeStatementRepo) SumNetChange(ctx context.Context, userID int64, start, end time.Time) (*StatementNetChange, error) {
	if c, ok := f.netByRange[rangeKey(start, end)]; ok {
		return c, nil
	}
	return &StatementNetChange{}, nil
}

// fakeBalanceSnapshotRepo 内存快照存储。
type fakeBalanceSnapshotRepo struct {
	snaps map[string]*BalanceSnapshotRecord // key: period（单用户测试）
}

func newFakeBalanceSnapshotRepo() *fakeBalanceSnapshotRepo {
	return &fakeBalanceSnapshotRepo{snaps: map[string]*BalanceSnapshotRecord{}}
}

func (f *fakeBalanceSnapshotRepo) Upsert(ctx context.Context, snap *BalanceSnapshotRecord) error {
	cp := *snap
	f.snaps[snap.Period] = &cp
	return nil
}

func (f *fakeBalanceSnapshotRepo) GetByUserPeriod(ctx context.Context, userID int64, period string) (*BalanceSnapshotRecord, error) {
	return f.snaps[period], nil
}

func (f *fakeBalanceSnapshotRepo) ListPeriodsByUser(ctx context.Context, userID int64) ([]string, error) {
	var out []string
	for p := range f.snaps {
		out = append(out, p)
	}
	return out, nil
}

// snapshotUserRepoStub 只实现月结用到的 UserRepository 方法。
type snapshotUserRepoStub struct {
	UserRepository
	user *User
}

func (s *snapshotUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	return s.user, nil
}

func (s *snapshotUserRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return []User{*s.user}, &pagination.PaginationResult{Pages: 1}, nil
}

func TestSettleDuePeriod(t *testing.T) {
	loc := timezone.Location()

	// 1 日 00:10（未到 00:30）：未到月结时点。
	_, due := settleDuePeriod(time.Date(2026, 7, 1, 0, 10, 0, 0, loc))
	assert.False(t, due)

	// 1 日 00:30：到点，结算上月。
	period, due := settleDuePeriod(time.Date(2026, 7, 1, 0, 30, 0, 0, loc))
	assert.True(t, due)
	assert.Equal(t, "2026-06", period)

	// 月中任意时刻：仍返回上月（幂等补算窗口贯穿全月）。
	period, due = settleDuePeriod(time.Date(2026, 7, 15, 12, 0, 0, 0, loc))
	assert.True(t, due)
	assert.Equal(t, "2026-06", period)

	// 跨年：1 月结算上年 12 月。
	period, due = settleDuePeriod(time.Date(2026, 1, 2, 0, 0, 0, 0, loc))
	assert.True(t, due)
	assert.Equal(t, "2025-12", period)
}

func TestStatementPeriodBounds(t *testing.T) {
	start, end, err := statementPeriodBounds("2026-06")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, timezone.Location()), start)
	assert.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, timezone.Location()), end)

	_, _, err = statementPeriodBounds("2026-6")
	assert.Error(t, err)
	_, _, err = statementPeriodBounds("bogus")
	assert.Error(t, err)
}

func TestRecomputeForUserMonth_BackwardsFromBalance(t *testing.T) {
	monthStart, monthEnd, err := statementPeriodBounds("2026-06")
	require.NoError(t, err)

	// 6 月净变动：+100 充值 −30 消费 = +70；月末至今净变动：−20 消费。
	stmtRepo := &fakeStatementRepo{netByRange: map[string]*StatementNetChange{}}
	stmtRepo.netByRange[rangeKey(monthStart, monthEnd)] = &StatementNetChange{
		Deposit: 100, Utilisation: 30, UtilisationGross: 60,
	}
	// 月末→now 的区间 end 是动态 timezone.Now()，fake 里用默认零值以外的
	// 匹配不可行；改为直接枚举：所有非本月区间返回 tail。
	tail := &StatementNetChange{Utilisation: 20}
	stmtRepoWithTail := &fakeStatementRepoTail{
		monthKey: rangeKey(monthStart, monthEnd),
		month:    stmtRepo.netByRange[rangeKey(monthStart, monthEnd)],
		tail:     tail,
	}

	snapRepo := newFakeBalanceSnapshotRepo()
	userRepo := &snapshotUserRepoStub{user: &User{ID: 7, Balance: 150, CreatedAt: monthStart.AddDate(0, -3, 0)}}
	svc := NewBalanceSnapshotService(stmtRepoWithTail, snapRepo, userRepo, time.Hour)

	snap, err := svc.RecomputeForUserMonth(context.Background(), 7, "2026-06")
	require.NoError(t, err)

	// closing = 150 − (−20) 消费为净减 → net = −20 → closing = 150 − (−20)？
	// tail.Net() = 0 − 0 + 0 − 20 = −20；closing = 150 − (−20) = 170。
	assert.InDelta(t, 170.0, snap.ClosingBalance, 1e-9)
	// opening = closing − monthNet = 170 − (100−30) = 100（无上月快照）。
	assert.InDelta(t, 100.0, snap.OpeningBalance, 1e-9)
	assert.InDelta(t, 100.0, snap.DepositTotal, 1e-9)
	assert.InDelta(t, 30.0, snap.UtilisationTotal, 1e-9)
	assert.InDelta(t, 60.0, snap.UtilisationGrossTotal, 1e-9)

	// 恒等式：closing = opening + deposit − withdraw + credit − utilisation。
	assert.InDelta(t, snap.ClosingBalance,
		snap.OpeningBalance+snap.DepositTotal-snap.WithdrawTotal+snap.CreditTotal-snap.UtilisationTotal, 1e-9)
}

func TestRecomputeForUserMonth_PrefersPrevSnapshotOpening(t *testing.T) {
	monthStart, monthEnd, err := statementPeriodBounds("2026-06")
	require.NoError(t, err)

	stmtRepo := &fakeStatementRepoTail{
		monthKey: rangeKey(monthStart, monthEnd),
		month:    &StatementNetChange{Deposit: 50},
		tail:     &StatementNetChange{},
	}
	snapRepo := newFakeBalanceSnapshotRepo()
	// 预置 5 月快照：期末 88 —— 6 月期初必须精确取它而非反推值。
	snapRepo.snaps["2026-05"] = &BalanceSnapshotRecord{Period: "2026-05", ClosingBalance: 88}

	userRepo := &snapshotUserRepoStub{user: &User{ID: 7, Balance: 140, CreatedAt: monthStart.AddDate(0, -6, 0)}}
	svc := NewBalanceSnapshotService(stmtRepo, snapRepo, userRepo, time.Hour)

	snap, err := svc.RecomputeForUserMonth(context.Background(), 7, "2026-06")
	require.NoError(t, err)
	assert.InDelta(t, 88.0, snap.OpeningBalance, 1e-9)
	assert.InDelta(t, 140.0, snap.ClosingBalance, 1e-9) // tail 净变动 0

	// 幂等：重算两次结果一致。
	again, err := svc.RecomputeForUserMonth(context.Background(), 7, "2026-06")
	require.NoError(t, err)
	assert.Equal(t, snap.OpeningBalance, again.OpeningBalance)
	assert.Equal(t, snap.ClosingBalance, again.ClosingBalance)
}

// fakeStatementRepoTail 精确匹配本月区间，其余区间一律返回 tail（模拟月末→now 动态区间）。
type fakeStatementRepoTail struct {
	fakeStatementRepo
	monthKey string
	month    *StatementNetChange
	tail     *StatementNetChange
}

func (f *fakeStatementRepoTail) SumNetChange(ctx context.Context, userID int64, start, end time.Time) (*StatementNetChange, error) {
	if rangeKey(start, end) == f.monthKey {
		return f.month, nil
	}
	return f.tail, nil
}
