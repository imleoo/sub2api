package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/teaminvitation"
	"github.com/Wei-Shaw/sub2api/ent/teammember"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type teamMemberRepository struct {
	client *dbent.Client
}

// NewTeamMemberRepository 创建 service.TeamMemberRepository 实现。
func NewTeamMemberRepository(client *dbent.Client) service.TeamMemberRepository {
	return &teamMemberRepository{client: client}
}

func toServiceTeamMember(m *dbent.TeamMember) *service.TeamMember {
	if m == nil {
		return nil
	}
	return &service.TeamMember{
		ID:                    m.ID,
		OwnerUserID:           m.OwnerUserID,
		MemberUserID:          m.MemberUserID,
		Role:                  m.Role,
		Status:                m.Status,
		DepartmentID:          m.DepartmentID,
		QuotaMode:             m.QuotaMode,
		AutoTopupThresholdUSD: m.AutoTopupThresholdUsd,
		AutoTopupTargetUSD:    m.AutoTopupTargetUsd,
		GrantedNetUSD:         m.GrantedNetUsd,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}

func (r *teamMemberRepository) Create(ctx context.Context, ownerUserID, memberUserID int64, role string) (*service.TeamMember, error) {
	m, err := r.client.TeamMember.Create().
		SetOwnerUserID(ownerUserID).
		SetMemberUserID(memberUserID).
		SetRole(role).
		SetStatus(domain.TeamMemberStatusActive).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return toServiceTeamMember(m), nil
}

func (r *teamMemberRepository) AcceptInvitation(ctx context.Context, p service.AcceptInvitationParams) (*service.TeamMember, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 对 owner 行加锁，序列化并发的多个邀请接受请求，避免"读到旧计数、都放行"的竞态。
	if _, err := tx.User.Query().Where(user.IDEQ(p.OwnerUserID)).ForUpdate().Only(ctx); err != nil {
		return nil, err
	}

	if p.MemberLimit > 0 {
		alreadyActive, err := tx.TeamMember.Query().
			Where(
				teammember.OwnerUserIDEQ(p.OwnerUserID),
				teammember.MemberUserIDEQ(p.MemberUserID),
				teammember.StatusEQ(domain.TeamMemberStatusActive),
			).
			Exist(ctx)
		if err != nil {
			return nil, err
		}
		if !alreadyActive {
			activeCount, err := tx.TeamMember.Query().
				Where(
					teammember.OwnerUserIDEQ(p.OwnerUserID),
					teammember.StatusEQ(domain.TeamMemberStatusActive),
				).
				Count(ctx)
			if err != nil {
				return nil, err
			}
			if activeCount+1 >= p.MemberLimit { // +1 为 owner 本人
				return nil, service.ErrTeamMemberLimitReached
			}
		}
	}

	create := tx.TeamMember.Create().
		SetOwnerUserID(p.OwnerUserID).
		SetMemberUserID(p.MemberUserID).
		SetRole(p.Role).
		SetStatus(domain.TeamMemberStatusActive).
		SetQuotaMode(p.QuotaMode)
	if p.DepartmentID != nil {
		create = create.SetDepartmentID(*p.DepartmentID)
	}
	id, err := create.
		OnConflictColumns(teammember.FieldOwnerUserID, teammember.FieldMemberUserID).
		UpdateNewValues().
		ID(ctx)
	if err != nil {
		return nil, err
	}

	n, err := tx.TeamInvitation.Update().
		Where(
			teaminvitation.IDEQ(p.InvitationID),
			teaminvitation.OwnerUserIDEQ(p.OwnerUserID),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
		).
		SetStatus(domain.TeamInvitationStatusAccepted).
		SetAcceptedByUserID(p.MemberUserID).
		SetAcceptedAt(p.AcceptedAt).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, service.ErrTeamInvitationNotFound
	}
	member, err := tx.TeamMember.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return toServiceTeamMember(member), nil
}

func (r *teamMemberRepository) FindActive(ctx context.Context, ownerUserID, memberUserID int64) (*service.TeamMember, error) {
	m, err := r.client.TeamMember.Query().
		Where(
			teammember.OwnerUserIDEQ(ownerUserID),
			teammember.MemberUserIDEQ(memberUserID),
			teammember.StatusEQ(domain.TeamMemberStatusActive),
		).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toServiceTeamMember(m), nil
}

