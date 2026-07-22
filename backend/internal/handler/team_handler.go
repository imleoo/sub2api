package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// TeamHandler 处理企业组织与额度分配（Team 协作 v2）相关请求。
// zhiguofan fork-only。
type TeamHandler struct {
	teamService         *service.TeamService
	departmentService   *service.TeamDepartmentService
	fundService         *service.TeamFundService
	accountUsageService *service.AccountUsageService
	settingService      *service.SettingService
}

// NewTeamHandler 创建 TeamHandler。
func NewTeamHandler(
	teamService *service.TeamService,
	departmentService *service.TeamDepartmentService,
	fundService *service.TeamFundService,
	accountUsageService *service.AccountUsageService,
	settingService *service.SettingService,
) *TeamHandler {
	return &TeamHandler{
		teamService:         teamService,
		departmentService:   departmentService,
		fundService:         fundService,
		accountUsageService: accountUsageService,
		settingService:      settingService,
	}
}

// resolveTeamOwnerID 解析本次请求要操作的企业主体 user id。
//
// v1 用全局 X-Team-Id 请求头 + 中间件切换"当前操作身份"，会连带影响 Key/支付/用量
// 等所有资源归属判断（已在 v2 回退，见 CHANGELOG）。v2 员工自管 Key，不再需要那种
// 全局身份切换；但企业允许设置多个 admin 共同管理，一个 admin 可能同时是别人企业的
// 员工——他管理"自己创建的企业"（若有）和"加入的企业"时的 ownerUserID 不同，因此
// 仅在 team/enterprise 这组接口上，允许显式带 ?owner_user_id= 声明本次操作的企业主体，
// 不传时默认为自己（管理自己创建的企业，最常见场景）。是否真的有权限操作该
// owner_user_id 由各 Service 内的 authorizeTeamManager 校验，这里只负责解析，不做鉴权。
func resolveTeamOwnerID(c *gin.Context, selfUserID int64) int64 {
	raw := c.Query("owner_user_id")
	if raw == "" {
		return selfUserID
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return selfUserID
	}
	return id
}

type teamSummaryDTO struct {
	OwnerUserID   int64   `json:"owner_user_id"`
	OwnerEmail    string  `json:"owner_email"`
	IsPersonal    bool    `json:"is_personal"`
	Role          string  `json:"role"`
	Balance       float64 `json:"balance"`
	FrozenBalance float64 `json:"frozen_balance"`
}

// ListMyTeams 列出当前用户可操作的账号：个人账户 + 作为成员加入的企业。
// GET /user/teams
func (h *TeamHandler) ListMyTeams(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	teams, err := h.teamService.ListMyTeams(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]teamSummaryDTO, 0, len(teams))
	for _, t := range teams {
		out = append(out, teamSummaryDTO{OwnerUserID: t.OwnerUserID, OwnerEmail: t.OwnerEmail, IsPersonal: t.IsPersonal, Role: t.Role, Balance: t.Balance, FrozenBalance: t.FrozenBalance})
	}
	response.Success(c, out)
}

// resolveFrontendBaseURL 解析邀请链接的前端基础 URL：优先取系统设置的 frontend_url，
// 未配置时回退为当前请求的来源（scheme + Host，尊重反代的 X-Forwarded-Proto/Host），
// 即「系统所在的 URL」。仅当两者都取不到（理论上不可能）才返回空。
func (h *TeamHandler) resolveFrontendBaseURL(c *gin.Context) string {
	if v := strings.TrimSpace(h.settingService.GetFrontendURL(c.Request.Context())); v != "" {
		return v
	}
	scheme := "http"
	if isRequestHTTPS(c) {
		scheme = "https"
	}
	host := strings.TrimSpace(c.Request.Host)
	if forwardedHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

// ResendInvitation owner 或 admin 可调用：重新发送待处理邀请邮件。
// POST /user/team/invitations/:id/resend
func (h *TeamHandler) ResendInvitation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	invitationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid invitation ID")
		return
	}
	frontendBaseURL := h.resolveFrontendBaseURL(c)
	if frontendBaseURL == "" {
		response.InternalError(c, "frontend URL is not configured")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.teamService.ResendInvitation(c.Request.Context(), ownerID, subject.UserID, invitationID, frontendBaseURL); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"resent": true})
}

