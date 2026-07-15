package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type teamActivityLogRepository struct {
	client *dbent.Client
}

func NewTeamActivityLogRepository(client *dbent.Client) service.TeamActivityLogRepository {
	return &teamActivityLogRepository{client: client}
}

func (r *teamActivityLogRepository) Append(ctx context.Context, ownerUserID, actorUserID int64, action, detail string) error {
	_, err := r.client.TeamActivityLog.Create().
		SetOwnerUserID(ownerUserID).
		SetActorUserID(actorUserID).
		SetAction(action).
		SetDetail(detail).
		Save(ctx)
	return err
}
