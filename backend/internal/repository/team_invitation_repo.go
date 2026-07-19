package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/teaminvitation"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type teamInvitationRepository struct {
	client *dbent.Client
}

// NewTeamInvitationRepository 创建 service.TeamInvitationRepository 实现。
func NewTeamInvitationRepository(client *dbent.Client) service.TeamInvitationRepository {
	return &teamInvitationRepository{client: client}
}

func toServiceTeamInvitation(inv *dbent.TeamInvitation) *service.TeamInvitation {
	if inv == nil {
		return nil
	}
	return &service.TeamInvitation{
		ID:               inv.ID,
		OwnerUserID:      inv.OwnerUserID,
		InvitedEmail:     inv.InvitedEmail,
		Token:            inv.Token,
		Status:           inv.Status,
		ExpiresAt:        inv.ExpiresAt,
		LastSentAt:       inv.LastSentAt,
		DepartmentID:     inv.DepartmentID,
		Role:             inv.Role,
		QuotaMode:        inv.QuotaMode,
		InitialGrantUSD:  inv.InitialGrantUsd,
		AcceptedByUserID: inv.AcceptedByUserID,
		AcceptedAt:       inv.AcceptedAt,
		CreatedAt:        inv.CreatedAt,
	}
}

func (r *teamInvitationRepository) Create(ctx context.Context, in service.TeamInvitationCreateInput) (*service.TeamInvitation, error) {
	create := r.client.TeamInvitation.Create().
		SetOwnerUserID(in.OwnerUserID).
		SetInvitedEmail(in.InvitedEmail).
		SetToken(in.Token).
		SetStatus(domain.TeamInvitationStatusPending).
		SetExpiresAt(in.ExpiresAt).
		SetRole(in.Role).
		SetQuotaMode(in.QuotaMode)
	if in.DepartmentID != nil {
		create = create.SetDepartmentID(*in.DepartmentID)
	}
	if in.InitialGrantUSD != nil {
		create = create.SetInitialGrantUsd(*in.InitialGrantUSD)
	}
	inv, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}
	return toServiceTeamInvitation(inv), nil
}

func (r *teamInvitationRepository) FindPendingByToken(ctx context.Context, token string) (*service.TeamInvitation, error) {
	inv, err := r.client.TeamInvitation.Query().
		Where(
			teaminvitation.TokenEQ(token),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
		).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toServiceTeamInvitation(inv), nil
}

func (r *teamInvitationRepository) FindPendingByID(ctx context.Context, id, ownerUserID int64) (*service.TeamInvitation, error) {
	inv, err := r.client.TeamInvitation.Query().
		Where(
			teaminvitation.IDEQ(id),
			teaminvitation.OwnerUserIDEQ(ownerUserID),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
		).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toServiceTeamInvitation(inv), nil
}

func (r *teamInvitationRepository) FindPendingByOwnerAndEmail(ctx context.Context, ownerUserID int64, invitedEmail string, now time.Time) (*service.TeamInvitation, error) {
	inv, err := r.client.TeamInvitation.Query().
		Where(
			teaminvitation.OwnerUserIDEQ(ownerUserID),
			teaminvitation.InvitedEmailEQ(invitedEmail),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
			teaminvitation.ExpiresAtGT(now),
		).
		Order(dbent.Desc(teaminvitation.FieldCreatedAt)).
		First(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toServiceTeamInvitation(inv), nil
}

func (r *teamInvitationRepository) ListPendingByOwner(ctx context.Context, ownerUserID int64) ([]service.TeamInvitation, error) {
	rows, err := r.client.TeamInvitation.Query().
		Where(
			teaminvitation.OwnerUserIDEQ(ownerUserID),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
			teaminvitation.ExpiresAtGT(time.Now()),
		).
		Order(teaminvitation.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]service.TeamInvitation, 0, len(rows))
	for _, inv := range rows {
		records = append(records, *toServiceTeamInvitation(inv))
	}
	return records, nil
}

func (r *teamInvitationRepository) ListPendingByInvitedEmail(ctx context.Context, invitedEmail string, now time.Time) ([]service.TeamInvitation, error) {
	rows, err := r.client.TeamInvitation.Query().
		Where(
			teaminvitation.InvitedEmailEQ(invitedEmail),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
			teaminvitation.ExpiresAtGT(now),
		).
		Order(teaminvitation.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]service.TeamInvitation, 0, len(rows))
	for _, inv := range rows {
		records = append(records, *toServiceTeamInvitation(inv))
	}
	return records, nil
}

func (r *teamInvitationRepository) MarkAccepted(ctx context.Context, id, acceptedByUserID int64, acceptedAt time.Time) error {
	_, err := r.client.TeamInvitation.UpdateOneID(id).
		SetStatus(domain.TeamInvitationStatusAccepted).
		SetAcceptedByUserID(acceptedByUserID).
		SetAcceptedAt(acceptedAt).
		Save(ctx)
	return err
}

func (r *teamInvitationRepository) TryMarkSent(ctx context.Context, id int64, now time.Time, minInterval time.Duration) (bool, error) {
	n, err := r.client.TeamInvitation.Update().
		Where(
			teaminvitation.IDEQ(id),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
			teaminvitation.LastSentAtLTE(now.Add(-minInterval)),
		).
		SetLastSentAt(now).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *teamInvitationRepository) Revoke(ctx context.Context, id, ownerUserID int64) (bool, error) {
	n, err := r.client.TeamInvitation.Update().
		Where(
			teaminvitation.IDEQ(id),
			teaminvitation.OwnerUserIDEQ(ownerUserID),
			teaminvitation.StatusEQ(domain.TeamInvitationStatusPending),
		).
		SetStatus(domain.TeamInvitationStatusRevoked).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
