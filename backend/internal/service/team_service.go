package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）。
//
// 企业主体沿用创建企业的那个 User（owner_user_id），员工（member_user_id）是独立
// 登录账号、自管自己的 API Key、消费扣自己的余额。企业通过真实余额划转分配额度：
// allocated = 管理员手动划转；shared = 后台自动补给。角色：owner（隐式超管）/
// admin（部门、邀请、划转回收、报表，可多个）/ member。仅 owner 可升降 admin。

var (
	ErrTeamInvitationNotFound       = infraerrors.NotFound("TEAM_INVITATION_NOT_FOUND", "invitation not found or already handled")
	ErrTeamInvitationExpired        = infraerrors.BadRequest("TEAM_INVITATION_EXPIRED", "invitation has expired")
	ErrTeamInvitationEmailMismatch  = infraerrors.Forbidden("TEAM_INVITATION_EMAIL_MISMATCH", "invitation email does not match your account email")
	ErrTeamCannotInviteSelf         = infraerrors.BadRequest("TEAM_CANNOT_INVITE_SELF", "cannot invite yourself")
	ErrTeamCannotRemoveOwner        = infraerrors.BadRequest("TEAM_CANNOT_REMOVE_OWNER", "cannot remove the team owner")
	ErrTeamAlreadyMember            = infraerrors.Conflict("TEAM_ALREADY_MEMBER", "this email is already a member of your team")
	ErrTeamMemberNotFound           = infraerrors.NotFound("TEAM_MEMBER_NOT_FOUND", "team member not found")
	ErrTeamInvitationResendTooSoon  = infraerrors.BadRequest("TEAM_INVITATION_RESEND_TOO_SOON", "please wait before resending this invitation")
	ErrTeamMemberLimitReached       = infraerrors.BadRequest("TEAM_MEMBER_LIMIT_REACHED", "team member limit reached")
	ErrEnterpriseRequired           = infraerrors.Forbidden("ENTERPRISE_REQUIRED", "upgrade to an enterprise account first")
	ErrEnterpriseAlreadyUpgraded    = infraerrors.Conflict("ENTERPRISE_ALREADY_UPGRADED", "this account is already an enterprise account")
	ErrTeamDepartmentNotFound       = infraerrors.NotFound("TEAM_DEPARTMENT_NOT_FOUND", "department not found")
	ErrTeamDepartmentNameTaken      = infraerrors.Conflict("TEAM_DEPARTMENT_NAME_TAKEN", "a department with this name already exists")
	ErrTeamInvalidRole              = infraerrors.BadRequest("TEAM_INVALID_ROLE", "invalid team role")
	ErrTeamInvalidQuotaMode         = infraerrors.BadRequest("TEAM_INVALID_QUOTA_MODE", "invalid quota mode")
	ErrTeamOwnerOnly                = infraerrors.Forbidden("TEAM_OWNER_ONLY", "only the enterprise owner can perform this action")
	ErrTeamInsufficientBalance      = infraerrors.BadRequest("TEAM_INSUFFICIENT_ENTERPRISE_BALANCE", "enterprise balance is insufficient for this transfer")
	ErrTeamReclaimExceedsLimit      = infraerrors.BadRequest("TEAM_RECLAIM_EXCEEDS_LIMIT", "reclaim amount exceeds min(granted net, member balance)")
	ErrTeamInvalidTransferAmount    = infraerrors.BadRequest("TEAM_INVALID_TRANSFER_AMOUNT", "transfer amount must be positive")
	ErrTeamInvalidTransferDirection = infraerrors.BadRequest("TEAM_INVALID_TRANSFER_DIRECTION", "invalid transfer direction")
)

const teamInvitationTTL = 7 * 24 * time.Hour
const teamInvitationResendInterval = 60 * time.Second

// teamMemberLimit 是企业成员数量上限（含 owner 本人）。
const teamMemberLimit = 50

// TeamMemberView 是企业成员列表的展示结构体，owner 一行为隐式合成（不落库）。
type TeamMemberView struct {
	UserID        int64
	Email         string
	Role          string
	DepartmentID  *int64
	QuotaMode     string
	GrantedNetUSD float64
	JoinedAt      *time.Time
}

