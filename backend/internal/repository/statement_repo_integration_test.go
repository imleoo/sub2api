//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatementRepo_CashflowsAndNetChange 覆盖月度对账五表聚合口径：
//   - Deposit：仅统计已完成(completed_at)的 balance 订单 + 企业划入；
//   - Credit：兑换码需排除充值链路（recharge_code 关联）生成的码，防双算；
//   - Withdraw：退款 + 企业划出；
//   - SumNetChange 与逐笔列表口径一致。
func TestStatementRepo_CashflowsAndNetChange(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := &statementRepository{sql: tx}

	user := mustCreateUser(t, client, &service.User{Email: "statement@test.com"})
	peer := mustCreateUser(t, client, &service.User{Email: "statement-peer@test.com"})

	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	mid := monthStart.AddDate(0, 0, 9)

	newOrder := func(amount float64, tradeNo, rechargeCode string) *dbent.PaymentOrderCreate {
		return client.PaymentOrder.Create().
			SetUserID(user.ID).
			SetUserEmail(user.Email).
			SetUserName("statement").
			SetAmount(amount).
			SetPayAmount(amount).
			SetRechargeCode(rechargeCode).
			SetOutTradeNo(tradeNo).
			SetPaymentType("epay").
			SetPaymentTradeNo("pt-" + tradeNo).
			SetOrderType("balance").
			SetExpiresAt(monthEnd).
			SetClientIP("127.0.0.1").
			SetSrcHost("test")
	}

	// 1) 已完成充值订单 50：计入 Deposit。
	_, err := newOrder(50, "trade-completed", "rcode-completed").
		SetStatus("COMPLETED").
		SetPaidAt(mid).
		SetCompletedAt(mid).
		Save(ctx)
	require.NoError(t, err)

	// 2) 未完成订单 999：不计入。
	_, err = newOrder(999, "trade-pending", "rcode-pending").
		SetStatus("PENDING").
		Save(ctx)
	require.NoError(t, err)

	// 3) 已退款订单：Deposit 计入 80（完成过），Withdraw 计入退款 30。
	_, err = newOrder(80, "trade-refunded", "rcode-refunded").
		SetStatus("PARTIALLY_REFUNDED").
		SetPaidAt(mid).
		SetCompletedAt(mid).
		SetRefundAmount(30).
		SetRefundAt(mid.AddDate(0, 0, 2)).
		Save(ctx)
	require.NoError(t, err)

	// 4) 充值链路兑换码（挂在订单 1 的 recharge_code 上）：Credit 必须排除。
	_, err = client.RedeemCode.Create().
		SetCode("rcode-completed").
		SetType("balance").
		SetValue(50).
		SetStatus("used").
		SetUsedBy(user.ID).
		SetUsedAt(mid).
		Save(ctx)
	require.NoError(t, err)

	// 5) 独立余额兑换码 5：计入 Credit。
	_, err = client.RedeemCode.Create().
		SetCode("rcode-standalone").
		SetType("balance").
		SetValue(5).
		SetStatus("used").
		SetUsedBy(user.ID).
		SetUsedAt(mid).
		Save(ctx)
	require.NoError(t, err)

	// 6) 优惠码赠送 3：计入 Credit。
	promo, err := client.PromoCode.Create().
		SetCode("PROMO-STMT").
		SetBonusAmount(3).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PromoCodeUsage.Create().
		SetPromoCodeID(promo.ID).
		SetUserID(user.ID).
		SetBonusAmount(3).
		SetUsedAt(mid).
		Save(ctx)
	require.NoError(t, err)

	// 7) 企业划转：user 作为成员收到 grant 12（Deposit）；被 reclaim 4（Withdraw）。
	_, err = client.TeamFundTransfer.Create().
		SetOwnerUserID(peer.ID).
		SetMemberUserID(user.ID).
		SetDirection("grant").
		SetAmount(12).
		SetOperatorUserID(peer.ID).
		SetCreatedAt(mid).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.TeamFundTransfer.Create().
		SetOwnerUserID(peer.ID).
		SetMemberUserID(user.ID).
		SetDirection("reclaim").
		SetAmount(4).
		SetOperatorUserID(peer.ID).
		SetCreatedAt(mid).
		Save(ctx)
	require.NoError(t, err)

	// 8) 区间外流水（上月）：一律不计入。
	_, err = newOrder(777, "trade-lastmonth", "rcode-lastmonth").
		SetStatus("COMPLETED").
		SetPaidAt(monthStart.AddDate(0, 0, -5)).
		SetCompletedAt(monthStart.AddDate(0, 0, -5)).
		Save(ctx)
	require.NoError(t, err)

	deposits, err := repo.ListDeposits(ctx, user.ID, monthStart, monthEnd)
	require.NoError(t, err)
	assert.Len(t, deposits, 3) // 50 + 80 + 12
	var depositSum float64
	for _, d := range deposits {
		depositSum += d.Amount
	}
	assert.InDelta(t, 142.0, depositSum, 1e-9)

	withdrawals, err := repo.ListWithdrawals(ctx, user.ID, monthStart, monthEnd)
	require.NoError(t, err)
	assert.Len(t, withdrawals, 2) // 退款 30 + reclaim 4
	var withdrawSum float64
	for _, w := range withdrawals {
		withdrawSum += w.Amount
	}
	assert.InDelta(t, 34.0, withdrawSum, 1e-9)

	credits, err := repo.ListCredits(ctx, user.ID, monthStart, monthEnd)
	require.NoError(t, err)
	assert.Len(t, credits, 2) // promo 3 + standalone 5（rcode-completed 被排除）
	var creditSum float64
	for _, c := range credits {
		creditSum += c.Amount
	}
	assert.InDelta(t, 8.0, creditSum, 1e-9)

	net, err := repo.SumNetChange(ctx, user.ID, monthStart, monthEnd)
	require.NoError(t, err)
	assert.InDelta(t, 142.0, net.Deposit, 1e-9)
	assert.InDelta(t, 34.0, net.Withdraw, 1e-9)
	assert.InDelta(t, 8.0, net.Credit, 1e-9)

	// peer（企业主）视角：grant 12 是划出、reclaim 4 是划入。
	peerNet, err := repo.SumNetChange(ctx, peer.ID, monthStart, monthEnd)
	require.NoError(t, err)
	assert.InDelta(t, 4.0, peerNet.Deposit, 1e-9)
	assert.InDelta(t, 12.0, peerNet.Withdraw, 1e-9)
}

