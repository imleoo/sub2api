package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// statementRepository 月度对账聚合查询（raw SQL，只读）。
//
// 资金口径详见 service/statement_port.go 顶部注释；此处 SQL 与该口径一一对应：
//   - 充值订单入账走「订单→兑换码→Redeem」链路，Credit 统计 redeem_codes 时
//     必须 NOT EXISTS(payment_orders.recharge_code) 排除，否则与 Deposit 双算。
//   - 时间区间统一半开 [start, end)。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
type statementRepository struct {
	sql sqlQueryer
}

// NewStatementRepository 创建月度对账聚合查询仓储。
func NewStatementRepository(sqlDB *sql.DB) service.StatementRepository {
	return &statementRepository{sql: sqlDB}
}

// depositsQuery 充值入账：已完成的余额充值订单（金额取 amount 面额，归月按
// completed_at 入账时刻）+ 企业划入（member 收 grant/auto_topup、owner 收 reclaim）。
const statementDepositsQuery = `
	SELECT * FROM (
		SELECT po.completed_at AS ts, po.amount AS amount,
		       'order ' || po.out_trade_no AS note, 'payment_order' AS source
		FROM payment_orders po
		WHERE po.user_id = $1 AND po.order_type = 'balance'
		  AND po.completed_at IS NOT NULL
		  AND po.completed_at >= $2 AND po.completed_at < $3
		UNION ALL
		SELECT t.created_at, t.amount,
		       CASE WHEN t.direction = 'reclaim' THEN 'team reclaim from member'
		            ELSE 'team ' || t.direction END,
		       'team_' || t.direction
		FROM team_fund_transfers t
		WHERE ((t.member_user_id = $1 AND t.direction IN ('grant', 'auto_topup'))
		    OR (t.owner_user_id = $1 AND t.direction = 'reclaim'))
		  AND t.created_at >= $2 AND t.created_at < $3
	) x ORDER BY ts, source
`

// withdrawalsQuery 资金流出：订单退款（按 refund_at）+ 企业划出
// （owner 划出 grant/auto_topup、member 被 reclaim）。金额返回正数量级。
const statementWithdrawalsQuery = `
	SELECT * FROM (
		SELECT po.refund_at AS ts, po.refund_amount AS amount,
		       'refund ' || po.out_trade_no AS note, 'refund' AS source
		FROM payment_orders po
		WHERE po.user_id = $1 AND po.order_type = 'balance'
		  AND po.refund_amount > 0 AND po.refund_at IS NOT NULL
		  AND po.refund_at >= $2 AND po.refund_at < $3
		UNION ALL
		SELECT t.created_at, t.amount,
		       CASE WHEN t.direction = 'reclaim' THEN 'team reclaim to owner'
		            ELSE 'team ' || t.direction END,
		       'team_' || t.direction
		FROM team_fund_transfers t
		WHERE ((t.owner_user_id = $1 AND t.direction IN ('grant', 'auto_topup'))
		    OR (t.member_user_id = $1 AND t.direction = 'reclaim'))
		  AND t.created_at >= $2 AND t.created_at < $3
	) x ORDER BY ts, source
`

// creditsQuery 赠送入账：优惠码 bonus + 余额兑换码（排除充值链路生成的码）。
const statementCreditsQuery = `
	SELECT * FROM (
		SELECT pcu.used_at AS ts, pcu.bonus_amount AS amount,
		       'promo bonus' AS note, 'promo' AS source
		FROM promo_code_usages pcu
		WHERE pcu.user_id = $1 AND pcu.used_at >= $2 AND pcu.used_at < $3
		UNION ALL
		SELECT rc.used_at, rc.value, 'redeem ' || rc.code, 'redeem'
		FROM redeem_codes rc
		WHERE rc.type = 'balance' AND rc.used_by = $1
		  AND rc.used_at IS NOT NULL
		  AND rc.used_at >= $2 AND rc.used_at < $3
		  AND NOT EXISTS (
			SELECT 1 FROM payment_orders po WHERE po.recharge_code = rc.code
		  )
	) x ORDER BY ts, source
`

