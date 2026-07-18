package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/balancesnapshot"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// balanceSnapshotRepository 余额月结快照仓储。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
type balanceSnapshotRepository struct {
	client *dbent.Client
}

// NewBalanceSnapshotRepository 创建余额月结快照仓储。
func NewBalanceSnapshotRepository(client *dbent.Client) service.BalanceSnapshotRepository {
	return &balanceSnapshotRepository{client: client}
}

func (r *balanceSnapshotRepository) Upsert(ctx context.Context, snap *service.BalanceSnapshotRecord) error {
	client := clientFromContext(ctx, r.client)
	return client.BalanceSnapshot.Create().
		SetUserID(snap.UserID).
		SetPeriod(snap.Period).
		SetOpeningBalance(snap.OpeningBalance).
		SetClosingBalance(snap.ClosingBalance).
		SetDepositTotal(snap.DepositTotal).
		SetWithdrawTotal(snap.WithdrawTotal).
		SetCreditTotal(snap.CreditTotal).
		SetUtilisationGrossTotal(snap.UtilisationGrossTotal).
		SetUtilisationTotal(snap.UtilisationTotal).
		SetComputedAt(time.Now()).
		OnConflictColumns(balancesnapshot.FieldUserID, balancesnapshot.FieldPeriod).
		UpdateNewValues().
		Exec(ctx)
}

func (r *balanceSnapshotRepository) GetByUserPeriod(ctx context.Context, userID int64, period string) (*service.BalanceSnapshotRecord, error) {
	row, err := r.client.BalanceSnapshot.Query().
		Where(
			balancesnapshot.UserIDEQ(userID),
			balancesnapshot.PeriodEQ(period),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return balanceSnapshotToService(row), nil
}

func (r *balanceSnapshotRepository) ListPeriodsByUser(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.client.BalanceSnapshot.Query().
		Where(balancesnapshot.UserIDEQ(userID)).
		Order(dbent.Asc(balancesnapshot.FieldPeriod)).
		Select(balancesnapshot.FieldPeriod).
		Strings(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func balanceSnapshotToService(row *dbent.BalanceSnapshot) *service.BalanceSnapshotRecord {
	if row == nil {
		return nil
	}
	return &service.BalanceSnapshotRecord{
		UserID:                row.UserID,
		Period:                row.Period,
		OpeningBalance:        row.OpeningBalance,
		ClosingBalance:        row.ClosingBalance,
		DepositTotal:          row.DepositTotal,
		WithdrawTotal:         row.WithdrawTotal,
		CreditTotal:           row.CreditTotal,
		UtilisationGrossTotal: row.UtilisationGrossTotal,
		UtilisationTotal:      row.UtilisationTotal,
		ComputedAt:            row.ComputedAt,
	}
}
