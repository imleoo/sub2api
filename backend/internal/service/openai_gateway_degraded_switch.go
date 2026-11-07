package service

// 降智检测与自动切号（degraded switch，错误码 999）。
//
// 背景：OpenAI 风控会对"环境可疑"的账号做静默流量塑形——模型被路由到低配实例，
// 表现为出站 reasoning effort 为 high/xhigh/max 时上游却返回 reasoning_tokens=0
//（有可见输出、零思考），即社区所称"降智"。
//
// 处置：检测发生在流结束后（语义字节已发给客户端，无法同请求内 failover），
// 因此对命中账号做 30 分钟调度熔断（内存 BlockAccountScheduling 即时生效 +
// DB SetTempUnschedulable 持久化，防运行时熔断被 fail-open 清除、跨实例传播），
// sticky session 由 openai_gateway_scheduling.go:999 的惰性解绑自动失效，
// 后续请求自动切到同组其他账号。
//
// 记录（双写，均 best-effort，不影响正常计费行）：
//  1. usage_logs 额外写一条零 token/零费用 marker 行（RequestType=degraded_switch）；
//     request_id 加 ":deg" 后缀避免与正常行的 (request_id, api_key_id) 去重冲突。
//  2. ops_error_logs 写 status_code=999、error_type="degraded_switch" 错误记录。

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	// openAIDegradedSwitchReason 是调度熔断与持久化 reason 的统一标记。
	openAIDegradedSwitchReason = "degraded_switch"
	// openAIDegradedSwitchDefaultCooldown 是 CooldownMinutes 配置无效时的回退值。
	openAIDegradedSwitchDefaultCooldown = 30 * time.Minute
	// degradedSwitchRequestIDSuffix 用于 marker 行 request_id，规避
	// usage_logs ON CONFLICT (request_id, api_key_id) 与正常行的去重冲突。
	degradedSwitchRequestIDSuffix = ":deg"
	// degradedSwitchErrorCode 是 ops_error_logs.status_code 上的内部错误码。
	degradedSwitchErrorCode = 999
	// degradedSwitchRequestIDMaxLen 对齐 usage_logs.request_id varchar(64)。
	degradedSwitchRequestIDMaxLen = 64
)

// isOpenAIDegradedReasoningTurn 判定本次转发是否构成"降智"证据：
// OpenAI OAuth 账号（排除 shadow/apikey），出站 effort ∈ {high,xhigh,max}，
// 有可见输出但上游报告 reasoning_tokens == 0。
// effort 为空/low/medium（监控探针、普通客户端）与空响应不在此判定。
func isOpenAIDegradedReasoningTurn(cfg *config.Config, account *Account, result *OpenAIForwardResult) bool {
	if cfg == nil || account == nil || result == nil {
		return false
	}
	if !cfg.Gateway.OpenAIDegradedSwitch.Enabled {
		return false
	}
	if account.Platform != PlatformOpenAI || account.IsShadow() || !isOpenAIOAuthAccount(account) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(derefString(result.ReasoningEffort))) {
	case "high", "xhigh", "max":
	default:
		return false
	}
	if result.Usage.OutputTokens <= 0 {
		return false
	}
	return result.Usage.ReasoningTokens <= 0
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// openAIDegradedSwitchCooldown 返回配置的熔断时长，非法配置回退 30 分钟。
func (s *OpenAIGatewayService) openAIDegradedSwitchCooldown() time.Duration {
	if s == nil || s.cfg == nil {
		return openAIDegradedSwitchDefaultCooldown
	}
	minutes := s.cfg.Gateway.OpenAIDegradedSwitch.CooldownMinutes
	if minutes <= 0 {
		return openAIDegradedSwitchDefaultCooldown
	}
	return time.Duration(minutes) * time.Minute
}

// degradedSwitchMarkerRequestID 派生 marker 行的 request_id：在正常行 request_id
// 基础上加 ":deg" 后缀；超长时截断基础部分保证总长不超 64。
func degradedSwitchMarkerRequestID(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	suffix := degradedSwitchRequestIDSuffix
	maxBase := degradedSwitchRequestIDMaxLen - len(suffix)
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return base + suffix
}

// handleOpenAIDegradedSwitch 是 RecordUsage 的降智检测钩子：判定 → 熔断（内存 +
// 持久化）→ 双写记录。全程 best-effort，任何失败只记日志，不阻断正常用量记录与计费。
func (s *OpenAIGatewayService) handleOpenAIDegradedSwitch(ctx context.Context, input *OpenAIRecordUsageInput) {
	if s == nil || input == nil || input.Result == nil {
		return
	}
	result := input.Result
	account := input.Account
	if !isOpenAIDegradedReasoningTurn(s.cfg, account, result) {
		return
	}

	cooldown := s.openAIDegradedSwitchCooldown()
	until := time.Now().Add(cooldown)
	effort := derefString(result.ReasoningEffort)

	// 内存熔断：本实例即时生效（幂等，取更晚的 until）。
	s.BlockAccountScheduling(account, until, openAIDegradedSwitchReason)
	// 持久化：跨实例传播，并使运行时熔断不被 fail-open 逻辑清除。
	if s.accountRepo != nil {
		if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, openAIDegradedSwitchReason); err != nil {
			logger.L().With(zap.String("component", "service.openai_gateway")).
				Warn("openai_degraded_switch_persist_failed",
					zap.Int64("account_id", account.ID), zap.Error(err))
		}
	}

	requestID := resolveUsageBillingRequestID(ctx, result.RequestID)
	if result.OpenAIWSMode {
		if upstreamRequestID := strings.TrimSpace(result.RequestID); upstreamRequestID != "" {
			requestID = upstreamRequestID
		}
	}
	markerRequestID := degradedSwitchMarkerRequestID(requestID)

	if s.usageLogRepo != nil {
		marker := buildDegradedSwitchUsageLog(input, markerRequestID)
		writeUsageLogBestEffort(ctx, s.usageLogRepo, marker, "service.openai_gateway")
	}
	if s.opsService != nil {
		if err := s.opsService.RecordError(ctx, buildDegradedSwitchOpsErrorInput(input, requestID, effort)); err != nil {
			logger.L().With(zap.String("component", "service.openai_gateway")).
				Warn("openai_degraded_switch_ops_error_failed",
					zap.Int64("account_id", account.ID), zap.Error(err))
		}
	}

	logger.L().With(zap.String("component", "service.openai_gateway")).
		Warn("openai_degraded_switch_triggered",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.String("model", result.Model),
			zap.String("reasoning_effort", effort),
			zap.Int("output_tokens", result.Usage.OutputTokens),
			zap.Int("reasoning_tokens", result.Usage.ReasoningTokens),
			zap.String("request_id", requestID),
			zap.Time("blocked_until", until))
}

