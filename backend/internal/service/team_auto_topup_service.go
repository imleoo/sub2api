package service

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"github.com/google/uuid"
)

// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）——共享额度自动补给。
//
// quota_mode=shared 的成员余额低于 auto_topup_threshold_usd 时，从企业余额
// 划转补到 auto_topup_target_usd。分钟级 SLA：滞后窗口内成员余额可能耗尽
// 触发 402/429，下一轮自动恢复。多实例用 leader lock 互斥（Redis 优先 +
// pg advisory 兜底），划转本身由 TeamFundRepository.Transfer 原子保证，
// 即使锁失效重复执行也只是多查一轮（余额已达标的成员不会再次命中扫描）。

const (
	teamAutoTopupInterval      = 60 * time.Second
	teamAutoTopupLeaderLockKey = "team_auto_topup"
	teamAutoTopupLeaderLockTTL = 5 * time.Minute
	teamAutoTopupBatchLimit    = 200
)

// TeamAutoTopupService 共享额度自动补给后台任务。
type TeamAutoTopupService struct {
	teamMemberRepo TeamMemberRepository
	fundRepo       TeamFundRepository
	activityRepo   TeamActivityLogRepository
	lockCache      LeaderLockCache
	db             *sql.DB
	instanceID     string

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

// NewTeamAutoTopupService 创建自动补给服务。
func NewTeamAutoTopupService(
	teamMemberRepo TeamMemberRepository,
	fundRepo TeamFundRepository,
	activityRepo TeamActivityLogRepository,
) *TeamAutoTopupService {
	return &TeamAutoTopupService{
		teamMemberRepo: teamMemberRepo,
		fundRepo:       fundRepo,
		activityRepo:   activityRepo,
		instanceID:     uuid.NewString(),
		stopCh:         make(chan struct{}),
	}
}

// SetLeaderLock 注入多实例互斥依赖（与 DashboardAggregationService 同模式）。
func (s *TeamAutoTopupService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// Start 启动后台补给循环。
func (s *TeamAutoTopupService) Start() {
	if s == nil || s.teamMemberRepo == nil || s.fundRepo == nil {
		return
	}
	s.startOnce.Do(func() {
		logger.LegacyPrintf("service.team_auto_topup", "[TeamAutoTopup] started interval=%s", teamAutoTopupInterval)
		go s.runLoop()
	})
}

// Stop 停止后台补给循环（wire cleanup 链调用）。
func (s *TeamAutoTopupService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
		logger.LegacyPrintf("service.team_auto_topup", "[TeamAutoTopup] stopped")
	})
}

func (s *TeamAutoTopupService) runLoop() {
	ticker := time.NewTicker(teamAutoTopupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runOnce()
		case <-s.stopCh:
			return
		}
	}
}

func (s *TeamAutoTopupService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, teamAutoTopupLeaderLockKey, s.instanceID, teamAutoTopupLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	candidates, err := s.teamMemberRepo.ListTopupCandidates(ctx, teamAutoTopupBatchLimit)
	if err != nil {
		logger.LegacyPrintf("service.team_auto_topup", "[TeamAutoTopup] scan failed: %v", err)
		return
	}
	for i := range candidates {
		c := candidates[i]
		amount := c.TargetUSD - c.MemberBalance
		if amount <= 0 {
			continue
		}
		if amount > teamTransferMaxAmount {
			amount = teamTransferMaxAmount
		}
		_, err := s.fundRepo.Transfer(ctx, c.OwnerUserID, c.MemberUserID, domain.TeamFundDirectionAutoTopup, amount, c.OwnerUserID, "auto topup to target")
		if err != nil {
			// 企业余额不足等业务失败：记审计后继续处理其余成员，不中断整轮。
			if s.activityRepo != nil {
				_ = s.activityRepo.Append(ctx, c.OwnerUserID, c.OwnerUserID, "fund.auto_topup_failed", err.Error())
			}
			logger.LegacyPrintf("service.team_auto_topup", "[TeamAutoTopup] transfer failed: owner=%d member=%d amount=%.8f err=%v", c.OwnerUserID, c.MemberUserID, amount, err)
			continue
		}
	}
}
