//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callCountUserRepo 记录 List 调用次数，用于证明 settlePeriod/runOnce 是否真正跑过分页扫描。
// listCalls 用 atomic 读写：后台 goroutine（Start 启动的轮询循环）与测试主 goroutine
// 会并发访问该字段，普通 int 会在 -race 下报数据竞争。
type callCountUserRepo struct {
	UserRepository
	users     []User
	listCalls atomic.Int64
}

func (s *callCountUserRepo) List(ctx context.Context, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	s.listCalls.Add(1)
	return s.users, &pagination.PaginationResult{Pages: 1}, nil
}

func TestRunOnce_AlreadySettled_SkipsRescan(t *testing.T) {
	period, _ := settleDuePeriod(timezone.Now())

	userRepo := &callCountUserRepo{}
	svc := NewBalanceSnapshotService(&fakeStatementRepo{netByRange: map[string]*StatementNetChange{}}, newFakeBalanceSnapshotRepo(), userRepo, time.Hour)
	svc.lastSettledPeriod = period

	svc.runOnce()

	assert.Equal(t, int64(0), userRepo.listCalls.Load(), "already-settled period must not trigger a rescan")
}

func TestRunOnce_FullRun_SettlesAndRecordsPeriod(t *testing.T) {
	now := timezone.Now()
	period, due := settleDuePeriod(now)
	if !due {
		t.Skip("test window falls within the 30-minute pre-settlement buffer; skip to avoid flakiness")
	}

	monthStart, monthEnd, err := statementPeriodBounds(period)
	require.NoError(t, err)

	userRepo := &callCountUserRepo{users: []User{
		{ID: 1, Balance: 100, CreatedAt: monthStart.AddDate(0, -1, 0)},
	}}
	snapRepo := newFakeBalanceSnapshotRepo()
	stmtRepo := &fakeStatementRepoTail{
		monthKey: rangeKey(monthStart, monthEnd),
		month:    &StatementNetChange{Deposit: 10},
		tail:     &StatementNetChange{},
	}

	svc := NewBalanceSnapshotService(stmtRepo, snapRepo, userRepo, time.Hour)
	svc.runOnce()

	assert.Equal(t, int64(1), userRepo.listCalls.Load(), "expected settlePeriod to page through users")
	assert.Equal(t, period, svc.lastSettledPeriod)
	assert.NotNil(t, snapRepo.snaps[period], "expected a snapshot to be recorded for the due period")

	// 再次调用应短路，不重复扫描（lastSettledPeriod 已记录该期）。
	svc.runOnce()
	assert.Equal(t, int64(1), userRepo.listCalls.Load(), "second runOnce within the same period should be a no-op")
}

func TestSettlePeriod_SkipsUsersCreatedAfterMonthEnd(t *testing.T) {
	monthStart, monthEnd, err := statementPeriodBounds("2026-06")
	require.NoError(t, err)

	// fakeBalanceSnapshotRepo 按 period 单键存储（不区分 userID），据此用最终存储值
	// 反推：若被跳过的用户 2（closing 应为 200）未出现在结果里，说明确实被过滤。
	userRepo := &callCountUserRepo{users: []User{
		{ID: 1, Balance: 50, CreatedAt: monthStart.AddDate(0, -1, 0)},  // 月结算范围内，应结算
		{ID: 2, Balance: 200, CreatedAt: monthEnd.Add(24 * time.Hour)}, // 月末之后才注册，应跳过
	}}
	snapRepo := newFakeBalanceSnapshotRepo()
	stmtRepo := &fakeStatementRepoTail{
		monthKey: rangeKey(monthStart, monthEnd),
		month:    &StatementNetChange{Deposit: 5},
		tail:     &StatementNetChange{},
	}
	svc := NewBalanceSnapshotService(stmtRepo, snapRepo, userRepo, time.Hour)

	err = svc.settlePeriod(context.Background(), "2026-06")
	require.NoError(t, err)

	require.NotNil(t, snapRepo.snaps["2026-06"])
	assert.Equal(t, 50.0, snapRepo.snaps["2026-06"].ClosingBalance,
		"only user 1 (created before month end) should have been settled; user 2 must be skipped")
}

func TestSettlePeriod_SkipsExistingSnapshot(t *testing.T) {
	monthStart, monthEnd, err := statementPeriodBounds("2026-06")
	require.NoError(t, err)

	userRepo := &callCountUserRepo{users: []User{
		{ID: 1, Balance: 999, CreatedAt: monthStart.AddDate(0, -1, 0)},
	}}
	snapRepo := newFakeBalanceSnapshotRepo()
	// 预置该月已有快照：期望 settlePeriod 跳过重算，不覆盖已有值。
	snapRepo.snaps["2026-06"] = &BalanceSnapshotRecord{Period: "2026-06", ClosingBalance: 12345}

	stmtRepo := &fakeStatementRepoTail{
		monthKey: rangeKey(monthStart, monthEnd),
		month:    &StatementNetChange{Deposit: 999},
		tail:     &StatementNetChange{},
	}
	svc := NewBalanceSnapshotService(stmtRepo, snapRepo, userRepo, time.Hour)

	err = svc.settlePeriod(context.Background(), "2026-06")
	require.NoError(t, err)

	assert.Equal(t, 12345.0, snapRepo.snaps["2026-06"].ClosingBalance, "existing snapshot must not be overwritten")
}

func TestBalanceSnapshotService_Start_GuardsIncompleteDeps(t *testing.T) {
	// 缺 snapRepo：Start 应直接返回，不启动后台 goroutine（Stop 立即返回不阻塞）。
	svc := NewBalanceSnapshotService(&fakeStatementRepo{}, nil, &callCountUserRepo{}, time.Hour)
	svc.Start()
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() should return immediately when Start() never spawned a goroutine")
	}
}

func TestBalanceSnapshotService_NilReceiver_NoPanic(t *testing.T) {
	var svc *BalanceSnapshotService
	assert.NotPanics(t, func() {
		svc.Start()
		svc.Stop()
		svc.SetLeaderLock(nil, nil)
	})
}

func TestBalanceSnapshotService_StartStop_RunsInBackground(t *testing.T) {
	now := timezone.Now()
	period, due := settleDuePeriod(now)
	if !due {
		t.Skip("test window falls within the 30-minute pre-settlement buffer; skip to avoid flakiness")
	}
	monthStart, monthEnd, err := statementPeriodBounds(period)
	require.NoError(t, err)

	userRepo := &callCountUserRepo{users: []User{
		{ID: 9, Balance: 10, CreatedAt: monthStart.AddDate(0, -1, 0)},
	}}
	snapRepo := newFakeBalanceSnapshotRepo()
	stmtRepo := &fakeStatementRepoTail{
		monthKey: rangeKey(monthStart, monthEnd),
		month:    &StatementNetChange{},
		tail:     &StatementNetChange{},
	}
	svc := NewBalanceSnapshotService(stmtRepo, snapRepo, userRepo, 20*time.Millisecond)

	svc.Start()
	deadline := time.Now().Add(2 * time.Second)
	for userRepo.listCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	svc.Stop()

	assert.GreaterOrEqual(t, userRepo.listCalls.Load(), int64(1), "expected background loop to invoke runOnce at least once")
}
