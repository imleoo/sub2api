package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/teamdepartment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type teamDepartmentRepository struct {
	client *dbent.Client
}

// NewTeamDepartmentRepository 创建 service.TeamDepartmentRepository 实现。
func NewTeamDepartmentRepository(client *dbent.Client) service.TeamDepartmentRepository {
	return &teamDepartmentRepository{client: client}
}

func toServiceTeamDepartment(d *dbent.TeamDepartment) *service.TeamDepartment {
	if d == nil {
		return nil
	}
	return &service.TeamDepartment{
		ID:           d.ID,
		OwnerUserID:  d.OwnerUserID,
		Name:         d.Name,
		DisplayOrder: d.DisplayOrder,
		CreatedAt:    d.CreatedAt,
	}
}

func (r *teamDepartmentRepository) Create(ctx context.Context, ownerUserID int64, name string, displayOrder int) (*service.TeamDepartment, error) {
	d, err := r.client.TeamDepartment.Create().
		SetOwnerUserID(ownerUserID).
		SetName(name).
		SetDisplayOrder(displayOrder).
		Save(ctx)
	if dbent.IsConstraintError(err) {
		return nil, service.ErrTeamDepartmentNameTaken
	}
	if err != nil {
		return nil, err
	}
	return toServiceTeamDepartment(d), nil
}

func (r *teamDepartmentRepository) List(ctx context.Context, ownerUserID int64) ([]service.TeamDepartment, error) {
	rows, err := r.client.TeamDepartment.Query().
		Where(teamdepartment.OwnerUserIDEQ(ownerUserID)).
		Order(teamdepartment.ByDisplayOrder(), teamdepartment.ByID()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.TeamDepartment, 0, len(rows))
	for _, d := range rows {
		out = append(out, *toServiceTeamDepartment(d))
	}
	return out, nil
}

func (r *teamDepartmentRepository) Exists(ctx context.Context, id, ownerUserID int64) (bool, error) {
	return r.client.TeamDepartment.Query().
		Where(teamdepartment.IDEQ(id), teamdepartment.OwnerUserIDEQ(ownerUserID)).
		Exist(ctx)
}

func (r *teamDepartmentRepository) Update(ctx context.Context, id, ownerUserID int64, name string, displayOrder int) (bool, error) {
	n, err := r.client.TeamDepartment.Update().
		Where(teamdepartment.IDEQ(id), teamdepartment.OwnerUserIDEQ(ownerUserID)).
		SetName(name).
		SetDisplayOrder(displayOrder).
		Save(ctx)
	if dbent.IsConstraintError(err) {
		return false, service.ErrTeamDepartmentNameTaken
	}
	return n > 0, err
}

func (r *teamDepartmentRepository) Delete(ctx context.Context, id, ownerUserID int64) (bool, error) {
	n, err := r.client.TeamDepartment.Delete().
		Where(teamdepartment.IDEQ(id), teamdepartment.OwnerUserIDEQ(ownerUserID)).
		Exec(ctx)
	return n > 0, err
}
