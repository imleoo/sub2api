package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/teamfundtransfer"
	"github.com/Wei-Shaw/sub2api/ent/teammember"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type teamFundRepository struct {
	client *dbent.Client
}

// NewTeamFundRepository 创建 service.TeamFundRepository 实现。
func NewTeamFundRepository(client *dbent.Client) service.TeamFundRepository {
	return &teamFundRepository{client: client}
}

func toServiceTeamFundTransfer(t *dbent.TeamFundTransfer) *service.TeamFundTransfer {
	if t == nil {
		return nil
	}
	return &service.TeamFundTransfer{
		ID:             t.ID,
		OwnerUserID:    t.OwnerUserID,
		MemberUserID:   t.MemberUserID,
		Direction:      t.Direction,
		Amount:         t.Amount,
		OperatorUserID: t.OperatorUserID,
		Note:           t.Note,
		CreatedAt:      t.CreatedAt,
	}
}

// Transfer 企业↔员工原子划转。事务内：
//  1. 按 min/max(user_id) 固定顺序对两个 users 行 FOR UPDATE（防死锁）；
//  2. 对 active 成员行 FOR UPDATE（granted_net 读改写在锁内）；
//  3. 校验：grant/auto_topup 要求企业余额 ≥ amount；reclaim 要求
//     amount ≤ min(granted_net_usd, 员工当前余额)；
//  4. 双方余额更新 + granted_net 更新 + 台账 INSERT，一并提交。
func (r *teamFundRepository) Transfer(ctx context.Context, ownerUserID, memberUserID int64, direction string, amount float64, operatorUserID int64, note string) (*service.TeamFundTransfer, error) {
	if amount <= 0 {
		return nil, service.ErrTeamInvalidTransferAmount
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 固定锁序：id 小者先锁。
	firstID, secondID := ownerUserID, memberUserID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	lockedUsers := map[int64]*dbent.User{}
	for _, id := range []int64{firstID, secondID} {
		u, err := tx.User.Query().Where(user.IDEQ(id)).ForUpdate().Only(ctx)
		if err != nil {
			return nil, err
		}
		lockedUsers[id] = u
	}

	member, err := tx.TeamMember.Query().
		Where(
			teammember.OwnerUserIDEQ(ownerUserID),
			teammember.MemberUserIDEQ(memberUserID),
			teammember.StatusEQ(domain.TeamMemberStatusActive),
		).
		ForUpdate().
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrTeamMemberNotFound
	}
	if err != nil {
		return nil, err
	}

	ownerRow := lockedUsers[ownerUserID]
	memberRow := lockedUsers[memberUserID]

	var grantedNetDelta float64
	switch direction {
	case domain.TeamFundDirectionGrant, domain.TeamFundDirectionAutoTopup:
		if ownerRow.Balance < amount {
			return nil, service.ErrTeamInsufficientBalance
		}
		if _, err := tx.User.UpdateOneID(ownerUserID).AddBalance(-amount).Save(ctx); err != nil {
			return nil, err
		}
		if _, err := tx.User.UpdateOneID(memberUserID).AddBalance(amount).Save(ctx); err != nil {
			return nil, err
		}
		grantedNetDelta = amount
	case domain.TeamFundDirectionReclaim:
		limit := member.GrantedNetUsd
		if memberRow.Balance < limit {
			limit = memberRow.Balance
		}
		if amount > limit {
			return nil, service.ErrTeamReclaimExceedsLimit
		}
		if _, err := tx.User.UpdateOneID(memberUserID).AddBalance(-amount).Save(ctx); err != nil {
			return nil, err
		}
		if _, err := tx.User.UpdateOneID(ownerUserID).AddBalance(amount).Save(ctx); err != nil {
			return nil, err
		}
		grantedNetDelta = -amount
	default:
		return nil, service.ErrTeamInvalidTransferDirection
	}

	if _, err := tx.TeamMember.UpdateOneID(member.ID).AddGrantedNetUsd(grantedNetDelta).Save(ctx); err != nil {
		return nil, err
	}

	record, err := tx.TeamFundTransfer.Create().
		SetOwnerUserID(ownerUserID).
		SetMemberUserID(memberUserID).
		SetDirection(direction).
		SetAmount(amount).
		SetOperatorUserID(operatorUserID).
		SetNote(note).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return toServiceTeamFundTransfer(record), nil
}

func (r *teamFundRepository) ListByOwner(ctx context.Context, ownerUserID int64, offset, limit int) ([]service.TeamFundTransfer, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	q := r.client.TeamFundTransfer.Query().
		Where(teamfundtransfer.OwnerUserIDEQ(ownerUserID))
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.
		Order(dbent.Desc(teamfundtransfer.FieldCreatedAt), dbent.Desc(teamfundtransfer.FieldID)).
		Offset(offset).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]service.TeamFundTransfer, 0, len(rows))
	for _, t := range rows {
		out = append(out, *toServiceTeamFundTransfer(t))
	}
	return out, total, nil
}
