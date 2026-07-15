package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enterpriseprofile"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type enterpriseProfileRepository struct {
	client *dbent.Client
}

// NewEnterpriseProfileRepository 创建 service.EnterpriseProfileRepository 实现。
func NewEnterpriseProfileRepository(client *dbent.Client) service.EnterpriseProfileRepository {
	return &enterpriseProfileRepository{client: client}
}

func toServiceEnterpriseProfile(p *dbent.EnterpriseProfile) *service.EnterpriseProfile {
	if p == nil {
		return nil
	}
	return &service.EnterpriseProfile{
		UserID:       p.UserID,
		CompanyName:  p.CompanyName,
		ContactName:  p.ContactName,
		ContactPhone: p.ContactPhone,
		Industry:     p.Industry,
		CreatedAt:    p.CreatedAt,
	}
}

func (r *enterpriseProfileRepository) Create(ctx context.Context, userID int64, companyName, contactName, contactPhone, industry string) (*service.EnterpriseProfile, error) {
	p, err := r.client.EnterpriseProfile.Create().
		SetUserID(userID).
		SetCompanyName(companyName).
		SetContactName(contactName).
		SetContactPhone(contactPhone).
		SetIndustry(industry).
		Save(ctx)
	if dbent.IsConstraintError(err) {
		return nil, service.ErrEnterpriseAlreadyUpgraded
	}
	if err != nil {
		return nil, err
	}
	return toServiceEnterpriseProfile(p), nil
}

func (r *enterpriseProfileRepository) GetByUserID(ctx context.Context, userID int64) (*service.EnterpriseProfile, error) {
	p, err := r.client.EnterpriseProfile.Query().
		Where(enterpriseprofile.UserIDEQ(userID)).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toServiceEnterpriseProfile(p), nil
}