func (r *statementRepository) listCashflows(ctx context.Context, query string, userID int64, start, end time.Time) (_ []service.StatementCashflow, err error) {
	rows, err := r.sql.QueryContext(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	var out []service.StatementCashflow
	for rows.Next() {
		var cf service.StatementCashflow
		if err = rows.Scan(&cf.Time, &cf.Amount, &cf.Note, &cf.Source); err != nil {
			return nil, err
		}
		out = append(out, cf)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *statementRepository) ListDeposits(ctx context.Context, userID int64, start, end time.Time) ([]service.StatementCashflow, error) {
	return r.listCashflows(ctx, statementDepositsQuery, userID, start, end)
}

func (r *statementRepository) ListWithdrawals(ctx context.Context, userID int64, start, end time.Time) ([]service.StatementCashflow, error) {
	return r.listCashflows(ctx, statementWithdrawalsQuery, userID, start, end)
}

func (r *statementRepository) ListCredits(ctx context.Context, userID int64, start, end time.Time) ([]service.StatementCashflow, error) {
	return r.listCashflows(ctx, statementCreditsQuery, userID, start, end)
}

func (r *statementRepository) GetDailyUtilisation(ctx context.Context, userID int64, start, end time.Time, tzName string) (_ []service.StatementDailyUsage, err error) {
	// created_at 为 timestamptz，AT TIME ZONE $4 转成请求时区本地时间后取自然日。
	query := `
		SELECT TO_CHAR(created_at AT TIME ZONE $4, 'YYYY-MM-DD') AS day,
		       COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0),
		       COALESCE(SUM(total_cost), 0),
		       COALESCE(SUM(actual_cost), 0)
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY day
		ORDER BY day
	`
	rows, err := r.sql.QueryContext(ctx, query, userID, start, end, tzName)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	var out []service.StatementDailyUsage
	for rows.Next() {
		var d service.StatementDailyUsage
		if err = rows.Scan(&d.Date, &d.TotalTokens, &d.CostBefore, &d.CostAfter); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *statementRepository) SumNetChange(ctx context.Context, userID int64, start, end time.Time) (*service.StatementNetChange, error) {
	// 单程聚合五表净变动；各子查询口径与上方逐笔查询严格一致。
	query := `
		SELECT
			(SELECT COALESCE(SUM(po.amount), 0) FROM payment_orders po
			 WHERE po.user_id = $1 AND po.order_type = 'balance'
			   AND po.completed_at IS NOT NULL
			   AND po.completed_at >= $2 AND po.completed_at < $3)
			+ (SELECT COALESCE(SUM(t.amount), 0) FROM team_fund_transfers t
			   WHERE ((t.member_user_id = $1 AND t.direction IN ('grant', 'auto_topup'))
			       OR (t.owner_user_id = $1 AND t.direction = 'reclaim'))
			     AND t.created_at >= $2 AND t.created_at < $3) AS deposit,
			(SELECT COALESCE(SUM(po.refund_amount), 0) FROM payment_orders po
			 WHERE po.user_id = $1 AND po.order_type = 'balance'
			   AND po.refund_amount > 0 AND po.refund_at IS NOT NULL
			   AND po.refund_at >= $2 AND po.refund_at < $3)
			+ (SELECT COALESCE(SUM(t.amount), 0) FROM team_fund_transfers t
			   WHERE ((t.owner_user_id = $1 AND t.direction IN ('grant', 'auto_topup'))
			       OR (t.member_user_id = $1 AND t.direction = 'reclaim'))
			     AND t.created_at >= $2 AND t.created_at < $3) AS withdraw,
			(SELECT COALESCE(SUM(pcu.bonus_amount), 0) FROM promo_code_usages pcu
			 WHERE pcu.user_id = $1 AND pcu.used_at >= $2 AND pcu.used_at < $3)
			+ (SELECT COALESCE(SUM(rc.value), 0) FROM redeem_codes rc
			   WHERE rc.type = 'balance' AND rc.used_by = $1
			     AND rc.used_at IS NOT NULL
			     AND rc.used_at >= $2 AND rc.used_at < $3
			     AND NOT EXISTS (
				SELECT 1 FROM payment_orders po WHERE po.recharge_code = rc.code
			     )) AS credit,
			(SELECT COALESCE(SUM(total_cost), 0) FROM usage_logs
			 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3) AS utilisation_gross,
			(SELECT COALESCE(SUM(actual_cost), 0) FROM usage_logs
			 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3) AS utilisation
	`
	var c service.StatementNetChange
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{userID, start, end},
		&c.Deposit,
		&c.Withdraw,
		&c.Credit,
		&c.UtilisationGross,
		&c.Utilisation,
	); err != nil {
		return nil, err
	}
	return &c, nil
}
