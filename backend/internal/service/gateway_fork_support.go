package service

// fork 独有的 Claude 网关支撑函数（上游 0.1.147 拆分 gateway_service.go 时无对应位置）。
//   - listSchedulableAccountsBase / invalidRequestFallbackGroupID：功能 25（generic 调度）/ 功能 37（跨组兜底）
//   - calculateVideoCost：功能 34（视频按秒计费）
//   - resolveProviderKey / resolveUpstreamModelForCost：功能 14（上游成本快照）
//   - claudeUsageToTokens：计费口径归一

import (
	"context"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// invalidRequestFallbackGroupID 返回当前组配置的「无效请求兜底组」ID
// （FallbackGroupIDOnInvalidRequest）；无配置或查询失败返回 nil。
func (s *GatewayService) invalidRequestFallbackGroupID(ctx context.Context, groupID *int64) *int64 {
	if groupID == nil {
		return nil
	}
	group, err := s.resolveGroupByID(ctx, *groupID)
	if err != nil || group == nil {
		return nil
	}
	return group.FallbackGroupIDOnInvalidRequest
}

func (s *GatewayService) listSchedulableAccountsBase(ctx context.Context, groupID *int64, platform string, hasForcePlatform bool) ([]Account, bool, error) {
	if s.schedulerSnapshot != nil {
		accounts, useMixed, err := s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, platform, hasForcePlatform)
		if err == nil {
			slog.Debug("account_scheduling_list_snapshot",
				"group_id", derefGroupID(groupID),
				"platform", platform,
				"use_mixed", useMixed,
				"count", len(accounts))
			if slog.Default().Enabled(ctx, slog.LevelDebug) {
				for _, acc := range accounts {
					slog.Debug("account_scheduling_account_detail",
						"account_id", acc.ID,
						"name", acc.Name,
						"platform", acc.Platform,
						"type", acc.Type,
						"status", acc.Status,
						"tls_fingerprint", acc.IsTLSFingerprintEnabled())
				}
			}
		}
		return accounts, useMixed, err
	}
	useMixed := false
	var accounts []Account
	var err error
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, platform)
	} else if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, platform)
		// 分组内无账号则返回空列表，由上层处理错误，不再回退到全平台查询
	} else {
		accounts, err = s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, platform)
	}
	if err != nil {
		slog.Debug("account_scheduling_list_failed",
			"group_id", derefGroupID(groupID),
			"platform", platform,
			"error", err)
		return nil, useMixed, err
	}
	slog.Debug("account_scheduling_list_single",
		"group_id", derefGroupID(groupID),
		"platform", platform,
		"count", len(accounts))
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		for _, acc := range accounts {
			slog.Debug("account_scheduling_account_detail",
				"account_id", acc.ID,
				"name", acc.Name,
				"platform", acc.Platform,
				"type", acc.Type,
				"status", acc.Status,
				"tls_fingerprint", acc.IsTLSFingerprintEnabled())
		}
	}
	return accounts, useMixed, nil
}

// calculateVideoCost 计算视频生成费用：渠道级别 per_request 定价优先，否则按 BillingService.CalculateVideoCost 走"单价 × 秒数"。
func (s *GatewayService) calculateVideoCost(
	ctx context.Context,
	result *ForwardResult,
	apiKey *APIKey,
	billingModel string,
	multiplier float64,
) *CostBreakdown {
	// 渠道定价（per_request mode + intervals）兜底优先级仍然存在，方便管理员针对单一渠道做覆盖。
	if resolved := s.resolveChannelPricing(ctx, billingModel, apiKey); resolved != nil {
		tokens := UsageTokens{
			InputTokens:       result.Usage.InputTokens,
			OutputTokens:      result.Usage.OutputTokens,
			ImageOutputTokens: result.Usage.ImageOutputTokens,
		}
		gid := apiKey.Group.ID
		cost, err := s.billingService.CalculateCostUnified(CostInput{
			Ctx:            ctx,
			Model:          billingModel,
			GroupID:        &gid,
			Tokens:         tokens,
			RequestCount:   1,
			RateMultiplier: multiplier,
			Resolver:       s.resolver,
			Resolved:       resolved,
		})
		if err != nil {
			logger.LegacyPrintf("service.gateway", "Calculate video channel cost failed: %v", err)
			return &CostBreakdown{ActualCost: 0}
		}
		return cost
	}
	return s.billingService.CalculatePerSecondVideoCost(billingModel, result.VideoSeconds, multiplier)
}

// resolveProviderKey 从账号推导 provider_key（Phase 1 P1-1 已升级）。
//
// 优先级：
//  1. account.extra.provider（DeepSeek / 硅基流动等 OpenAI-compatible 渠道写在 extra）
//  2. account.Platform（原厂账号：anthropic / openai / gemini / lingjing）
//
// 两路均经 NormalizeProvider 规范化（详见 docs/glossary.md §1.3 别名表）。
func resolveProviderKey(account *Account) string {
	if account == nil {
		return ""
	}
	if v := NormalizeProvider(account.GetExtraString("provider")); v != "" {
		return v
	}
	return NormalizeProvider(account.Platform)
}

// resolveUpstreamModelForCost 取上游模型名（命中 provider_pricing 用），
// 优先 result.UpstreamModel（已应用模型映射），fallback result.Model。
func resolveUpstreamModelForCost(result *ForwardResult) string {
	if result == nil {
		return ""
	}
	if result.UpstreamModel != "" {
		return result.UpstreamModel
	}
	return result.Model
}

// claudeUsageToTokens 把 ClaudeUsage 映射成 UsageTokens（Phase 0 P0-5 上游成本快照用）。
func claudeUsageToTokens(u ClaudeUsage) UsageTokens {
	return UsageTokens{
		InputTokens:           u.InputTokens,
		OutputTokens:          u.OutputTokens,
		CacheCreationTokens:   u.CacheCreationInputTokens,
		CacheReadTokens:       u.CacheReadInputTokens,
		CacheCreation5mTokens: u.CacheCreation5mTokens,
		CacheCreation1hTokens: u.CacheCreation1hTokens,
		ImageOutputTokens:     u.ImageOutputTokens,
	}
}
