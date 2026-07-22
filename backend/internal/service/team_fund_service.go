package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）——余额划转。
//
// 计费模型：员工 Key 消费扣员工自己的余额；企业通过真实划转分配额度。
// 原子性由 repository 层 Transfer 保证（双行 FOR UPDATE 固定锁序），本层
// 只做权限与参数校验。

// teamTransferMaxAmount 单笔划转上限，防误操作（如把 1e9 当分输入）。
const teamTransferMaxAmount = 1_000_000

// TeamFundReportRow 是企业成员报表的一行。
// Cost30d 与 TodayCost/ByPlatform 复用 admin 用户列表"使用统计"同一套语义
// （近 30 天合计 / 当天消费 / 按平台拆分），与 usagestats.BatchUserUsageStats 对齐。
type TeamFundReportRow struct {
	UserID        int64
	Email         string
	Role          string
	DepartmentID  *int64
	QuotaMode     string
	Balance       float64
	GrantedNetUSD float64
	Cost30d       float64
	TodayCost     float64
	ByPlatform    []usagestats.PlatformUsage
}

// TeamUsageStatsProvider 是成员用量聚合的端口（由 DashboardService 适配实现，
// 复用 GetBatchUserUsageStats，与 admin 用户列表"使用统计"列同一数据源）。
type TeamUsageStatsProvider interface {
	GetBatchUserUsageStats(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchUserUsageStats, error)
}

type dashboardTeamUsageAdapter struct {
	ds *DashboardService
}

// NewTeamUsageStatsProvider 用 DashboardService 适配 TeamUsageStatsProvider。
func NewTeamUsageStatsProvider(ds *DashboardService) TeamUsageStatsProvider {
	return dashboardTeamUsageAdapter{ds: ds}
}

func (a dashboardTeamUsageAdapter) GetBatchUserUsageStats(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchUserUsageStats, error) {
	if a.ds == nil {
		return map[int64]*usagestats.BatchUserUsageStats{}, nil
	}
	return a.ds.GetBatchUserUsageStats(ctx, userIDs, startTime, endTime)
}

// TeamFundService 提供企业↔员工额度划转、台账与报表。
type TeamFundService struct {
	fundRepo       TeamFundRepository
	teamMemberRepo TeamMemberRepository
	userRepo       UserRepository
	activityRepo   TeamActivityLogRepository
	usageStats     TeamUsageStatsProvider
}

// NewTeamFundService 创建 TeamFundService 实例。
func NewTeamFundService(
	fundRepo TeamFundRepository,
	teamMemberRepo TeamMemberRepository,
	userRepo UserRepository,
	activityRepo TeamActivityLogRepository,
	usageStats TeamUsageStatsProvider,
) *TeamFundService {
	return &TeamFundService{
		fundRepo:       fundRepo,
		teamMemberRepo: teamMemberRepo,
		userRepo:       userRepo,
		activityRepo:   activityRepo,
		usageStats:     usageStats,
	}
}

func validateTransferAmount(amount float64) error {
	if amount <= 0 {
		return ErrTeamInvalidTransferAmount
	}
	if amount > teamTransferMaxAmount {
		return infraerrors.BadRequest("TEAM_TRANSFER_AMOUNT_TOO_LARGE", "transfer amount exceeds the per-operation cap")
	}
	return nil
}

// GrantToMember owner 或 admin 可调用：从企业余额划转 amount 给成员。
func (s *TeamFundService) GrantToMember(ctx context.Context, ownerUserID, actorUserID, memberUserID int64, amount float64, note string) (*TeamFundTransfer, error) {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return nil, err
	}
	if err := validateTransferAmount(amount); err != nil {
		return nil, err
	}
	t, err := s.fundRepo.Transfer(ctx, ownerUserID, memberUserID, domain.TeamFundDirectionGrant, amount, actorUserID, note)
	if err != nil {
		return nil, err
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "fund.grant", fmt.Sprintf("member=%d amount=%.8f", memberUserID, amount))
	return t, nil
}