// TeamSummary 是"我的企业列表"的展示结构体，包含个人账户一行 + 我作为成员加入的企业。
type TeamSummary struct {
	OwnerUserID   int64
	OwnerEmail    string
	IsPersonal    bool
	Role          string
	Balance       float64
	FrozenBalance float64
}

// TeamInviteInput 是发起邀请的业务入参。
type TeamInviteInput struct {
	Email           string
	DepartmentID    *int64
	Role            string
	QuotaMode       string
	InitialGrantUSD *float64
}

// TeamService 提供企业组织的邀请/成员/角色管理能力。
type TeamService struct {
	teamMemberRepo     TeamMemberRepository
	teamInvitationRepo TeamInvitationRepository
	teamActivityRepo   TeamActivityLogRepository
	userRepo           UserRepository
	enterpriseRepo     EnterpriseProfileRepository
	departmentRepo     TeamDepartmentRepository
	fundRepo           TeamFundRepository
	emailService       *EmailService
	settingService     *SettingService
}

// NewTeamService 创建 TeamService 实例。
func NewTeamService(
	teamMemberRepo TeamMemberRepository,
	teamInvitationRepo TeamInvitationRepository,
	teamActivityRepo TeamActivityLogRepository,
	userRepo UserRepository,
	enterpriseRepo EnterpriseProfileRepository,
	departmentRepo TeamDepartmentRepository,
	fundRepo TeamFundRepository,
	emailService *EmailService,
	settingService *SettingService,
) *TeamService {
	return &TeamService{
		teamMemberRepo:     teamMemberRepo,
		teamInvitationRepo: teamInvitationRepo,
		teamActivityRepo:   teamActivityRepo,
		userRepo:           userRepo,
		enterpriseRepo:     enterpriseRepo,
		departmentRepo:     departmentRepo,
		fundRepo:           fundRepo,
		emailService:       emailService,
		settingService:     settingService,
	}
}

func generateTeamInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func normalizeTeamEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// authorizeTeamManager 校验 actor 对该企业具备管理权限（owner 本人或 active admin 成员），
// 返回 actor 的角色（"owner"/"admin"）。供 team/fund/department 各服务共用。
func authorizeTeamManager(ctx context.Context, memberRepo TeamMemberRepository, ownerUserID, actorUserID int64) (string, error) {
	if actorUserID == ownerUserID {
		return "owner", nil
	}
	m, err := memberRepo.FindActive(ctx, ownerUserID, actorUserID)
	if err != nil {
		return "", err
	}
	if m == nil || m.Role != domain.TeamMemberRoleAdmin {
		return "", ErrInsufficientPerms
	}
	return domain.TeamMemberRoleAdmin, nil
}

// requireEnterprise 校验 owner 已升级为企业客户。
func (s *TeamService) requireEnterprise(ctx context.Context, ownerUserID int64) error {
	if s.enterpriseRepo == nil {
		return nil // 测试桩场景
	}
	p, err := s.enterpriseRepo.GetByUserID(ctx, ownerUserID)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrEnterpriseRequired
	}
	return nil
}

func validateTeamRole(role string) error {
	if role != domain.TeamMemberRoleAdmin && role != domain.TeamMemberRoleMember {
		return ErrTeamInvalidRole
	}
	return nil
}

func validateQuotaMode(mode string) error {
	if mode != domain.TeamQuotaModeAllocated && mode != domain.TeamQuotaModeShared {
		return ErrTeamInvalidQuotaMode
	}
	return nil
}

