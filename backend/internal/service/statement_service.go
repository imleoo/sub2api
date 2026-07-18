package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/statement"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// StatementService 月度对账单拼装：六种行（期初/充值/提现/赠送/按日消耗/期末）+
// 快照优先、反推兜底的期初/期末口径 + 恒等式差额。资金口径见 statement_port.go。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
type StatementService struct {
	stmtRepo    StatementRepository
	snapRepo    BalanceSnapshotRepository
	snapshotSvc *BalanceSnapshotService
	userRepo    UserRepository
}

// NewStatementService 创建月度对账服务。
func NewStatementService(stmtRepo StatementRepository, snapRepo BalanceSnapshotRepository, snapshotSvc *BalanceSnapshotService, userRepo UserRepository) *StatementService {
	return &StatementService{
		stmtRepo:    stmtRepo,
		snapRepo:    snapRepo,
		snapshotSvc: snapshotSvc,
		userRepo:    userRepo,
	}
}

// GetStatement 生成 userID 在 month（YYYY-MM）的对账单；tzName 为空时用服务端时区。
func (s *StatementService) GetStatement(ctx context.Context, userID int64, month, tzName string) (*statement.Statement, error) {
	loc, tzName, err := resolveStatementLocation(tzName)
	if err != nil {
		return nil, err
	}
	monthStart, err := time.ParseInLocation("2006-01", month, loc)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_MONTH", "month must be in YYYY-MM format")
	}
	monthEnd := monthStart.AddDate(0, 1, 0)
	now := time.Now()
	if !monthStart.Before(now) {
		return nil, infraerrors.BadRequest("INVALID_MONTH", "month must not be in the future")
	}
	closed := !now.Before(monthEnd)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 并列拉取四类流水明细。
	deposits, err := s.stmtRepo.ListDeposits(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("list deposits: %w", err)
	}
	withdrawals, err := s.stmtRepo.ListWithdrawals(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	credits, err := s.stmtRepo.ListCredits(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("list credits: %w", err)
	}
	daily, err := s.stmtRepo.GetDailyUtilisation(ctx, userID, monthStart, monthEnd, tzName)
	if err != nil {
		return nil, fmt.Errorf("daily utilisation: %w", err)
	}

	// 期初/期末：已封账月优先读月结快照（精确值）；缺失或当月则实时反推。
	opening, closing, source, err := s.resolveBalances(ctx, user, month, monthStart, monthEnd, now, closed)
	if err != nil {
		return nil, err
	}

	st := &statement.Statement{
		Period:         month,
		Timezone:       tzName,
		Source:         source,
		Closed:         closed,
		OpeningBalance: opening,
		ClosingBalance: closing,
		UserEmail:      user.Email,
	}
	buildStatementRows(st, loc, monthStart, monthEnd, deposits, withdrawals, credits, daily)
	return st, nil
}

// ListAvailableMonths 返回可查询月份（注册月起至当月，倒序）。
func (s *StatementService) ListAvailableMonths(ctx context.Context, userID int64) ([]string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	loc := timezone.Location()
	cursor := time.Date(user.CreatedAt.In(loc).Year(), user.CreatedAt.In(loc).Month(), 1, 0, 0, 0, 0, loc)
	current := timezone.StartOfMonth(time.Now().In(loc))
	var months []string
	for !cursor.After(current) {
		months = append(months, cursor.Format("2006-01"))
		cursor = cursor.AddDate(0, 1, 0)
	}
	// 倒序：最近的月份在前。
	for i, j := 0, len(months)-1; i < j; i, j = i+1, j-1 {
		months[i], months[j] = months[j], months[i]
	}
	return months, nil
}

// RecomputeMonth 手动重算指定月份快照（转调月结服务，幂等）。
func (s *StatementService) RecomputeMonth(ctx context.Context, userID int64, month string) error {
	if s.snapshotSvc == nil {
		return infraerrors.InternalServer("SNAPSHOT_SERVICE_UNAVAILABLE", "snapshot service not available")
	}
	_, err := s.snapshotSvc.RecomputeForUserMonth(ctx, userID, month)
	return err
}

