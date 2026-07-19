//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// orphanAccountRepoStub 模拟账号已被软删除：GetByID 恒返回 ErrAccountNotFound。
type orphanAccountRepoStub struct {
	accountRepoStub
}

func (s *orphanAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	return nil, ErrAccountNotFound
}

// orphanPlanRepoStub 记录 Delete 调用，其余方法不应被触达。
type orphanPlanRepoStub struct {
	deletedIDs []int64
}

func (s *orphanPlanRepoStub) Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	panic("unexpected Create call")
}

func (s *orphanPlanRepoStub) GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error) {
	panic("unexpected GetByID call")
}

func (s *orphanPlanRepoStub) ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error) {
	panic("unexpected ListByAccountID call")
}

func (s *orphanPlanRepoStub) ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error) {
	panic("unexpected ListDue call")
}

func (s *orphanPlanRepoStub) Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	panic("unexpected Update call")
}

func (s *orphanPlanRepoStub) Delete(ctx context.Context, id int64) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return nil
}

func (s *orphanPlanRepoStub) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	panic("unexpected UpdateAfterRun call")
}

// TestRunTestBackground_AccountNotFound_ReturnsSentinel 回归守护。
//
// 账号软删除后 scheduled_test_plans 的 ON DELETE CASCADE 不会触发，孤儿计划每分钟
// 走进 SSE 测试路径刷一条 "Account test error: Account not found" ERROR+堆栈
// （线上 2026-07-19 实际发生）。RunTestBackground 必须先做存在性检查并把
// ErrAccountNotFound 抛给调用方，而非吞进 result.ErrorMessage。
func TestRunTestBackground_AccountNotFound_ReturnsSentinel(t *testing.T) {
	svc := NewAccountTestService(&orphanAccountRepoStub{}, nil, nil, nil, &config.Config{}, nil)

	result, err := svc.RunTestBackground(context.Background(), 12345, "")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccountNotFound))
	require.Nil(t, result)
}

// TestScheduledTestRunner_RunOnePlan_PrunesOrphanPlan 账号不存在时 runner 应删除孤儿计划自愈。
func TestScheduledTestRunner_RunOnePlan_PrunesOrphanPlan(t *testing.T) {
	planRepo := &orphanPlanRepoStub{}
	accountTestSvc := NewAccountTestService(&orphanAccountRepoStub{}, nil, nil, nil, &config.Config{}, nil)
	runner := NewScheduledTestRunnerService(planRepo, nil, accountTestSvc, nil, &config.Config{})

	runner.runOnePlan(context.Background(), &ScheduledTestPlan{ID: 7, AccountID: 12345})

	require.Equal(t, []int64{7}, planRepo.deletedIDs, "孤儿计划必须被删除，否则每分钟报错刷屏")
}