// InviteMember owner 或 admin 可调用：邀请一个员工加入企业（支持邀请未注册邮箱）。
// 可指定部门、角色（仅 owner 可直接邀请为 admin）、额度模式与初始划转额度。
func (s *TeamService) InviteMember(ctx context.Context, ownerUserID, actorUserID int64, in TeamInviteInput, frontendBaseURL string) (*TeamInvitation, error) {
	actorRole, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEnterprise(ctx, ownerUserID); err != nil {
		return nil, err
	}

	email := normalizeTeamEmail(in.Email)
	if email == "" {
		return nil, infraerrors.BadRequest("INVALID_EMAIL", "email is required")
	}
	if in.Role == "" {
		in.Role = domain.TeamMemberRoleMember
	}
	if err := validateTeamRole(in.Role); err != nil {
		return nil, err
	}
	// 仅 owner 可直接邀请为 admin（admin 不能自我复制权限）。
	if in.Role == domain.TeamMemberRoleAdmin && actorRole != "owner" {
		return nil, ErrTeamOwnerOnly
	}
	if in.QuotaMode == "" {
		in.QuotaMode = domain.TeamQuotaModeAllocated
	}
	if err := validateQuotaMode(in.QuotaMode); err != nil {
		return nil, err
	}
	if in.InitialGrantUSD != nil && *in.InitialGrantUSD <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INITIAL_GRANT", "initial grant must be positive")
	}
	if in.DepartmentID != nil && s.departmentRepo != nil {
		ok, err := s.departmentRepo.Exists(ctx, *in.DepartmentID, ownerUserID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrTeamDepartmentNotFound
		}
	}

	owner, err := s.userRepo.GetByID(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(owner.Email, email) {
		return nil, ErrTeamCannotInviteSelf
	}

	if invitee, err := s.userRepo.GetByEmail(ctx, email); err == nil && invitee != nil {
		existing, err := s.teamMemberRepo.FindActive(ctx, ownerUserID, invitee.ID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, ErrTeamAlreadyMember
		}
	}
	if existing, err := s.teamInvitationRepo.FindPendingByOwnerAndEmail(ctx, ownerUserID, email, time.Now()); err != nil {
		return nil, err
	} else if existing != nil {
		now := time.Now()
		claimed, err := s.teamInvitationRepo.TryMarkSent(ctx, existing.ID, now, teamInvitationResendInterval)
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, ErrTeamInvitationResendTooSoon
		}
		if err := s.sendInviteEmail(ctx, owner.Email, email, existing.Token, frontendBaseURL); err != nil {
			return nil, fmt.Errorf("resend existing team invitation email: %w", err)
		}
		existing.LastSentAt = now
		s.appendActivity(ctx, ownerUserID, actorUserID, "member.invite_resend", fmt.Sprintf("invitation #%d", existing.ID))
		return existing, nil
	}

	if count, err := s.activeMemberCount(ctx, ownerUserID); err != nil {
		return nil, err
	} else if count >= teamMemberLimit {
		return nil, ErrTeamMemberLimitReached
	}

	token, err := generateTeamInviteToken()
	if err != nil {
		return nil, err
	}
	invitation, err := s.teamInvitationRepo.Create(ctx, TeamInvitationCreateInput{
		OwnerUserID:     ownerUserID,
		InvitedEmail:    email,
		Token:           token,
		ExpiresAt:       time.Now().Add(teamInvitationTTL),
		DepartmentID:    in.DepartmentID,
		Role:            in.Role,
		QuotaMode:       in.QuotaMode,
		InitialGrantUSD: in.InitialGrantUSD,
	})
	if err != nil {
		return nil, err
	}

	if err := s.sendInviteEmail(ctx, owner.Email, email, token, frontendBaseURL); err != nil {
		return nil, fmt.Errorf("send team invitation email: %w", err)
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "member.invite", fmt.Sprintf("invited %s role=%s", email, in.Role))

	return invitation, nil
}

func (s *TeamService) sendInviteEmail(ctx context.Context, ownerEmail, invitedEmail, token, frontendBaseURL string) error {
	if s.emailService == nil {
		return nil
	}
	siteName := "TokenPanel"
	if s.settingService != nil {
		siteName = s.settingService.GetSiteName(ctx)
	}
	acceptURL := fmt.Sprintf("%s/team/invite/accept?token=%s", strings.TrimSuffix(frontendBaseURL, "/"), token)
	subject := fmt.Sprintf("%s 企业邀请", siteName)
	body := buildTeamInviteEmailBody(siteName, ownerEmail, acceptURL)
	return s.emailService.SendEmail(ctx, invitedEmail, subject, body)
}

