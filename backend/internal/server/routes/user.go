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
	settingService *service.SettingService,
) {
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
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
			user.GET("/api-keys/:id/usage/trend", h.Usage.GetMyAPIKeyUsageTrend)
			user.GET("/api-keys/:id/usage/models", h.Usage.GetMyAPIKeyModelStats)
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
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/tags", h.APIKey.ListTags)
			keys.POST("/batch", h.APIKey.BatchCreate)
			keys.POST("/batch-update", h.APIKey.BatchUpdate)
			keys.POST("/batch-delete", h.APIKey.BatchDelete)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 企业成员、成员分组与成员 Key 管理
		enterpriseMembers := authenticated.Group("/enterprise/members")
		{
			enterpriseMembers.GET("/import/template", h.EnterpriseMember.ImportTemplate)
			enterpriseMembers.POST("/import/preview", h.EnterpriseMember.ImportPreview)
			enterpriseMembers.POST("/import/commit", h.EnterpriseMember.ImportCommit)
			enterpriseMembers.GET("/import/jobs/:job_id", h.EnterpriseMember.GetImportJob)
			enterpriseMembers.POST("/import/jobs/:job_id/result-secrets", h.EnterpriseMember.ConsumeImportResultSecrets)
			enterpriseMembers.GET("/import/jobs/:job_id/error-report", h.EnterpriseMember.DownloadImportErrorReport)
			enterpriseMembers.GET("/usage/summary", h.EnterpriseMember.GetOwnerUsageSummary)
			enterpriseMembers.GET("/usage/trend", h.EnterpriseMember.GetOwnerUsageTrend)
			enterpriseMembers.GET("/audit", h.EnterpriseMember.ListOwnerAuditEvents)
			enterpriseMembers.GET("", h.EnterpriseMember.List)
			enterpriseMembers.POST("", h.EnterpriseMember.Create)
			enterpriseMembers.GET("/:id", h.EnterpriseMember.Get)
			enterpriseMembers.PATCH("/:id", h.EnterpriseMember.Update)
			enterpriseMembers.POST("/:id/disable", h.EnterpriseMember.Disable)
			enterpriseMembers.POST("/:id/enable", h.EnterpriseMember.Enable)
			enterpriseMembers.DELETE("/:id", h.EnterpriseMember.Delete)
			enterpriseMembers.GET("/:id/groups", h.EnterpriseMember.GetGroups)
			enterpriseMembers.PUT("/:id/groups", h.EnterpriseMember.ReplaceGroups)
			enterpriseMembers.GET("/:id/adoptable-keys", h.EnterpriseMember.ListAdoptableKeys)
			enterpriseMembers.GET("/:id/keys", h.EnterpriseMember.ListKeys)
			enterpriseMembers.POST("/:id/keys", h.EnterpriseMember.CreateKey)
			enterpriseMembers.POST("/:id/keys/:key_id/adopt", h.EnterpriseMember.AdoptKey)
			enterpriseMembers.PATCH("/:id/keys/:key_id", h.EnterpriseMember.UpdateKey)
			enterpriseMembers.DELETE("/:id/keys/:key_id", h.EnterpriseMember.DeleteKey)
			enterpriseMembers.GET("/:id/budget", h.EnterpriseMember.GetBudget)
			enterpriseMembers.GET("/:id/budget/entries", h.EnterpriseMember.ListBudgetEntries)
			enterpriseMembers.GET("/:id/audit", h.EnterpriseMember.ListAuditEvents)
			enterpriseMembers.POST("/:id/budget/adjustments", h.EnterpriseMember.CreateBudgetAdjustment)
			enterpriseMembers.PUT("/:id/usage", h.EnterpriseMember.SetUsage)
			enterpriseMembers.GET("/:id/usage/records", h.EnterpriseMember.ListUsageRecords)
			enterpriseMembers.GET("/:id/usage/analytics", h.EnterpriseMember.GetUsageAnalytics)
			enterpriseMembers.GET("/:id/usage", h.EnterpriseMember.GetUsageAnalytics)
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
			usage.GET("/members", h.Usage.ListOwnerUsageMembers)
			usage.GET("/errors", h.Usage.ListErrors)
			usage.GET("/errors/:id", h.Usage.GetErrorDetail)
			usage.GET("/analytics/summary", h.Usage.GetOwnerAPIKeyAnalyticsSummary)
			usage.GET("/analytics/leaderboard", h.Usage.GetOwnerAPIKeyAnalyticsLeaderboard)
			usage.GET("/analytics/members", h.Usage.GetOwnerMemberAnalyticsLeaderboard)
			usage.GET("/analytics/models", h.Usage.GetOwnerAPIKeyModelAnalytics)
			usage.GET("/analytics/groups", h.Usage.GetOwnerAPIKeyGroupAnalytics)
			usage.GET("/analytics/tags", h.Usage.GetOwnerAPIKeyTagAnalytics)
			usage.GET("/analytics/trend", h.Usage.GetOwnerAPIKeyUsageTrend)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.GET("/dashboard/snapshot-v2", h.Usage.DashboardSnapshotV2)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

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

		// 模型服务状态（用户只读，隐藏上游渠道与探针细节）
		modelStatus := authenticated.Group("/model-status")
		{
			modelStatus.GET("", h.ChannelMonitor.ListModelStatus)
			modelStatus.GET("/detail", h.ChannelMonitor.GetModelStatus)
		}
	}
}
