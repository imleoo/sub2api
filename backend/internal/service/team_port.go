package service

import (
	"context"
	"time"
)

// TeamMember 是企业成员关系的 service 层传输结构体，与 ent.TeamMember 解耦。
// owner_user_id 是企业主体，member_user_id 是独立登录的员工账号。
type TeamMember struct {
	ID                    int64
	OwnerUserID           int64
	MemberUserID          int64
	Role                  string
	Status                string
	DepartmentID          *int64
	QuotaMode             string
	AutoTopupThresholdUSD *float64
	AutoTopupTargetUSD    *float64
	GrantedNetUSD         float64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// TeamInvitation 是企业邀请记录的 service 层传输结构体，与 ent.TeamInvitation 解耦。
type TeamInvitation struct {
	ID               int64
	OwnerUserID      int64
	InvitedEmail     string
	Token            string
	Status           string
	ExpiresAt        time.Time
	LastSentAt       time.Time
	DepartmentID     *int64
	Role             string
	QuotaMode        string
	InitialGrantUSD  *float64
	AcceptedByUserID *int64
	AcceptedAt       *time.Time
	CreatedAt        time.Time
}

// EnterpriseProfile 是企业客户资料的 service 层传输结构体。
type EnterpriseProfile struct {
	UserID       int64
	CompanyName  string
	ContactName  string
	ContactPhone string
	Industry     string
	CreatedAt    time.Time
}

// TeamDepartment 是企业一级部门的 service 层传输结构体。
type TeamDepartment struct {
	ID           int64
	OwnerUserID  int64
	Name         string
	DisplayOrder int
	CreatedAt    time.Time
}

// TeamFundTransfer 是企业↔员工余额划转台账的 service 层传输结构体。
type TeamFundTransfer struct {
	ID             int64
	OwnerUserID    int64
	MemberUserID   int64
	Direction      string
	Amount         float64
	OperatorUserID int64
	Note           string
	CreatedAt      time.Time
}

// TeamTopupCandidate 是自动补给扫描返回的候选成员。
type TeamTopupCandidate struct {
	OwnerUserID   int64
	MemberUserID  int64
	MemberBalance float64
	TargetUSD     float64
}

// AcceptInvitationParams 是接受邀请的入参（成员属性来自邀请行）。
type AcceptInvitationParams struct {
	InvitationID int64
	OwnerUserID  int64
	MemberUserID int64
	Role         string
	DepartmentID *int64
	QuotaMode    string
	AcceptedAt   time.Time
	// MemberLimit 含 owner 本人；<=0 表示不限制。
	MemberLimit int
}

// TeamInvitationCreateInput 是创建邀请的入参。
type TeamInvitationCreateInput struct {
	OwnerUserID     int64
	InvitedEmail    string
	Token           string
	ExpiresAt       time.Time
	DepartmentID    *int64
	Role            string
	QuotaMode       string
	InitialGrantUSD *float64
}

// TeamMemberRepository 定义 service 层所需的企业成员关系数据访问端口。
type TeamMemberRepository interface {
	// Create 新增一条成员关系（owner_user_id + member_user_id 唯一）。
	Create(ctx context.Context, ownerUserID, memberUserID int64, role string) (*TeamMember, error)
	// AcceptInvitation 在同一事务中对 owner 行加锁、原子校验成员数上限、创建或重新
	// 激活成员（写入邀请携带的角色/部门/额度模式），并把 pending 邀请标记为 accepted。
	// 已是 active 成员时（幂等双击）跳过上限校验。超限时返回 ErrTeamMemberLimitReached。
	AcceptInvitation(ctx context.Context, p AcceptInvitationParams) (*TeamMember, error)
	// FindActive 查询指定 owner 下某成员的 active 关系，未找到返回 (nil, nil)。
	FindActive(ctx context.Context, ownerUserID, memberUserID int64) (*TeamMember, error)
	// ListActiveByOwner 列出某 owner 下所有 active 成员。
	ListActiveByOwner(ctx context.Context, ownerUserID int64) ([]TeamMember, error)
	// ListActiveByMember 列出某 member 所属的所有 active 企业（"我加入的企业"）。
	ListActiveByMember(ctx context.Context, memberUserID int64) ([]TeamMember, error)
	// Remove 将 active 成员标记为 removed（保留记录用于审计追溯），返回是否命中。
	Remove(ctx context.Context, ownerUserID, memberUserID int64) (bool, error)
	// SetDepartment 设置成员所属部门（nil = 移出部门），返回是否命中 active 成员。
	SetDepartment(ctx context.Context, ownerUserID, memberUserID int64, departmentID *int64) (bool, error)
	// SetQuotaSettings 设置成员额度模式与自动补给水位，返回是否命中 active 成员。
	SetQuotaSettings(ctx context.Context, ownerUserID, memberUserID int64, quotaMode string, threshold, target *float64) (bool, error)
	// SetRole 设置成员角色（admin/member），返回是否命中 active 成员。
	SetRole(ctx context.Context, ownerUserID, memberUserID int64, role string) (bool, error)
	// ListTopupCandidates 扫描 quota_mode=shared 且余额低于补给阈值的 active 成员。
	ListTopupCandidates(ctx context.Context, limit int) ([]TeamTopupCandidate, error)
}

// TeamInvitationRepository 定义 service 层所需的企业邀请记录数据访问端口。
type TeamInvitationRepository interface {
	Create(ctx context.Context, in TeamInvitationCreateInput) (*TeamInvitation, error)
	FindPendingByToken(ctx context.Context, token string) (*TeamInvitation, error)
	FindPendingByID(ctx context.Context, id, ownerUserID int64) (*TeamInvitation, error)
	FindPendingByOwnerAndEmail(ctx context.Context, ownerUserID int64, invitedEmail string, now time.Time) (*TeamInvitation, error)
	ListPendingByOwner(ctx context.Context, ownerUserID int64) ([]TeamInvitation, error)
	// ListPendingByInvitedEmail 列出发给某邮箱、仍在有效期内的 pending 邀请（被邀请人视角）。
	ListPendingByInvitedEmail(ctx context.Context, invitedEmail string, now time.Time) ([]TeamInvitation, error)
	MarkAccepted(ctx context.Context, id, acceptedByUserID int64, acceptedAt time.Time) error
	// Revoke 仅在邀请仍为 pending 且属于该 owner 时生效，返回是否命中。
	Revoke(ctx context.Context, id, ownerUserID int64) (bool, error)
	// TryMarkSent 原子地"认领"一次发送配额：仅当邀请仍 pending 且
	// last_sent_at <= now-minInterval 时才更新 last_sent_at 并返回 true；
	// 否则不修改任何数据并返回 false。调用方必须先认领成功再真正发送邮件。
	TryMarkSent(ctx context.Context, id int64, now time.Time, minInterval time.Duration) (bool, error)
}

// TeamActivityLogRepository 持久化企业控制面审计事件。
type TeamActivityLogRepository interface {
	Append(ctx context.Context, ownerUserID, actorUserID int64, action, detail string) error
}

// EnterpriseProfileRepository 定义企业客户资料数据访问端口。
type EnterpriseProfileRepository interface {
	// Create 创建企业资料；user_id 已存在时返回 ErrEnterpriseAlreadyUpgraded。
	Create(ctx context.Context, userID int64, companyName, contactName, contactPhone, industry string) (*EnterpriseProfile, error)
	// GetByUserID 查询企业资料，未找到返回 (nil, nil)。
	GetByUserID(ctx context.Context, userID int64) (*EnterpriseProfile, error)
}

// TeamDepartmentRepository 定义企业一级部门数据访问端口。
type TeamDepartmentRepository interface {
	// Create 创建部门；同名冲突返回 ErrTeamDepartmentNameTaken。
	Create(ctx context.Context, ownerUserID int64, name string, displayOrder int) (*TeamDepartment, error)
	List(ctx context.Context, ownerUserID int64) ([]TeamDepartment, error)
	// Exists 校验部门属于该企业。
	Exists(ctx context.Context, id, ownerUserID int64) (bool, error)
	// Update 更新名称与排序，返回是否命中。
	Update(ctx context.Context, id, ownerUserID int64, name string, displayOrder int) (bool, error)
	// Delete 删除部门（成员的 department_id 由外键 SET NULL），返回是否命中。
	Delete(ctx context.Context, id, ownerUserID int64) (bool, error)
}

// TeamFundRepository 定义企业↔员工余额划转数据访问端口。
type TeamFundRepository interface {
	// Transfer 原子划转：事务内按 min/max(user_id) 顺序对两个 users 行 FOR UPDATE，
	// 锁内校验（grant/auto_topup：企业余额充足；reclaim：amount ≤ min(granted_net,
	// 员工余额)），双方余额更新 + 台账 INSERT + team_members.granted_net_usd 更新。
	// 要求成员关系为 active。
	Transfer(ctx context.Context, ownerUserID, memberUserID int64, direction string, amount float64, operatorUserID int64, note string) (*TeamFundTransfer, error)
	// ListByOwner 台账分页（按 created_at 倒序）。
	ListByOwner(ctx context.Context, ownerUserID int64, offset, limit int) ([]TeamFundTransfer, int, error)
}
