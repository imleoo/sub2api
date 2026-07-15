package service

import (
	"context"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）——一级部门管理。
// owner 或 admin 可管理部门；成员通过 team_members.department_id 归属部门。

// TeamDepartmentService 提供企业一级部门 CRUD。
type TeamDepartmentService struct {
	departmentRepo TeamDepartmentRepository
	teamMemberRepo TeamMemberRepository
	activityRepo   TeamActivityLogRepository
}

// NewTeamDepartmentService 创建 TeamDepartmentService 实例。
func NewTeamDepartmentService(
	departmentRepo TeamDepartmentRepository,
	teamMemberRepo TeamMemberRepository,
	activityRepo TeamActivityLogRepository,
) *TeamDepartmentService {
	return &TeamDepartmentService{
		departmentRepo: departmentRepo,
		teamMemberRepo: teamMemberRepo,
		activityRepo:   activityRepo,
	}
}

func validateDepartmentName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", infraerrors.BadRequest("INVALID_DEPARTMENT_NAME", "department name is required")
	}
	if len(name) > 100 {
		return "", infraerrors.BadRequest("INVALID_DEPARTMENT_NAME", "department name is too long")
	}
	return name, nil
}

// Create owner 或 admin 可调用：创建部门（同企业内名称唯一）。
func (s *TeamDepartmentService) Create(ctx context.Context, ownerUserID, actorUserID int64, name string, displayOrder int) (*TeamDepartment, error) {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return nil, err
	}
	name, err := validateDepartmentName(name)
	if err != nil {
		return nil, err
	}
	d, err := s.departmentRepo.Create(ctx, ownerUserID, name, displayOrder)
	if err != nil {
		return nil, err
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "department.create", name)
	return d, nil
}

// List owner 或任意 active 成员可调用（邀请表单等场景成员也需要看到部门列表）。
func (s *TeamDepartmentService) List(ctx context.Context, ownerUserID, actorUserID int64) ([]TeamDepartment, error) {
	if actorUserID != ownerUserID {
		m, err := s.teamMemberRepo.FindActive(ctx, ownerUserID, actorUserID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return nil, ErrInsufficientPerms
		}
	}
	return s.departmentRepo.List(ctx, ownerUserID)
}

// Update owner 或 admin 可调用：更新部门名称/排序。
func (s *TeamDepartmentService) Update(ctx context.Context, ownerUserID, actorUserID, departmentID int64, name string, displayOrder int) error {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return err
	}
	name, err := validateDepartmentName(name)
	if err != nil {
		return err
	}
	ok, err := s.departmentRepo.Update(ctx, departmentID, ownerUserID, name, displayOrder)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTeamDepartmentNotFound
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "department.update", fmt.Sprintf("#%d -> %s", departmentID, name))
	return nil
}

// Delete owner 或 admin 可调用：删除部门（成员的 department_id 由外键 SET NULL 自动置空）。
func (s *TeamDepartmentService) Delete(ctx context.Context, ownerUserID, actorUserID, departmentID int64) error {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return err
	}
	ok, err := s.departmentRepo.Delete(ctx, departmentID, ownerUserID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTeamDepartmentNotFound
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "department.delete", fmt.Sprintf("#%d", departmentID))
	return nil
}

func (s *TeamDepartmentService) appendActivity(ctx context.Context, ownerUserID, actorUserID int64, action, detail string) {
	if s.activityRepo == nil {
		return
	}
	_ = s.activityRepo.Append(ctx, ownerUserID, actorUserID, action, detail)
}