// TestStatementRepo_DailyUtilisationTimezone 验证按请求时区归日：
// UTC 2026-06-01T20:00 在 Asia/Shanghai (+8) 属于 6 月 2 日。
func TestStatementRepo_DailyUtilisationTimezone(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := &statementRepository{sql: tx}

	user := mustCreateUser(t, client, &service.User{Email: "statement-tz@test.com"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-stmt-tz", Name: "k"})
	account := mustCreateAccount(t, client, &service.Account{Name: "acc-stmt-tz"})
	usageRepo := newUsageLogRepositoryWithSQL(client, tx)

	create := func(at time.Time, totalCost, actualCost float64, tokens int) {
		_, err := usageRepo.Create(ctx, &service.UsageLog{
			UserID:       user.ID,
			APIKeyID:     apiKey.ID,
			AccountID:    account.ID,
			Model:        "claude-test",
			InputTokens:  tokens,
			OutputTokens: 0,
			TotalCost:    totalCost,
			ActualCost:   actualCost,
			CreatedAt:    at,
		})
		require.NoError(t, err)
	}

	// 上海时间 6-01 18:00（UTC 10:00）→ 归 6-01。
	create(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), 1.0, 0.5, 100)
	// 上海时间 6-02 04:00（UTC 6-01 20:00）→ 归 6-02。
	create(time.Date(2026, 6, 1, 20, 0, 0, 0, time.UTC), 2.0, 1.0, 200)

	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Add(-8 * time.Hour)
	monthEnd := monthStart.AddDate(0, 1, 0)
	days, err := repo.GetDailyUtilisation(ctx, user.ID, monthStart, monthEnd, "Asia/Shanghai")
	require.NoError(t, err)
	require.Len(t, days, 2)
	assert.Equal(t, "2026-06-01", days[0].Date)
	assert.InDelta(t, 1.0, days[0].CostBefore, 1e-9)
	assert.InDelta(t, 0.5, days[0].CostAfter, 1e-9)
	assert.Equal(t, int64(100), days[0].TotalTokens)
	assert.Equal(t, "2026-06-02", days[1].Date)
	assert.InDelta(t, 2.0, days[1].CostBefore, 1e-9)

	// UTC 归日则两条同属 6-01。
	daysUTC, err := repo.GetDailyUtilisation(ctx, user.ID, monthStart, monthEnd, "UTC")
	require.NoError(t, err)
	require.Len(t, daysUTC, 1)
	assert.Equal(t, "2026-06-01", daysUTC[0].Date)
	assert.InDelta(t, 3.0, daysUTC[0].CostBefore, 1e-9)
}