type inviteMemberRequest struct {
	Email           string   `json:"email" binding:"required,email"`
	DepartmentID    *int64   `json:"department_id"`
	Role            string   `json:"role"`
	QuotaMode       string   `json:"quota_mode"`
	InitialGrantUSD *float64 `json:"initial_grant_usd"`
}

type teamInvitationDTO struct {
	ID              int64    `json:"id"`
	InvitedEmail    string   `json:"invited_email"`
	Status          string   `json:"status"`
	DepartmentID    *int64   `json:"department_id,omitempty"`
	Role            string   `json:"role"`
	QuotaMode       string   `json:"quota_mode"`
	InitialGrantUSD *float64 `json:"initial_grant_usd,omitempty"`
	ExpiresAt       string   `json:"expires_at"`
	CreatedAt       string   `json:"created_at"`
}

func toTeamInvitationDTO(inv *service.TeamInvitation) teamInvitationDTO {
	return teamInvitationDTO{
		ID:              inv.ID,
		InvitedEmail:    inv.InvitedEmail,
		Status:          inv.Status,
		DepartmentID:    inv.DepartmentID,
		Role:            inv.Role,
		QuotaMode:       inv.QuotaMode,
		InitialGrantUSD: inv.InitialGrantUSD,
		ExpiresAt:       inv.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:       inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// InviteMember owner 或 admin 可调用：邀请一个员工加入企业。
// POST /user/team/invitations
func (h *TeamHandler) InviteMember(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req inviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	frontendBaseURL := h.resolveFrontendBaseURL(c)
	if frontendBaseURL == "" {
		response.InternalError(c, "frontend URL is not configured")
		return
	}

	ownerID := resolveTeamOwnerID(c, subject.UserID)
	invitation, err := h.teamService.InviteMember(c.Request.Context(), ownerID, subject.UserID, service.TeamInviteInput{
		Email:           req.Email,
		DepartmentID:    req.DepartmentID,
		Role:            req.Role,
		QuotaMode:       req.QuotaMode,
		InitialGrantUSD: req.InitialGrantUSD,
	}, frontendBaseURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, toTeamInvitationDTO(invitation))
}

// ListInvitations owner 或 admin 可调用：列出本企业所有待处理邀请。
// GET /user/team/invitations
func (h *TeamHandler) ListInvitations(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	invitations, err := h.teamService.ListInvitations(c.Request.Context(), ownerID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]teamInvitationDTO, 0, len(invitations))
	for i := range invitations {
		out = append(out, toTeamInvitationDTO(&invitations[i]))
	}
	response.Success(c, out)
}

type teamReceivedInvitationDTO struct {
	ID              int64    `json:"id"`
	OwnerUserID     int64    `json:"owner_user_id"`
	OwnerEmail      string   `json:"owner_email"`
	Role            string   `json:"role"`
	QuotaMode       string   `json:"quota_mode"`
	InitialGrantUSD *float64 `json:"initial_grant_usd,omitempty"`
	Token           string   `json:"token"`
	ExpiresAt       string   `json:"expires_at"`
	CreatedAt       string   `json:"created_at"`
}

// ListReceivedInvitations 由被邀请人（当前登录用户）调用：列出发给自己邮箱的待处理邀请，
// 让已注册用户不依赖邀请邮件即可在站内接受（配合 AcceptInvitation 的 token 接口）。
// GET /user/team/invitations/received
func (h *TeamHandler) ListReceivedInvitations(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	invitations, err := h.teamService.ListReceivedInvitations(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]teamReceivedInvitationDTO, 0, len(invitations))
	for _, inv := range invitations {
		out = append(out, teamReceivedInvitationDTO{
			ID:              inv.ID,
			OwnerUserID:     inv.OwnerUserID,
			OwnerEmail:      inv.OwnerEmail,
			Role:            inv.Role,
			QuotaMode:       inv.QuotaMode,
			InitialGrantUSD: inv.InitialGrantUSD,
			Token:           inv.Token,
			ExpiresAt:       inv.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedAt:       inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	response.Success(c, out)
}

// RevokeInvitation owner 或 admin 可调用：撤销一条待处理邀请。
// DELETE /user/team/invitations/:id
func (h *TeamHandler) RevokeInvitation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	invitationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid invitation ID")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.teamService.RevokeInvitation(c.Request.Context(), ownerID, subject.UserID, invitationID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"revoked": true})
}

// AcceptInvitation 由被邀请人（当前登录用户）调用：凭 token 接受邀请、加入企业。
// POST /user/team/invitations/accept/:token
func (h *TeamHandler) AcceptInvitation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	token := c.Param("token")
	if strings.TrimSpace(token) == "" {
		response.BadRequest(c, "Invalid invitation token")
		return
	}
	member, err := h.teamService.AcceptInvitation(c.Request.Context(), token, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"owner_user_id": member.OwnerUserID, "role": member.Role})
}

