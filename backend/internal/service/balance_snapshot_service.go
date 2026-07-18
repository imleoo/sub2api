package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	// balanceSnapshotLeaderLockKey 保证多实例部署时只有一个实例执行月结扫描。
	balanceSnapshotLeaderLockKey = "balance:snapshot:monthly:leader"
	// balanceSnapshotLeaderLockTTL 月结扫描需分页遍历全部用户并逐个聚合，
	// 给足单轮时间以避免锁提前过期导致双实例并跑。
	balanceSnapshotLeaderLockTTL = 10 * time.Minute
	// balanceSnapshotSettleDelay 月结在每月 1 日该时刻之后才执行，
	// 留出跨月写入（迟到账单/退款回调）落库的缓冲。
	balanceSnapshotSettleDelay = 30 * time.Minute
)

// BalanceSnapshotService 余额月结快照后台服务：每月 1 日 00:30（服务端时区）后
// 为上一自然月生成每用户余额快照（幂等 upsert），供月度对账单读取精确期初/期末。
//
// 计算口径（反推法，锚点为 users.balance 当前值）：
//
//	closing(M) = balance(now) − Net(M月末 → now)
//	opening(M) = 上月快照 closing；无上月快照时 = closing(M) − Net(M)
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
type BalanceSnapshotService struct {
	stmtRepo StatementRepository
	snapRepo BalanceSnapshotRepository
	userRepo UserRepository
	interval time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	mu                sync.Mutex
	lastSettledPeriod string // 本进程已完成月结的最近月份，避免每小时重复全量扫描
}

// NewBalanceSnapshotService 创建月结快照服务。
func NewBalanceSnapshotService(stmtRepo StatementRepository, snapRepo BalanceSnapshotRepository, userRepo UserRepository, interval time.Duration) *BalanceSnapshotService {
	return &BalanceSnapshotService{
		stmtRepo:   stmtRepo,
		snapRepo:   snapRepo,
		userRepo:   userRepo,
		interval:   interval,
		stopCh:     make(chan struct{}),
		instanceID: uuid.NewString(),
	}
}

// SetLeaderLock 注入多实例领导锁依赖；两者为 nil 时扫描不设防（单实例/测试行为）。
func (s *BalanceSnapshotService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *BalanceSnapshotService) Start() {
	if s == nil || s.stmtRepo == nil || s.snapRepo == nil || s.userRepo == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *BalanceSnapshotService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *BalanceSnapshotService) runOnce() {
	now := timezone.Now()
	period, due := settleDuePeriod(now)
	if !due {
		return
	}
	s.mu.Lock()
	settled := s.lastSettledPeriod >= period
	s.mu.Unlock()
	if settled {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), balanceSnapshotLeaderLockTTL)
	defer cancel()

	// 多实例护栏：仅领导实例执行全量用户扫描；upsert 幂等，锁丢失重跑无副作用。
	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, balanceSnapshotLeaderLockKey, s.instanceID, balanceSnapshotLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	if err := s.settlePeriod(ctx, period); err != nil {
		log.Printf("[BalanceSnapshot] Settle period %s failed: %v", period, err)
		return
	}
	s.mu.Lock()
	s.lastSettledPeriod = period
	s.mu.Unlock()
}

// settleDuePeriod 判断当前是否已过本月月结时点，返回待结算的上月 period。
func settleDuePeriod(now time.Time) (string, bool) {
	monthStart := timezone.StartOfMonth(now)
	if now.Before(monthStart.Add(balanceSnapshotSettleDelay)) {
		return "", false
	}
	return monthStart.AddDate(0, -1, 0).Format("2006-01"), true
}

// settlePeriod 分页遍历用户，为 period 月补算缺失快照（已存在的跳过）。
func (s *BalanceSnapshotService) settlePeriod(ctx context.Context, period string) error {
	monthStart, monthEnd, err := statementPeriodBounds(period)
	if err != nil {
		return err
	}
	var settledCount int
	for page := 1; ; page++ {
		users, pag, err := s.userRepo.List(ctx, pagination.PaginationParams{Page: page, PageSize: 200})
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		for i := range users {
			u := &users[i]
			// 月末之后才注册的用户无该月对账。
			if !u.CreatedAt.Before(monthEnd) {
				continue
			}
			existing, err := s.snapRepo.GetByUserPeriod(ctx, u.ID, period)
			if err != nil {
				return fmt.Errorf("check snapshot user=%d: %w", u.ID, err)
			}
			if existing != nil {
				continue
			}
			if _, err := s.recomputeLocked(ctx, u, period, monthStart, monthEnd); err != nil {
				// 单用户失败不中断整轮：记录后继续，下轮补算。
				log.Printf("[BalanceSnapshot] Recompute user=%d period=%s failed: %v", u.ID, period, err)
				continue
			}
			settledCount++
		}
		if pag == nil || page >= pag.Pages || len(users) == 0 {
			break
		}
	}
	if settledCount > 0 {
		log.Printf("[BalanceSnapshot] Settled %d snapshots for period %s", settledCount, period)
	}
	return nil
}

// RecomputeForUserMonth 为指定用户/月份重算快照并幂等落库（月结扫描与管理员手动
// 重算共用入口）。
func (s *BalanceSnapshotService) RecomputeForUserMonth(ctx context.Context, userID int64, period string) (*BalanceSnapshotRecord, error) {
	monthStart, monthEnd, err := statementPeriodBounds(period)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.recomputeLocked(ctx, user, period, monthStart, monthEnd)
}

func (s *BalanceSnapshotService) recomputeLocked(ctx context.Context, user *User, period string, monthStart, monthEnd time.Time) (*BalanceSnapshotRecord, error) {
	net, err := s.stmtRepo.SumNetChange(ctx, user.ID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("sum net change: %w", err)
	}
	// 期末 = 当前余额 − 月末至今的净变动（反推锚点：users.balance）。
	tail, err := s.stmtRepo.SumNetChange(ctx, user.ID, monthEnd, timezone.Now())
	if err != nil {
		return nil, fmt.Errorf("sum tail net change: %w", err)
	}
	closing := user.Balance - tail.Net()

	// 期初优先取上月快照的期末（精确值）；缺失时用本月净变动反推。
	opening := closing - net.Net()
	prevPeriod := monthStart.AddDate(0, -1, 0).Format("2006-01")
	prev, err := s.snapRepo.GetByUserPeriod(ctx, user.ID, prevPeriod)
	if err != nil {
		return nil, fmt.Errorf("load prev snapshot: %w", err)
	}
	if prev != nil {
		opening = prev.ClosingBalance
	}

	snap := &BalanceSnapshotRecord{
		UserID:                user.ID,
		Period:                period,
		OpeningBalance:        opening,
		ClosingBalance:        closing,
		DepositTotal:          net.Deposit,
		WithdrawTotal:         net.Withdraw,
		CreditTotal:           net.Credit,
		UtilisationGrossTotal: net.UtilisationGross,
		UtilisationTotal:      net.Utilisation,
	}
	if err := s.snapRepo.Upsert(ctx, snap); err != nil {
		return nil, fmt.Errorf("upsert snapshot: %w", err)
	}
	return snap, nil
}

// statementPeriodBounds 把 YYYY-MM 解析为服务端时区的自然月半开区间 [start, end)。
func statementPeriodBounds(period string) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01", period, timezone.Location())
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period %q: %w", period, err)
	}
	return start, start.AddDate(0, 1, 0), nil
}