func (s *StatementService) resolveBalances(ctx context.Context, user *User, period string, monthStart, monthEnd time.Time, now time.Time, closed bool) (opening, closing float64, source string, err error) {
	if closed {
		snap, err := s.snapRepo.GetByUserPeriod(ctx, user.ID, period)
		if err != nil {
			return 0, 0, "", fmt.Errorf("load snapshot: %w", err)
		}
		if snap != nil {
			return snap.OpeningBalance, snap.ClosingBalance, "snapshot", nil
		}
	}

	// 反推：期末 = 当前余额 − 月末至今净变动（当月时 monthEnd>now，区间为空、净变动 0）。
	tailStart := monthEnd
	if tailStart.After(now) {
		tailStart = now
	}
	tail, err := s.stmtRepo.SumNetChange(ctx, user.ID, tailStart, now)
	if err != nil {
		return 0, 0, "", fmt.Errorf("sum tail net change: %w", err)
	}
	closing = user.Balance - tail.Net()

	net, err := s.stmtRepo.SumNetChange(ctx, user.ID, monthStart, monthEnd)
	if err != nil {
		return 0, 0, "", fmt.Errorf("sum month net change: %w", err)
	}
	opening = closing - net.Net()

	// 期初优先取上月快照期末（精确值），差异体现在恒等式 gap 上。
	prevPeriod := monthStart.AddDate(0, -1, 0).Format("2006-01")
	prev, err := s.snapRepo.GetByUserPeriod(ctx, user.ID, prevPeriod)
	if err != nil {
		return 0, 0, "", fmt.Errorf("load prev snapshot: %w", err)
	}
	if prev != nil {
		opening = prev.ClosingBalance
	}
	return opening, closing, "computed", nil
}

// buildStatementRows 拼装六种行 + 逐行滚动余额 + 合计与恒等式差额。
func buildStatementRows(st *statement.Statement, loc *time.Location, monthStart, monthEnd time.Time, deposits, withdrawals, credits []StatementCashflow, daily []StatementDailyUsage) {
	type flowRow struct {
		row statement.Row
		ord int // 同日内排序：deposit < withdraw < credit < utilisation
	}
	var flows []flowRow

	appendCashflows := func(items []StatementCashflow, nature string, ord int) {
		for _, cf := range items {
			flows = append(flows, flowRow{
				row: statement.Row{
					Date:   cf.Time.In(loc).Format("2006-01-02"),
					Nature: nature,
					Amount: cf.Amount,
					Note:   cf.Note,
				},
				ord: ord,
			})
		}
	}
	appendCashflows(deposits, statement.NatureDeposit, 0)
	appendCashflows(withdrawals, statement.NatureWithdraw, 1)
	appendCashflows(credits, statement.NatureCredit, 2)

	for _, d := range daily {
		if d.CostBefore == 0 && d.CostAfter == 0 && d.TotalTokens == 0 {
			continue
		}
		var unitPrice, discountRate float64
		if d.TotalTokens > 0 {
			unitPrice = d.CostBefore / float64(d.TotalTokens)
		}
		if d.CostBefore > 0 {
			discountRate = 1 - d.CostAfter/d.CostBefore
		}
		flows = append(flows, flowRow{
			row: statement.Row{
				Date:         d.Date,
				Nature:       statement.NatureUtilisation,
				Qty:          -d.TotalTokens,
				UnitPrice:    unitPrice,
				CostBefore:   d.CostBefore,
				DiscountRate: discountRate,
				CostAfter:    d.CostAfter,
			},
			ord: 3,
		})
	}

	sort.SliceStable(flows, func(i, j int) bool {
		if flows[i].row.Date != flows[j].row.Date {
			return flows[i].row.Date < flows[j].row.Date
		}
		return flows[i].ord < flows[j].ord
	})

	rows := make([]statement.Row, 0, len(flows)+2)
	rows = append(rows, statement.Row{
		Date:         monthStart.Format("2006-01-02"),
		Nature:       statement.NatureOpening,
		RunningTotal: st.OpeningBalance,
	})

	running := st.OpeningBalance
	for _, f := range flows {
		r := f.row
		switch r.Nature {
		case statement.NatureDeposit:
			running += r.Amount
			st.Totals.Deposit += r.Amount
		case statement.NatureCredit:
			running += r.Amount
			st.Totals.Credit += r.Amount
		case statement.NatureWithdraw:
			running -= r.Amount
			st.Totals.Withdraw += r.Amount
		case statement.NatureUtilisation:
			running -= r.CostAfter
			st.Totals.UtilisationBefore += r.CostBefore
			st.Totals.UtilisationAfter += r.CostAfter
		}
		r.RunningTotal = running
		rows = append(rows, r)
	}

	rows = append(rows, statement.Row{
		Date:         monthEnd.AddDate(0, 0, -1).Format("2006-01-02"),
		Nature:       statement.NatureClosing,
		RunningTotal: st.ClosingBalance,
	})
	st.Rows = rows
	st.Totals.IdentityGap = st.ClosingBalance - running
}

func resolveStatementLocation(tzName string) (*time.Location, string, error) {
	if tzName == "" {
		return timezone.Location(), timezone.Name(), nil
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, "", infraerrors.BadRequest("INVALID_TIMEZONE", "invalid timezone")
	}
	return loc, tzName, nil
}