type teamMemberDTO struct {
	UserID        int64   `json:"user_id"`
	Email         string  `json:"email"`
	Role          string  `json:"role"`
	DepartmentID  *int64  `json:"department_id,omitempty"`
	QuotaMode     string  `json:"quota_mode,omitempty"`
	GrantedNetUSD float64 `json:"granted_net_usd"`
	JoinedAt      *string `json:"joined_at,omitempty"`
}

// ListMembers 列出本企业所有成员（owner + active 成员），owner 本人或企业内活跃成员均可调用。
// GET /user/team/members
func (h *TeamHandler) ListMembers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	members, err := h.teamService.ListMembers(c.Request.Context(), ownerID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]teamMemberDTO, 0, len(members))
	for _, m := range members {
		dto := teamMemberDTO{UserID: m.UserID, Email: m.Email, Role: m.Role, DepartmentID: m.DepartmentID, QuotaMode: m.QuotaMode, GrantedNetUSD: m.GrantedNetUSD}
		if m.JoinedAt != nil {
			formatted := m.JoinedAt.Format("2006-01-02T15:04:05Z07:00")
			dto.JoinedAt = &formatted
		}
		out = append(out, dto)
	}
	response.Success(c, out)
}

// RemoveMember owner 或 admin 可调用：移除一个员工（admin 只能移除 member）。
// DELETE /user/team/members/:member_user_id
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	memberUserID, err := strconv.ParseInt(c.Param("member_user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member user ID")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.teamService.RemoveMember(c.Request.Context(), ownerID, subject.UserID, memberUserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"removed": true})
}

type setMemberDepartmentRequest struct {
	DepartmentID *int64 `json:"department_id"`
}