// ReclaimFromMember owner 或 admin 可调用：从成员余额回收 amount 回企业。
// 上限 = min(granted_net_usd, 成员当前余额)，由 repository 锁内校验。
func (s *TeamFundService) ReclaimFromMember(ctx context.Context, ownerUserID, actorUserID, memberUserID int64, amount float64, note string) (*TeamFundTransfer, error) {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return nil, err
	}
	if err := validateTransferAmount(amount); err != nil {
		return nil, err
	}
	t, err := s.fundRepo.Transfer(ctx, ownerUserID, memberUserID, domain.TeamFundDirectionReclaim, amount, actorUserID, note)
	if err != nil {
		return nil, err
	}
	s.appendActivity(ctx, ownerUserID, actorUserID, "fund.reclaim", fmt.Sprintf("member=%d amount=%.8f", memberUserID, amount))
	return t, nil
}

// TeamFundTransferView 是台账行的展示视图：附带成员/操作人 email（台账需可读地
// 回答"划给了谁、谁操作的"，裸 user_id 对管理员不可读）。
type TeamFundTransferView struct {
	TeamFundTransfer
	MemberEmail   string
	OperatorEmail string
}

// ListTransfers owner 或 admin 可调用：台账分页（行内解析成员/操作人 email，
// 账号已删除等边缘场景 email 留空、不阻断列表）。
func (s *TeamFundService) ListTransfers(ctx context.Context, ownerUserID, actorUserID int64, offset, limit int) ([]TeamFundTransferView, int, error) {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return nil, 0, err
	}
	transfers, total, err := s.fundRepo.ListByOwner(ctx, ownerUserID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	emailCache := map[int64]string{}
	resolveEmail := func(userID int64) string {
		if userID == 0 {
			return ""
		}
		if email, ok := emailCache[userID]; ok {
			return email
		}
		email := ""
		if u, err := s.userRepo.GetByID(ctx, userID); err == nil && u != nil {
			email = u.Email
		}
		emailCache[userID] = email
		return email
	}
	views := make([]TeamFundTransferView, 0, len(transfers))
	for i := range transfers {
		views = append(views, TeamFundTransferView{
			TeamFundTransfer: transfers[i],
			MemberEmail:      resolveEmail(transfers[i].MemberUserID),
			OperatorEmail:    resolveEmail(transfers[i].OperatorUserID),
		})
	}
	return views, total, nil
}

// BuildReport owner 或 admin 可调用：成员报表（余额、净投入、近 30 天消费）。
func (s *TeamFundService) BuildReport(ctx context.Context, ownerUserID, actorUserID int64) ([]TeamFundReportRow, error) {
	if _, err := authorizeTeamManager(ctx, s.teamMemberRepo, ownerUserID, actorUserID); err != nil {
		return nil, err
	}
	members, err := s.teamMemberRepo.ListActiveByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}

	rows := make([]TeamFundReportRow, 0, len(members)+1)
	userIDs := make([]int64, 0, len(members)+1)

	owner, err := s.userRepo.GetByID(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	rows = append(rows, TeamFundReportRow{UserID: owner.ID, Email: owner.Email, Role: "owner", Balance: owner.Balance})
	userIDs = append(userIDs, owner.ID)

	for i := range members {
		m := members[i]
		u, err := s.userRepo.GetByID(ctx, m.MemberUserID)
		if err != nil {
			continue
		}
		rows = append(rows, TeamFundReportRow{
			UserID:        u.ID,
			Email:         u.Email,
			Role:          m.Role,
			DepartmentID:  m.DepartmentID,
			QuotaMode:     m.QuotaMode,
			Balance:       u.Balance,
			GrantedNetUSD: m.GrantedNetUSD,
		})
		userIDs = append(userIDs, u.ID)
	}

	if s.usageStats != nil {
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		stats, err := s.usageStats.GetBatchUserUsageStats(ctx, userIDs, start, end)
		if err == nil {
			for i := range rows {
				if st := stats[rows[i].UserID]; st != nil {
					rows[i].Cost30d = st.TotalActualCost
					rows[i].TodayCost = st.TodayActualCost
					rows[i].ByPlatform = st.ByPlatform
				}
			}
		}
		// 用量聚合失败不阻断报表主体（余额/净投入仍然可用）。
	}
	return rows, nil
}

func (s *TeamFundService) appendActivity(ctx context.Context, ownerUserID, actorUserID int64, action, detail string) {
	if s.activityRepo == nil {
		return
	}
	_ = s.activityRepo.Append(ctx, ownerUserID, actorUserID, action, detail)
}