// ResendInvitation owner 或 admin 可调用：重新发送仍处于 pending 状态的邀请。
func (s *TeamService) ResendInvitation(ctx context.Context, ownerUserID, actorUserID, invitationID int64, frontendBaseURL string) error {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return err
	}
	invitation, err := s.teamInvitationRepo.FindPendingByID(ctx, invitationID, ownerUserID)
	if err != nil {
		return err
	}
	if invitation == nil || time.Now().After(invitation.ExpiresAt) {
		return ErrTeamInvitationNotFound
	}
	claimed, err := s.teamInvitationRepo.TryMarkSent(ctx, invitationID, time.Now(), teamInvitationResendInterval)
	if err != nil {
		return err
	}
	if !claimed {
		return ErrTeamInvitationResendTooSoon
	}
	owner, err := s.userRepo.GetByID(ctx, ownerUserID)
	if err != nil {
		return err
	}
	if err := s.sendInviteEmail(ctx, owner.Email, invitation.InvitedEmail, invitation.Token, frontendBaseURL); err != nil {
		return fmt.Errorf("resend team invitation email: %w", err)
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "member.invite_resend", fmt.Sprintf("invitation #%d", invitationID))
	return nil
}

// activeMemberCount 返回企业当前成员数（含 owner 本人 + 所有 active 成员）。
func (s *TeamService) activeMemberCount(ctx context.Context, ownerUserID int64) (int, error) {
	members, err := s.teamMemberRepo.ListActiveByOwner(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}
	return len(members) + 1, nil
}