// SetMemberDepartment owner 或 admin 可调用：设置成员所属部门。
// PUT /user/team/members/:member_user_id/department
func (h *TeamHandler) SetMemberDepartment(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	memberUserID, err := strconv.ParseInt(c.Param("member_user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member user ID")
		return
	}
	var req setMemberDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.teamService.SetMemberDepartment(c.Request.Context(), ownerID, subject.UserID, memberUserID, req.DepartmentID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

type setMemberQuotaSettingsRequest struct {
	QuotaMode string   `json:"quota_mode" binding:"required"`
	Threshold *float64 `json:"auto_topup_threshold_usd"`
	Target    *float64 `json:"auto_topup_target_usd"`
}

// SetMemberQuotaSettings owner 或 admin 可调用：设置成员额度模式与自动补给水位。
// PUT /user/team/members/:member_user_id/quota-settings
func (h *TeamHandler) SetMemberQuotaSettings(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	memberUserID, err := strconv.ParseInt(c.Param("member_user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member user ID")
		return
	}
	var req setMemberQuotaSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.teamService.SetMemberQuotaSettings(c.Request.Context(), ownerID, subject.UserID, memberUserID, req.QuotaMode, req.Threshold, req.Target); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

type setMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// SetMemberRole 仅 owner 可调用：提升/降级成员角色。
// PUT /user/team/members/:member_user_id/role
func (h *TeamHandler) SetMemberRole(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	memberUserID, err := strconv.ParseInt(c.Param("member_user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member user ID")
		return
	}
	var req setMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.teamService.SetMemberRole(c.Request.Context(), ownerID, subject.UserID, memberUserID, req.Role); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

type teamDepartmentDTO struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"display_order"`
}

type createDepartmentRequest struct {
	Name         string `json:"name" binding:"required"`
	DisplayOrder int    `json:"display_order"`
}

// CreateDepartment owner 或 admin 可调用：创建部门。
// POST /user/team/departments
func (h *TeamHandler) CreateDepartment(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	d, err := h.departmentService.Create(c.Request.Context(), ownerID, subject.UserID, req.Name, req.DisplayOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, teamDepartmentDTO{ID: d.ID, Name: d.Name, DisplayOrder: d.DisplayOrder})
}

// ListDepartments owner 或任意 active 成员可调用：列出企业部门。
// GET /user/team/departments
func (h *TeamHandler) ListDepartments(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	departments, err := h.departmentService.List(c.Request.Context(), ownerID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]teamDepartmentDTO, 0, len(departments))
	for _, d := range departments {
		out = append(out, teamDepartmentDTO{ID: d.ID, Name: d.Name, DisplayOrder: d.DisplayOrder})
	}
	response.Success(c, out)
}

// UpdateDepartment owner 或 admin 可调用：更新部门名称/排序。
// PUT /user/team/departments/:id
func (h *TeamHandler) UpdateDepartment(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	departmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid department ID")
		return
	}
	var req createDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.departmentService.Update(c.Request.Context(), ownerID, subject.UserID, departmentID, req.Name, req.DisplayOrder); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

// DeleteDepartment owner 或 admin 可调用：删除部门。
// DELETE /user/team/departments/:id
func (h *TeamHandler) DeleteDepartment(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	departmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid department ID")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.departmentService.Delete(c.Request.Context(), ownerID, subject.UserID, departmentID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

type transferRequest struct {
	Amount float64 `json:"amount" binding:"required"`
	Note   string  `json:"note"`
}

type teamFundTransferDTO struct {
	ID             int64   `json:"id"`
	MemberUserID   int64   `json:"member_user_id"`
	MemberEmail    string  `json:"member_email"`
	Direction      string  `json:"direction"`
	Amount         float64 `json:"amount"`
	OperatorUserID int64   `json:"operator_user_id"`
	OperatorEmail  string  `json:"operator_email"`
	Note           string  `json:"note"`
	CreatedAt      string  `json:"created_at"`
}

// GrantToMember owner 或 admin 可调用：从企业余额划转给成员。
// POST /user/team/members/:member_user_id/grant
func (h *TeamHandler) GrantToMember(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	memberUserID, err := strconv.ParseInt(c.Param("member_user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member user ID")
		return
	}
	var req transferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	t, err := h.fundService.GrantToMember(c.Request.Context(), ownerID, subject.UserID, memberUserID, req.Amount, req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, teamFundTransferDTO{ID: t.ID, MemberUserID: t.MemberUserID, Direction: t.Direction, Amount: t.Amount, OperatorUserID: t.OperatorUserID, Note: t.Note, CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
}

// ReclaimFromMember owner 或 admin 可调用：从成员余额回收回企业。
// POST /user/team/members/:member_user_id/reclaim
func (h *TeamHandler) ReclaimFromMember(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	memberUserID, err := strconv.ParseInt(c.Param("member_user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member user ID")
		return
	}
	var req transferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	t, err := h.fundService.ReclaimFromMember(c.Request.Context(), ownerID, subject.UserID, memberUserID, req.Amount, req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, teamFundTransferDTO{ID: t.ID, MemberUserID: t.MemberUserID, Direction: t.Direction, Amount: t.Amount, OperatorUserID: t.OperatorUserID, Note: t.Note, CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
}

// ListTransfers owner 或 admin 可调用：划转台账分页。
// GET /user/team/transfers
func (h *TeamHandler) ListTransfers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	transfers, total, err := h.fundService.ListTransfers(c.Request.Context(), ownerID, subject.UserID, (page-1)*pageSize, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]teamFundTransferDTO, 0, len(transfers))
	for _, t := range transfers {
		out = append(out, teamFundTransferDTO{ID: t.ID, MemberUserID: t.MemberUserID, MemberEmail: t.MemberEmail, Direction: t.Direction, Amount: t.Amount, OperatorUserID: t.OperatorUserID, OperatorEmail: t.OperatorEmail, Note: t.Note, CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	response.Success(c, gin.H{"items": out, "total": total})
}

type teamReportPlatformUsageDTO struct {
	Platform        string  `json:"platform"`
	TodayActualCost float64 `json:"today_actual_cost"`
	TotalActualCost float64 `json:"total_actual_cost"`
}

type teamReportRowDTO struct {
	UserID        int64                        `json:"user_id"`
	Email         string                       `json:"email"`
	Role          string                       `json:"role"`
	DepartmentID  *int64                       `json:"department_id,omitempty"`
	QuotaMode     string                       `json:"quota_mode,omitempty"`
	Balance       float64                      `json:"balance"`
	GrantedNetUSD float64                      `json:"granted_net_usd"`
	Cost30d       float64                      `json:"cost_30d"`
	TodayCost     float64                      `json:"today_cost"`
	ByPlatform    []teamReportPlatformUsageDTO `json:"by_platform,omitempty"`
}

// GetReport owner 或 admin 可调用：成员报表（余额、净投入、近 30 天消费，
// 含按平台拆分的使用统计，与 admin 用户列表"使用统计"同一数据源）。
// GET /user/team/report
func (h *TeamHandler) GetReport(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	rows, err := h.fundService.BuildReport(c.Request.Context(), ownerID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]teamReportRowDTO, 0, len(rows))
	for _, r := range rows {
		byPlatform := make([]teamReportPlatformUsageDTO, 0, len(r.ByPlatform))
		for _, p := range r.ByPlatform {
			byPlatform = append(byPlatform, teamReportPlatformUsageDTO{Platform: p.Platform, TodayActualCost: p.TodayActualCost, TotalActualCost: p.TotalActualCost})
		}
		out = append(out, teamReportRowDTO{
			UserID: r.UserID, Email: r.Email, Role: r.Role, DepartmentID: r.DepartmentID, QuotaMode: r.QuotaMode,
			Balance: r.Balance, GrantedNetUSD: r.GrantedNetUSD, Cost30d: r.Cost30d, TodayCost: r.TodayCost, ByPlatform: byPlatform,
		})
	}
	response.Success(c, out)
}

// GetMemberUsageStats owner 或 admin 可调用：查看指定成员的详细使用统计
// （消费/请求/Token 趋势、模型分布、端点分布），与 admin 后台"用户管理"的
// 使用统计弹窗同一份数据（复用 AccountUsageService.GetUserUsageStats）。
// VerifyMemberAccess 确保只能查看本企业 owner 本人或 active 成员，不能跨企业越权查看。
// GET /user/team/members/:member_user_id/usage-stats
func (h *TeamHandler) GetMemberUsageStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	memberUserID, err := strconv.ParseInt(c.Param("member_user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member user ID")
		return
	}
	ownerID := resolveTeamOwnerID(c, subject.UserID)
	if err := h.teamService.VerifyMemberAccess(c.Request.Context(), ownerID, subject.UserID, memberUserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}
	now := timezone.Now()
	endTime := timezone.StartOfDay(now.AddDate(0, 0, 1))
	startTime := timezone.StartOfDay(now.AddDate(0, 0, -days+1))

	stats, err := h.accountUsageService.GetUserUsageStats(c.Request.Context(), memberUserID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}