func (r *teamMemberRepository) ListActiveByOwner(ctx context.Context, ownerUserID int64) ([]service.TeamMember, error) {
	rows, err := r.client.TeamMember.Query().
		Where(
			teammember.OwnerUserIDEQ(ownerUserID),
			teammember.StatusEQ(domain.TeamMemberStatusActive),
		).
		Order(teammember.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]service.TeamMember, 0, len(rows))
	for _, m := range rows {
		records = append(records, *toServiceTeamMember(m))
	}
	return records, nil
}

func (r *teamMemberRepository) ListActiveByMember(ctx context.Context, memberUserID int64) ([]service.TeamMember, error) {
	rows, err := r.client.TeamMember.Query().
		Where(
			teammember.MemberUserIDEQ(memberUserID),
			teammember.StatusEQ(domain.TeamMemberStatusActive),
		).
		Order(teammember.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]service.TeamMember, 0, len(rows))
	for _, m := range rows {
		records = append(records, *toServiceTeamMember(m))
	}
	return records, nil
}

func (r *teamMemberRepository) Remove(ctx context.Context, ownerUserID, memberUserID int64) (bool, error) {
	n, err := r.client.TeamMember.Update().
		Where(
			teammember.OwnerUserIDEQ(ownerUserID),
			teammember.MemberUserIDEQ(memberUserID),
			teammember.StatusEQ(domain.TeamMemberStatusActive),
		).
		SetStatus(domain.TeamMemberStatusRemoved).
		Save(ctx)
	return n > 0, err
}

func (r *teamMemberRepository) activeMemberUpdate(ownerUserID, memberUserID int64) *dbent.TeamMemberUpdate {
	return r.client.TeamMember.Update().
		Where(
			teammember.OwnerUserIDEQ(ownerUserID),
			teammember.MemberUserIDEQ(memberUserID),
			teammember.StatusEQ(domain.TeamMemberStatusActive),
		)
}

func (r *teamMemberRepository) SetDepartment(ctx context.Context, ownerUserID, memberUserID int64, departmentID *int64) (bool, error) {
	u := r.activeMemberUpdate(ownerUserID, memberUserID)
	if departmentID == nil {
		u = u.ClearDepartmentID()
	} else {
		u = u.SetDepartmentID(*departmentID)
	}
	n, err := u.Save(ctx)
	return n > 0, err
}

func (r *teamMemberRepository) SetQuotaSettings(ctx context.Context, ownerUserID, memberUserID int64, quotaMode string, threshold, target *float64) (bool, error) {
	u := r.activeMemberUpdate(ownerUserID, memberUserID).SetQuotaMode(quotaMode)
	if threshold == nil {
		u = u.ClearAutoTopupThresholdUsd()
	} else {
		u = u.SetAutoTopupThresholdUsd(*threshold)
	}
	if target == nil {
		u = u.ClearAutoTopupTargetUsd()
	} else {
		u = u.SetAutoTopupTargetUsd(*target)
	}
	n, err := u.Save(ctx)
	return n > 0, err
}

func (r *teamMemberRepository) SetRole(ctx context.Context, ownerUserID, memberUserID int64, role string) (bool, error) {
	n, err := r.activeMemberUpdate(ownerUserID, memberUserID).SetRole(role).Save(ctx)
	return n > 0, err
}

// ListTopupCandidates 扫描 shared 模式、余额低于补给阈值的 active 成员。
// 走 partial index idx_team_members_shared_active，JOIN users 读余额。
func (r *teamMemberRepository) ListTopupCandidates(ctx context.Context, limit int) ([]service.TeamTopupCandidate, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.client.QueryContext(ctx, `
		SELECT tm.owner_user_id, tm.member_user_id, u.balance, tm.auto_topup_target_usd
		FROM team_members tm
		JOIN users u ON u.id = tm.member_user_id
		WHERE tm.quota_mode = $1
		  AND tm.status = $2
		  AND tm.auto_topup_threshold_usd IS NOT NULL
		  AND tm.auto_topup_target_usd IS NOT NULL
		  AND u.balance < tm.auto_topup_threshold_usd
		ORDER BY tm.id
		LIMIT $3`,
		domain.TeamQuotaModeShared, domain.TeamMemberStatusActive, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []service.TeamTopupCandidate
	for rows.Next() {
		var c service.TeamTopupCandidate
		if err := rows.Scan(&c.OwnerUserID, &c.MemberUserID, &c.MemberBalance, &c.TargetUSD); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