func buildTeamInviteEmailBody(siteName, ownerEmail, acceptURL string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif; background-color:#f5f5f5; margin:0; padding:20px;">
  <div style="max-width:600px;margin:0 auto;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,0.1);">
    <div style="background:linear-gradient(135deg,#667eea 0%%,#764ba2 100%%);color:#fff;padding:30px;text-align:center;">
      <h1 style="margin:0;font-size:24px;">%s</h1>
    </div>
    <div style="padding:40px 30px;text-align:center;">
      <p style="font-size:18px;color:#333;">企业邀请</p>
      <p style="color:#666;">%s 邀请你加入其企业，加入后你将使用自己的账号和 API Key，企业会为你分配用量额度。</p>
      <a href="%s" style="display:inline-block;background:linear-gradient(135deg,#667eea 0%%,#764ba2 100%%);color:#fff;padding:14px 32px;text-decoration:none;border-radius:8px;font-size:16px;font-weight:600;margin:20px 0;">接受邀请</a>
      <p style="color:#666;font-size:14px;">此邀请将在 <strong>7 天</strong>后失效。若你还没有账号，请先用本邮箱注册。</p>
      <div style="color:#666;font-size:12px;word-break:break-all;margin-top:20px;padding:15px;background:#f8f9fa;border-radius:4px;">%s</div>
    </div>
  </div>
</body>
</html>
`, html.EscapeString(siteName), html.EscapeString(ownerEmail), acceptURL, html.EscapeString(acceptURL))
}

// ListInvitations owner 或 admin 可调用：列出企业下所有待处理邀请。
func (s *TeamService) ListInvitations(ctx context.Context, ownerUserID, actorUserID int64) ([]TeamInvitation, error) {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return nil, err
	}
	return s.teamInvitationRepo.ListPendingByOwner(ctx, ownerUserID)
}

// TeamReceivedInvitation 是"我收到的邀请"的展示结构体（被邀请人视角）。
// Token 即邮件链接中的接受凭证——本接口只返回给邮箱匹配的被邀请人本人，
// 与其收到邀请邮件所获得的信息等价，使已注册用户无需依赖邮件即可站内接受。
type TeamReceivedInvitation struct {
	ID              int64
	OwnerUserID     int64
	OwnerEmail      string
	Role            string
	QuotaMode       string
	InitialGrantUSD *float64
	Token           string
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

// ListReceivedInvitations 列出当前登录用户邮箱收到的、仍在有效期内的 pending 邀请。
// 用于已注册用户在站内直接看到并接受邀请（不依赖邀请邮件送达）。
func (s *TeamService) ListReceivedInvitations(ctx context.Context, currentUserID int64) ([]TeamReceivedInvitation, error) {
	currentUser, err := s.userRepo.GetByID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}
	email := normalizeTeamEmail(currentUser.Email)
	if email == "" {
		return []TeamReceivedInvitation{}, nil
	}
	invitations, err := s.teamInvitationRepo.ListPendingByInvitedEmail(ctx, email, time.Now())
	if err != nil {
		return nil, err
	}
	out := make([]TeamReceivedInvitation, 0, len(invitations))
	for i := range invitations {
		inv := invitations[i]
		ownerEmail := ""
		if owner, err := s.userRepo.GetByID(ctx, inv.OwnerUserID); err == nil && owner != nil {
			ownerEmail = owner.Email
		}
		out = append(out, TeamReceivedInvitation{
			ID:              inv.ID,
			OwnerUserID:     inv.OwnerUserID,
			OwnerEmail:      ownerEmail,
			Role:            inv.Role,
			QuotaMode:       inv.QuotaMode,
			InitialGrantUSD: inv.InitialGrantUSD,
			Token:           inv.Token,
			ExpiresAt:       inv.ExpiresAt,
			CreatedAt:       inv.CreatedAt,
		})
	}
	return out, nil
}

// RevokeInvitation owner 或 admin 可调用：撤销一条待处理邀请。
func (s *TeamService) RevokeInvitation(ctx context.Context, ownerUserID, actorUserID, invitationID int64) error {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return err
	}
	ok, err := s.teamInvitationRepo.Revoke(ctx, invitationID, ownerUserID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTeamInvitationNotFound
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "member.invite_revoke", fmt.Sprintf("invitation #%d", invitationID))
	return nil
}

// AcceptInvitation 由被邀请人（当前登录用户）调用：凭 token 加入企业。
// 要求当前登录用户绑定的邮箱与邀请邮箱一致，防止邀请链接被转发滥用。
// 接受成功后若邀请带初始额度，追加一笔企业→员工划转（失败不回滚成员加入，记审计）。
func (s *TeamService) AcceptInvitation(ctx context.Context, token string, currentUserID int64) (*TeamMember, error) {
	invitation, err := s.teamInvitationRepo.FindPendingByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if invitation == nil {
		return nil, ErrTeamInvitationNotFound
	}
	if time.Now().After(invitation.ExpiresAt) {
		return nil, ErrTeamInvitationExpired
	}
	if invitation.OwnerUserID == currentUserID {
		return nil, ErrTeamCannotInviteSelf
	}

	currentUser, err := s.userRepo.GetByID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(normalizeTeamEmail(currentUser.Email), invitation.InvitedEmail) {
		return nil, ErrTeamInvitationEmailMismatch
	}

	role := invitation.Role
	if validateTeamRole(role) != nil {
		role = domain.TeamMemberRoleMember
	}
	quotaMode := invitation.QuotaMode
	if validateQuotaMode(quotaMode) != nil {
		quotaMode = domain.TeamQuotaModeAllocated
	}

	// 成员数上限的权威校验在 repository 层同一事务内对 owner 行加锁后原子完成，
	// 避免多个不同邀请人并发接受时都读到"未超限"从而一起放行（TOCTOU）。
	member, err := s.teamMemberRepo.AcceptInvitation(ctx, AcceptInvitationParams{
		InvitationID: invitation.ID,
		OwnerUserID:  invitation.OwnerUserID,
		MemberUserID: currentUserID,
		Role:         role,
		DepartmentID: invitation.DepartmentID,
		QuotaMode:    quotaMode,
		AcceptedAt:   time.Now(),
		MemberLimit:  teamMemberLimit,
	})
	if err != nil {
		return nil, err
	}
	s.appendActivity(ctx, invitation.OwnerUserID, currentUserID, "member.accept", fmt.Sprintf("accepted invite as %s", currentUser.Email))

	// 初始额度划转：独立于成员加入事务，失败只记审计（管理员可后续手动划转）。
	if invitation.InitialGrantUSD != nil && *invitation.InitialGrantUSD > 0 && s.fundRepo != nil {
		note := fmt.Sprintf("initial grant for invitation #%d", invitation.ID)
		if _, err := s.fundRepo.Transfer(ctx, invitation.OwnerUserID, currentUserID, domain.TeamFundDirectionGrant, *invitation.InitialGrantUSD, invitation.OwnerUserID, note); err != nil {
			s.appendActivity(ctx, invitation.OwnerUserID, currentUserID, "fund.initial_grant_failed", fmt.Sprintf("invitation #%d amount=%.8f err=%v", invitation.ID, *invitation.InitialGrantUSD, err))
			logger.LegacyPrintf("service.team", "initial grant transfer failed: owner=%d member=%d amount=%.8f err=%v", invitation.OwnerUserID, currentUserID, *invitation.InitialGrantUSD, err)
		} else {
			s.appendActivity(ctx, invitation.OwnerUserID, currentUserID, "fund.initial_grant", fmt.Sprintf("invitation #%d amount=%.8f", invitation.ID, *invitation.InitialGrantUSD))
		}
	}
	return member, nil
}

// ListMembers 列出企业成员（owner 隐式一行 + active 成员）。
// owner/admin 见全量；普通 member 仅见 owner 行与自己一行——他人的额度模式与
// GrantedNetUSD 属管理信息，成员之间不得互见（2026-07-23 收紧，防直调 API 泄露）。
func (s *TeamService) ListMembers(ctx context.Context, ownerUserID, actorUserID int64) ([]TeamMemberView, error) {
	managerView := actorUserID == ownerUserID
	if !managerView {
		active, err := s.teamMemberRepo.FindActive(ctx, ownerUserID, actorUserID)
		if err != nil {
			return nil, err
		}
		if active == nil {
			return nil, ErrInsufficientPerms
		}
		managerView = active.Role == domain.TeamMemberRoleAdmin
	}

	owner, err := s.userRepo.GetByID(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	views := []TeamMemberView{{UserID: owner.ID, Email: owner.Email, Role: "owner"}}

	members, err := s.teamMemberRepo.ListActiveByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		m := members[i]
		if !managerView && m.MemberUserID != actorUserID {
			continue // member 视角：不返回其他成员的行
		}
		u, err := s.userRepo.GetByID(ctx, m.MemberUserID)
		if err != nil {
			continue // 成员账号已被删除等边缘场景：跳过展示，不阻断整体列表
		}
		joinedAt := m.CreatedAt
		views = append(views, TeamMemberView{
			UserID:        u.ID,
			Email:         u.Email,
			Role:          m.Role,
			DepartmentID:  m.DepartmentID,
			QuotaMode:     m.QuotaMode,
			GrantedNetUSD: m.GrantedNetUSD,
			JoinedAt:      &joinedAt,
		})
	}
	return views, nil
}

// VerifyMemberAccess 校验 actor 对 ownerUserID 企业具备管理权限，且 memberUserID
// 确实是该企业的 owner 本人或 active 成员——用于"查看指定成员详细数据"这类接口，
// 防止管理员跨企业越权查看任意用户的用量统计。
func (s *TeamService) VerifyMemberAccess(ctx context.Context, ownerUserID, actorUserID, memberUserID int64) error {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return err
	}
	if memberUserID == ownerUserID {
		return nil
	}
	m, err := s.teamMemberRepo.FindActive(ctx, ownerUserID, memberUserID)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrTeamMemberNotFound
	}
	return nil
}

// RemoveMember owner 或 admin 可调用；admin 只能移除 member，移除 admin 仅限 owner。
// 移除不自动回收余额（管理员可在移除前手动回收），仅解除成员关系。
func (s *TeamService) RemoveMember(ctx context.Context, ownerUserID, actorUserID, memberUserID int64) error {
	actorRole, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID)
	if err != nil {
		return err
	}
	if memberUserID == ownerUserID {
		return ErrTeamCannotRemoveOwner
	}
	target, err := s.teamMemberRepo.FindActive(ctx, ownerUserID, memberUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return ErrTeamMemberNotFound
	}
	if target.Role == domain.TeamMemberRoleAdmin && actorRole != "owner" {
		return ErrTeamOwnerOnly
	}
	removed, err := s.teamMemberRepo.Remove(ctx, ownerUserID, memberUserID)
	if err != nil {
		return err
	}
	if !removed {
		return ErrTeamMemberNotFound
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "member.remove", fmt.Sprintf("removed member user_id=%d", memberUserID))
	return nil
}

// SetMemberDepartment owner 或 admin 可调用：设置成员所属部门（nil = 移出部门）。
func (s *TeamService) SetMemberDepartment(ctx context.Context, ownerUserID, actorUserID, memberUserID int64, departmentID *int64) error {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return err
	}
	if departmentID != nil && s.departmentRepo != nil {
		ok, err := s.departmentRepo.Exists(ctx, *departmentID, ownerUserID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrTeamDepartmentNotFound
		}
	}
	ok, err := s.teamMemberRepo.SetDepartment(ctx, ownerUserID, memberUserID, departmentID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTeamMemberNotFound
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "member.set_department", fmt.Sprintf("member=%d department=%v", memberUserID, departmentID))
	return nil
}

// SetMemberQuotaSettings owner 或 admin 可调用：设置成员额度模式与自动补给水位。
// shared 模式要求 threshold/target 同时给出且 target > threshold > 0。
func (s *TeamService) SetMemberQuotaSettings(ctx context.Context, ownerUserID, actorUserID, memberUserID int64, quotaMode string, threshold, target *float64) error {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return err
	}
	if err := validateQuotaMode(quotaMode); err != nil {
		return err
	}
	if quotaMode == domain.TeamQuotaModeShared {
		if threshold == nil || target == nil || *threshold <= 0 || *target <= *threshold {
			return infraerrors.BadRequest("TEAM_INVALID_TOPUP_LEVELS", "shared mode requires target > threshold > 0")
		}
	} else {
		threshold, target = nil, nil
	}
	ok, err := s.teamMemberRepo.SetQuotaSettings(ctx, ownerUserID, memberUserID, quotaMode, threshold, target)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTeamMemberNotFound
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "member.set_quota", fmt.Sprintf("member=%d mode=%s", memberUserID, quotaMode))
	return nil
}

// SetMemberRole 仅 owner 可调用：提升/降级成员角色（admin/member）。
func (s *TeamService) SetMemberRole(ctx context.Context, ownerUserID, actorUserID, memberUserID int64, role string) error {
	if actorUserID != ownerUserID {
		return ErrTeamOwnerOnly
	}
	if err := validateTeamRole(role); err != nil {
		return err
	}
	ok, err := s.teamMemberRepo.SetRole(ctx, ownerUserID, memberUserID, role)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTeamMemberNotFound
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "member.set_role", fmt.Sprintf("member=%d role=%s", memberUserID, role))
	return nil
}

// ListMyTeams 返回当前用户视角的账号列表：个人账户 + 作为成员加入的所有企业。
// 余额字段只在本人行返回真实值；加入的企业行不暴露企业余额（member 无权查看）。
func (s *TeamService) ListMyTeams(ctx context.Context, currentUserID int64) ([]TeamSummary, error) {
	currentUser, err := s.userRepo.GetByID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}
	summaries := []TeamSummary{{OwnerUserID: currentUser.ID, OwnerEmail: currentUser.Email, IsPersonal: true, Role: "owner", Balance: currentUser.Balance, FrozenBalance: currentUser.FrozenBalance}}

	memberships, err := s.teamMemberRepo.ListActiveByMember(ctx, currentUserID)
	if err != nil {
		return nil, err
	}
	for i := range memberships {
		m := memberships[i]
		owner, err := s.userRepo.GetByID(ctx, m.OwnerUserID)
		if err != nil {
			continue
		}
		summaries = append(summaries, TeamSummary{OwnerUserID: owner.ID, OwnerEmail: owner.Email, IsPersonal: false, Role: m.Role})
	}
	return summaries, nil
}

// appendActivity 写企业操作审计，失败不阻断主流程。
func (s *TeamService) appendActivity(ctx context.Context, ownerUserID, actorUserID int64, action, detail string) {
	if s.teamActivityRepo == nil {
		return
	}
	if err := s.teamActivityRepo.Append(ctx, ownerUserID, actorUserID, action, detail); err != nil {
		logger.LegacyPrintf("service.team", "append team activity log failed: owner_user_id=%d actor_user_id=%d action=%s err=%v", ownerUserID, actorUserID, action, err)
	}
}
