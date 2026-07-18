package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
) {
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	// 用户管理面变更类操作入审计（含 TOTP 启用/禁用、step-up 验证、密码修改等安全事件）
	authenticated.Use(gin.HandlerFunc(auditLog))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)
			user.GET("/aff", h.User.GetAffiliate)
			user.POST("/aff/transfer", h.User.TransferAffiliateQuota)
			user.POST("/account-bindings/email/send-code", h.User.SendEmailBindingCode)
			user.POST("/account-bindings/email", h.User.BindEmailIdentity)
			user.DELETE("/account-bindings/:provider", h.User.UnbindIdentity)
			user.POST("/auth-identities/bind/start", h.User.StartIdentityBinding)
			user.GET("/api-keys/:id/usage/daily", h.Usage.GetMyAPIKeyDailyUsage)
			user.GET("/platform-quotas", h.User.GetMyPlatformQuotas)

			// 通知邮箱管理
			notifyEmail := user.Group("/notify-email")
			{
				notifyEmail.POST("/send-code", h.User.SendNotifyEmailCode)
				notifyEmail.POST("/verify", h.User.VerifyNotifyEmail)
				notifyEmail.PUT("/toggle", h.User.ToggleNotifyEmail)
				notifyEmail.DELETE("", h.User.RemoveNotifyEmail)
			}

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
				// 敏感操作二次验证：授予当前会话一段时间的 step-up 权限
				totp.POST("/step-up", h.Totp.StepUp)
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 企业组织与额度分配（Team 协作 v2，zhiguofan fork-only）
		team := authenticated.Group("/team")
		{
			team.POST("/invitations/accept/:token", h.Team.AcceptInvitation)
			team.POST("/invitations", h.Team.InviteMember)
			team.GET("/invitations", h.Team.ListInvitations)
			team.POST("/invitations/:id/resend", h.Team.ResendInvitation)
			team.DELETE("/invitations/:id", h.Team.RevokeInvitation)
			team.GET("/members", h.Team.ListMembers)
			team.DELETE("/members/:member_user_id", h.Team.RemoveMember)
			team.PUT("/members/:member_user_id/department", h.Team.SetMemberDepartment)
			team.PUT("/members/:member_user_id/quota-settings", h.Team.SetMemberQuotaSettings)
			team.PUT("/members/:member_user_id/role", h.Team.SetMemberRole)
			team.POST("/members/:member_user_id/grant", h.Team.GrantToMember)
			team.POST("/members/:member_user_id/reclaim", h.Team.ReclaimFromMember)
			team.GET("/members/:member_user_id/usage-stats", h.Team.GetMemberUsageStats)
			team.GET("/departments", h.Team.ListDepartments)
			team.POST("/departments", h.Team.CreateDepartment)
			team.PUT("/departments/:id", h.Team.UpdateDepartment)
			team.DELETE("/departments/:id", h.Team.DeleteDepartment)
			team.GET("/transfers", h.Team.ListTransfers)
			team.GET("/report", h.Team.GetReport)
		}
		authenticated.GET("/teams", h.Team.ListMyTeams)

		// 企业客户自助升级（zhiguofan fork-only: Team 协作 v2）
		enterprise := authenticated.Group("/enterprise")
		{
			enterprise.POST("/profile", h.Enterprise.Upgrade)
			enterprise.GET("/profile", h.Enterprise.GetProfile)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 用户可用渠道（非管理员接口）
		channels := authenticated.Group("/channels")
		{
			channels.GET("/available", h.AvailableChannel.List)
		}

		// 使用记录
		usage := authenticated.Group("/usage")
		{
			usage.GET("", h.Usage.List)
			usage.GET("/errors", h.Usage.ListErrors)
			usage.GET("/errors/:id", h.Usage.GetErrorDetail)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// zhiguofan fork-only: 月度对账（功能 45）；静态段优先于上方 /:id，无冲突
			usage.GET("/statement", h.Statement.GetStatement)
			usage.GET("/statement/months", h.Statement.GetMonths)
			usage.GET("/statement/export", h.Statement.Export)
			// User dashboard endpoints
			dashboard := usage.Group("/dashboard")
			{
				dashboard.GET("/stats", h.Usage.DashboardStats)
				dashboard.GET("/trend", h.Usage.DashboardTrend)
				dashboard.GET("/models", h.Usage.DashboardModels)
				dashboard.GET("/snapshot-v2", h.Usage.DashboardSnapshotV2)
				dashboard.POST("/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
			}
		}

		// 模型列表（用户可见的全部模型 + 定价信息）
		authenticated.GET("/models", h.Usage.ListModels)

		// 公告（用户可见）
		announcements := authenticated.Group("/announcements")
		{
			announcements.GET("", h.Announcement.List)
			announcements.POST("/:id/read", h.Announcement.MarkRead)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		// 渠道监控（用户只读）
		monitors := authenticated.Group("/channel-monitors")
		{
			monitors.GET("", h.ChannelMonitor.List)
			monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
		}
	}
}