// buildDegradedSwitchUsageLog 构造零 token/零费用的降智切号 marker 行。
// 身份/模型/端点等元数据与正常用量行同口径，便于按 request_id 关联。
func buildDegradedSwitchUsageLog(input *OpenAIRecordUsageInput, markerRequestID string) *UsageLog {
	result := input.Result
	apiKey := input.APIKey
	user := input.User
	account := input.Account

	requestedModel := result.Model
	if input.OriginalModel != "" {
		requestedModel = input.OriginalModel
	}
	usageLog := &UsageLog{
		UserID:                   user.ID,
		APIKeyID:                 apiKey.ID,
		AccountID:                account.ID,
		RequestID:                markerRequestID,
		UpstreamRequestID:        usageUpstreamRequestIDPtr(account, result.UpstreamHeaders, result.OpenAIWSMode),
		Model:                    result.Model,
		RequestedModel:           requestedModel,
		UpstreamModel:            optionalTrimmedStringPtr(result.UpstreamModel),
		UpstreamResponseModel:    optionalTrimmedStringPtr(result.UpstreamResponseModel),
		ReasoningEffort:          result.ReasoningEffort,
		RequestedReasoningEffort: coalesceRequestedReasoningEffort(result.RequestedReasoningEffort, result.ReasoningEffort),
		InboundEndpoint:          optionalTrimmedStringPtr(input.InboundEndpoint),
		UpstreamEndpoint:         optionalTrimmedStringPtr(input.UpstreamEndpoint),
		// token 与费用全零：失败型占位记录（统计以 actual_cost > 0 判成功，天然排除）。
		InputTokens:       0,
		OutputTokens:      0,
		BillingType:       BillingTypeBalance,
		RequestType:       RequestTypeDegradedSwitch,
		Stream:            result.Stream,
		OpenAIWSMode:      result.OpenAIWSMode,
		CreatedAt:         time.Now(),
		ChannelID:         optionalInt64Ptr(input.ChannelID),
		ModelMappingChain: optionalTrimmedStringPtr(input.ModelMappingChain),
	}
	billingMode := string(BillingModeToken)
	usageLog.BillingMode = &billingMode
	if input.UserAgent != "" {
		usageLog.UserAgent = &input.UserAgent
	}
	if input.IPAddress != "" {
		usageLog.IPAddress = &input.IPAddress
	}
	usageLog.SessionID = optionalTrimmedStringPtr(input.SessionID)
	if apiKey.GroupID != nil {
		usageLog.GroupID = apiKey.GroupID
	}
	if input.Subscription != nil {
		usageLog.SubscriptionID = &input.Subscription.ID
	}
	return usageLog
}

// buildDegradedSwitchOpsErrorInput 构造 ops_error_logs 记录（status_code=999）。
func buildDegradedSwitchOpsErrorInput(input *OpenAIRecordUsageInput, requestID string, effort string) *OpsInsertErrorLogInput {
	result := input.Result
	account := input.Account
	apiKey := input.APIKey

	userID := input.User.ID
	apiKeyID := apiKey.ID
	accountID := account.ID
	upstreamStatus := 200
	errorMessage := "reasoning effort=" + effort + " but reasoning_tokens=0; account scheduling blocked (degraded switch, code 999)"

	var groupID *int64
	if apiKey.GroupID != nil {
		groupID = apiKey.GroupID
	}
	requestType := int16(RequestTypeFromLegacy(result.Stream, result.OpenAIWSMode))
	clientIP := input.IPAddress

	return &OpsInsertErrorLogInput{
		RequestID:         requestID,
		UserID:            &userID,
		APIKeyID:          &apiKeyID,
		AccountID:         &accountID,
		GroupID:           groupID,
		ClientIP:          &clientIP,
		Platform:          account.Platform,
		Model:             result.Model,
		RequestedModel:    firstNonEmpty(input.OriginalModel, result.Model),
		UpstreamModel:     result.UpstreamModel,
		InboundEndpoint:   input.InboundEndpoint,
		UpstreamEndpoint:  input.UpstreamEndpoint,
		Stream:            result.Stream,
		RequestType:       &requestType,
		UserAgent:         input.UserAgent,
		ErrorPhase:        "upstream",
		ErrorType:         openAIDegradedSwitchReason,
		Severity:          "warning",
		StatusCode:        degradedSwitchErrorCode,
		ErrorMessage:      errorMessage,
		UpstreamStatusCode: &upstreamStatus,
		UpstreamErrorMessage: &errorMessage,
		CreatedAt:         time.Now(),
	}
}
