package service

import (
	"context"
	"time"
)

// 本文件定义月度对账（Vendor Report）的数据访问端口与领域类型。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
//
// 资金口径（与余额入账实现一一对应，防止双算/漏算）：
//   - Deposit  = payment_orders（order_type='balance' 且 completed_at 非空，金额取
//     amount 即实际入账面额）+ team_fund_transfers 中该用户作为受益方的划入
//     （member 收到 grant/auto_topup、owner 收到 reclaim）。
//   - Withdraw = payment_orders.refund_amount（按 refund_at）+ team_fund_transfers
//     中该用户作为支出方的划出（owner 划出 grant/auto_topup、member 被 reclaim）。
//   - Credit   = promo_code_usages.bonus_amount + redeem_codes（type='balance'，
//     used_by=该用户）。注意：充值订单入账走「订单→兑换码→Redeem」链路
//     （payment_fulfillment.go doBalance），这类兑换码必须用
//     NOT EXISTS(payment_orders.recharge_code = code) 排除，否则与 Deposit 双算。
//   - Utilisation = usage_logs（total_cost=倍率前折前、actual_cost=倍率后实际扣款）。

// StatementCashflow 单笔现金流条目（Deposit/Withdraw/Credit 三类行共用）。
type StatementCashflow struct {
	Time   time.Time
	Amount float64 // 正数量级；方向由行类型表达
	Note   string  // 订单号/兑换码/划转备注等可追溯标识
	Source string  // payment_order|refund|promo|redeem|team_grant|team_reclaim|team_auto_topup
}

// StatementDailyUsage 按天聚合的消费（对账单 Utilisation 行，一天一行）。
type StatementDailyUsage struct {
	Date        string // YYYY-MM-DD（按请求时区）
	TotalTokens int64
	CostBefore  float64 // Σ total_cost（倍率前）
	CostAfter   float64 // Σ actual_cost（倍率后实际扣款）
}

// StatementNetChange 区间内各类资金净变动汇总（反推期初/期末余额用）。
type StatementNetChange struct {
	Deposit          float64 // 充值 + 划入
	Withdraw         float64 // 退款 + 划出
	Credit           float64 // 赠送 + 兑换（已排除充值链路兑换码）
	UtilisationGross float64 // Σ total_cost
	Utilisation      float64 // Σ actual_cost
}

// Net 返回区间净变动（正=余额增加）。
func (c *StatementNetChange) Net() float64 {
	if c == nil {
		return 0
	}
	return c.Deposit - c.Withdraw + c.Credit - c.Utilisation
}

// StatementRepository 定义月度对账聚合查询端口。
// 时间区间统一为半开区间 [start, end)。
type StatementRepository interface {
	ListDeposits(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error)
	ListWithdrawals(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error)
	ListCredits(ctx context.Context, userID int64, start, end time.Time) ([]StatementCashflow, error)
	// GetDailyUtilisation 按 tzName 时区把 usage_logs 聚合成自然日行。
	GetDailyUtilisation(ctx context.Context, userID int64, start, end time.Time, tzName string) ([]StatementDailyUsage, error)
	// SumNetChange 单程聚合区间内全部资金净变动（反推法）。
	SumNetChange(ctx context.Context, userID int64, start, end time.Time) (*StatementNetChange, error)
}

// BalanceSnapshotRecord 月结快照（service 层结构，对应 ent BalanceSnapshot）。
type BalanceSnapshotRecord struct {
	UserID                int64
	Period                string // YYYY-MM
	OpeningBalance        float64
	ClosingBalance        float64
	DepositTotal          float64
	WithdrawTotal         float64 // 正数量级
	CreditTotal           float64
	UtilisationGrossTotal float64
	UtilisationTotal      float64 // 正数量级
	ComputedAt            time.Time
}

// BalanceSnapshotRepository 定义余额月结快照数据访问端口。
type BalanceSnapshotRepository interface {
	// Upsert 以 (user_id, period) 幂等写入。
	Upsert(ctx context.Context, snap *BalanceSnapshotRecord) error
	// GetByUserPeriod 未找到返回 (nil, nil)。
	GetByUserPeriod(ctx context.Context, userID int64, period string) (*BalanceSnapshotRecord, error)
	// ListPeriodsByUser 返回该用户已有快照的月份列表（升序）。
	ListPeriodsByUser(ctx context.Context, userID int64) ([]string, error)
}
