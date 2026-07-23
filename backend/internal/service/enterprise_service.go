package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）。
// 用户补充企业信息自助升级为企业客户，即时生效（无平台审批）。

// EnterpriseUpgradeInput 是自助升级企业客户的入参。
type EnterpriseUpgradeInput struct {
	CompanyName  string
	ContactName  string
	ContactPhone string
	Industry     string
}

// EnterpriseService 提供企业客户资料管理能力。
// ErrEnterpriseMemberCannotUpgrade：已是他人企业员工的账号不可再自助升级为企业客户
// （员工与企业主是互斥身份；如需自建企业请先退出所有企业）。
var ErrEnterpriseMemberCannotUpgrade = infraerrors.Forbidden("ENTERPRISE_MEMBER_CANNOT_UPGRADE", "you are already a member of an enterprise; leave it before upgrading")

type EnterpriseService struct {
	enterpriseRepo EnterpriseProfileRepository
	activityRepo   TeamActivityLogRepository
	teamMemberRepo TeamMemberRepository // 升级门：已是他人企业员工则拒绝升级（可为 nil，测试桩场景跳过）
}

// NewEnterpriseService 创建 EnterpriseService 实例。
func NewEnterpriseService(enterpriseRepo EnterpriseProfileRepository, activityRepo TeamActivityLogRepository, teamMemberRepo TeamMemberRepository) *EnterpriseService {
	return &EnterpriseService{enterpriseRepo: enterpriseRepo, activityRepo: activityRepo, teamMemberRepo: teamMemberRepo}
}

// Upgrade 自助升级为企业客户：写入企业资料，重复升级返回 ErrEnterpriseAlreadyUpgraded。
func (s *EnterpriseService) Upgrade(ctx context.Context, userID int64, in EnterpriseUpgradeInput) (*EnterpriseProfile, error) {
	companyName := strings.TrimSpace(in.CompanyName)
	if companyName == "" {
		return nil, infraerrors.BadRequest("INVALID_COMPANY_NAME", "company name is required")
	}
	if len(companyName) > 200 {
		return nil, infraerrors.BadRequest("INVALID_COMPANY_NAME", "company name is too long")
	}
	if s.teamMemberRepo != nil {
		memberships, err := s.teamMemberRepo.ListActiveByMember(ctx, userID)
		if err != nil {
			return nil, err
		}
		if len(memberships) > 0 {
			return nil, ErrEnterpriseMemberCannotUpgrade
		}
	}
	profile, err := s.enterpriseRepo.Create(ctx, userID,
		companyName,
		strings.TrimSpace(in.ContactName),
		strings.TrimSpace(in.ContactPhone),
		strings.TrimSpace(in.Industry),
	)
	if err != nil {
		return nil, err
	}
	if s.activityRepo != nil {
		_ = s.activityRepo.Append(ctx, userID, userID, "enterprise.upgrade", "upgraded to enterprise: "+companyName)
	}
	return profile, nil
}

// GetProfile 查询企业资料；非企业客户返回 (nil, nil)。
func (s *EnterpriseService) GetProfile(ctx context.Context, userID int64) (*EnterpriseProfile, error) {
	return s.enterpriseRepo.GetByUserID(ctx, userID)
}
