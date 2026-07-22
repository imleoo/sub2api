package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// GrokCredentialUnavailableClientMessage is the client-facing message returned
// when account failover is exhausted while getting an upstream credential.
const GrokCredentialUnavailableClientMessage = "No available upstream credential; please retry later"

// Grok OAuth 逆向订阅（功能 35）已整体移除，account.IsGrokOAuth() 恒为 false，
// getRequestCredential 退化为对 GetAccessToken 的直接透传（无需 OAuth 失败切换/重试逻辑）。
// 保留这两个转发热路径广泛调用的符号，避免逐个改动调用点。

func (a *Account) IsGrokOAuth() bool { return false }

func (s *OpenAIGatewayService) getRequestCredential(ctx context.Context, _ *gin.Context, account *Account) (string, string, error) {
	return s.GetAccessToken(ctx, account)
}

// OpenAIOAuth429FailoverState 曾用于跟踪 OAuth 账号共享配额池触发的 429 风暴场景，
// 该场景随订阅逆向清理（功能 35）一并移除；保留空结构体仅为兼容调用点签名。
type OpenAIOAuth429FailoverState struct{}

// ShouldStopOpenAIOAuth429Failover 恒返回 false：apikey 账号使用独立配额，不存在
// OAuth 共享池 429 风暴场景，切换账号数仅受调用方自身的 maxAccountSwitches 限制。
func (s *OpenAIGatewayService) ShouldStopOpenAIOAuth429Failover(_ *Account, _ int, _ int, _ *OpenAIOAuth429FailoverState) bool {
	return false
}

// extractOpenAICodexProbeUpdates 从探测响应头提取 codex 5h/7d 限流快照，用于回写
// account.Extra。apikey/compact 两条探测路径共用（与账号是否曾用于 OAuth 无关）。
func extractOpenAICodexProbeUpdates(resp *http.Response) (map[string]any, error) {
	if resp == nil {
		return nil, nil
	}
	if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
		return buildCodexUsageExtraUpdates(snapshot, time.Now()), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai codex probe returned status %d", resp.StatusCode)
	}
	return nil, nil
}

const (
	openAIAccountStateUpdateTimeout    = 5 * time.Second
	openAIStopSchedulingBridgeCooldown = 2 * time.Minute
)

// Codex "agent identity" 冒充（伪装官方 Codex CLI 设备身份访问订阅账号）随订阅逆向清理
// 功能 35 一并移除。以下守卫函数保留最小 no-op 实现，使调用方分支恒不可达，
// 避免为清理每个调用点而改动大量转发热路径文件。

func (s *OpenAIGatewayService) isAgentIdentityAccount(_ context.Context, _ *Account) bool {
	return false
}

func (s *OpenAIGatewayService) recoverAgentIdentityTask(_ context.Context, _ *Account, _ string) error {
	return errors.New("agent identity task recovery is not supported")
}

func agentIdentityTaskRecoveryWasTried(_ context.Context) bool {
	return true
}

func markAgentIdentityTaskRecoveryTried(ctx context.Context) context.Context {
	return ctx
}

func isAgentIdentityTaskInvalidHTTPResponse(_ int, _ []byte) bool {
	return false
}

func isAgentIdentityTaskInvalidWSDialError(_ error) bool {
	return false
}

// buildOpenAIAuthenticationHeaders 构造转发到上游的鉴权请求头。fork 仅支持 apikey
// 接入（Codex agent identity 冒充随功能 35 移除），恒使用标准 Bearer token。
func (s *OpenAIGatewayService) buildOpenAIAuthenticationHeaders(_ context.Context, _ *Account, token string) (http.Header, error) {
	headers := make(http.Header, 1)
	headers.Set("authorization", "Bearer "+token)
	return headers, nil
}

// redactAgentIdentitySensitiveBody 已随 Codex agent identity 冒充功能移除，原样返回。
func (s *OpenAIGatewayService) redactAgentIdentitySensitiveBody(_ context.Context, _ *Account, body []byte) []byte {
	return body
}

// refreshOpenAIAgentIdentityHeaders 已随 Codex agent identity 冒充功能移除，原样返回。
func (s *OpenAIGatewayService) refreshOpenAIAgentIdentityHeaders(_ context.Context, _ *Account, headers http.Header) (http.Header, error) {
	return headers, nil
}

// redactAgentIdentitySensitiveBodyForAccount 是上面同名方法的包级变体，供不持有
// OpenAIGatewayService 接收者的调用方（如 AccountTestService）使用；原样返回。
func redactAgentIdentitySensitiveBodyForAccount(_ context.Context, _ AccountRepository, _ *Account, body []byte) []byte {
	return body
}

// buildAgentIdentityAuthenticationHeaders 与 ensureAgentIdentityTaskForAccount 恒不可达
// （调用方均已由 Account.IsOpenAIAgentIdentity() 恒 false 短路），仅保留签名以兼容调用点。
func buildAgentIdentityAuthenticationHeaders(_ context.Context, _ AccountRepository, _ any, _ *sync.Mutex, _ *Account) (http.Header, error) {
	return nil, errors.New("agent identity authentication is not supported")
}

func ensureAgentIdentityTaskForAccount(_ context.Context, _ AccountRepository, _ any, _ *sync.Mutex, _ *Account, _ string) error {
	return errors.New("agent identity task recovery is not supported")
}

func resolveCredentialAccount(_ context.Context, _ AccountRepository, account *Account) (*Account, error) {
	return account, nil
}

func openAIAccountStateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, openAIAccountStateUpdateTimeout)
}

func isOpenAIAccount(account *Account) bool {
	// 官方 xAI(Grok) 账号复用 openai 网关的运行时封锁/冷却快路径。
	return account != nil && (account.Platform == PlatformOpenAI || account.Platform == PlatformGrok)
}

// isGrokOAuthAccount 已随 Grok OAuth 逆向订阅（功能 35）移除，恒为 false。
func isGrokOAuthAccount(_ *Account) bool { return false }

// agentIdentityTaskRecoveredError 标记通过冒充官方 Codex CLI 设备身份（功能 35）恢复会话
// 成功的错误分支；该冒充链路已整体移除，此类型仅保留最小实现以满足编译。
type agentIdentityTaskRecoveredError struct{}

func (*agentIdentityTaskRecoveredError) Error() string { return "agent identity task recovered" }

// handleOpenAIAccountUpstreamError expects canonicalModel to be the model used
// for scheduling after applying account mapping exactly once.
func (s *OpenAIGatewayService) handleOpenAIAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, canonicalModel ...string) bool {
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()

	if isOpenAIImageRateLimitError(statusCode, responseBody) {
		if s != nil && s.rateLimitService != nil {
			_ = s.rateLimitService.HandleOpenAIImageRateLimit(stateCtx, account, statusCode, headers, responseBody)
		}
		return false
	}

	if s == nil || account == nil {
		return false
	}
	stateCtx = withTempUnschedulableModel(stateCtx, canonicalModel)
	if s.rateLimitService != nil && len(canonicalModel) > 0 && s.rateLimitService.HandleUpstreamModelNotFound(stateCtx, account, canonicalModel[0], statusCode, responseBody) {
		return true
	}
	// Isolate a custom temporary-unschedulable match to the known upstream
	// model before entering the generic account error path. This keeps the
	// account available to other models and avoids the account runtime blocker.
	if s.rateLimitService != nil && statusCode != http.StatusUnauthorized && len(canonicalModel) > 0 && strings.TrimSpace(canonicalModel[0]) != "" &&
		s.rateLimitService.HandleTempUnschedulable(stateCtx, account, statusCode, responseBody, canonicalModel[0]) {
		return true
	}
	if s.rateLimitService == nil {
		return false
	}
	shouldDisable := s.rateLimitService.HandleUpstreamError(stateCtx, account, statusCode, headers, responseBody)
	modelTempMatched := statusCode != http.StatusUnauthorized && tempUnschedulableModel(stateCtx, nil) != "" &&
		len(matchTempUnschedulableRules(account, statusCode, responseBody)) > 0
	if shouldDisable && !modelTempMatched {
		s.BlockAccountScheduling(account, time.Time{}, "upstream_disable")
	}
	if !shouldDisable && account.Platform == PlatformOpenAI && account.Type == AccountTypeAPIKey && shouldCooldownOpenAITransientUpstreamError(statusCode, responseBody) {
		model := ""
		if len(canonicalModel) > 0 {
			model = canonicalModel[0]
		}
		decision := s.recordOpenAIAccountModelTransientFailure(account, model, time.Now())
		if decision.FailureStreak > 0 {
			slog.Warn("openai_model_transient_state",
				"account_id", account.ID,
				"model", openAIAccountModelTransientModel(model),
				"failure_streak", decision.FailureStreak,
				"cooldown_ms", decision.Cooldown.Milliseconds(),
				"block_scope", "account_model",
			)
		}
	}
	return shouldDisable
}

func shouldCooldownOpenAITransientUpstreamError(statusCode int, responseBody []byte) bool {
	switch statusCode {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 520, 521, 522, 523, 524:
		return true
	case http.StatusBadRequest:
		return isOpenAITransientProcessingError(statusCode, "", responseBody)
	default:
		return false
	}
}

func (s *OpenAIGatewayService) BlockAccountScheduling(account *Account, until time.Time, reason string) {
	if s == nil || !isOpenAIAccount(account) {
		return
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	_, _ = s.blockAccountSchedulingLocked(account, until, reason)
}

func (s *OpenAIGatewayService) openAIAccountRuntimeBlockLock(accountID int64) *sync.Mutex {
	actual, _ := s.openaiAccountRuntimeBlockLocks.LoadOrStore(accountID, &sync.Mutex{})
	mu, ok := actual.(*sync.Mutex)
	if !ok {
		mu = &sync.Mutex{}
		s.openaiAccountRuntimeBlockLocks.Store(accountID, mu)
	}
	return mu
}

func (s *OpenAIGatewayService) blockAccountSchedulingLocked(account *Account, until time.Time, _ string) (uint64, bool) {
	generation := s.openaiAccountRuntimeBlockSequence.Add(1)
	s.openaiAccountRuntimeBlockGeneration.Store(account.ID, generation)
	now := time.Now()
	blockUntil := until
	if blockUntil.IsZero() || !blockUntil.After(now) {
		blockUntil = now.Add(openAIStopSchedulingBridgeCooldown)
	}

	for {
		current, loaded := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
		if !loaded {
			actual, stored := s.openaiAccountRuntimeBlockUntil.LoadOrStore(account.ID, blockUntil)
			if !stored {
				return generation, true
			}
			current = actual
		}

		currentUntil, ok := current.(time.Time)
		if !ok || currentUntil.IsZero() {
			if s.openaiAccountRuntimeBlockUntil.CompareAndSwap(account.ID, current, blockUntil) {
				return generation, true
			}
			continue
		}
		if !blockUntil.After(currentUntil) {
			return generation, false
		}
		if s.openaiAccountRuntimeBlockUntil.CompareAndSwap(account.ID, current, blockUntil) {
			return generation, true
		}
	}
}

func (s *OpenAIGatewayService) ClearAccountSchedulingBlock(accountID int64) {
	if s == nil || accountID <= 0 {
		return
	}
	mu := s.openAIAccountRuntimeBlockLock(accountID)
	mu.Lock()
	defer mu.Unlock()
	s.openaiAccountRuntimeBlockUntil.Delete(accountID)
	s.openaiAccountRuntimeBlockGeneration.Store(accountID, s.openaiAccountRuntimeBlockSequence.Add(1))
}

func (s *OpenAIGatewayService) isOpenAIAccountRuntimeBlocked(account *Account) bool {
	if s == nil || !isOpenAIAccount(account) {
		return false
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	value, ok := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
	if !ok {
		return false
	}
	cooldownUntil, ok := value.(time.Time)
	if !ok || cooldownUntil.IsZero() {
		s.openaiAccountRuntimeBlockUntil.Delete(account.ID)
		s.openaiAccountRuntimeBlockGeneration.Store(account.ID, s.openaiAccountRuntimeBlockSequence.Add(1))
		return false
	}
	if time.Now().Before(cooldownUntil) {
		return true
	}
	s.openaiAccountRuntimeBlockUntil.Delete(account.ID)
	s.openaiAccountRuntimeBlockGeneration.Store(account.ID, s.openaiAccountRuntimeBlockSequence.Add(1))
	return false
}

func (s *OpenAIGatewayService) getOpenAIAccountModelTransientState() *openAIAccountModelTransientState {
	if s == nil {
		return nil
	}
	s.openaiModelTransientOnce.Do(func() {
		if s.openaiModelTransient == nil {
			s.openaiModelTransient = newOpenAIAccountModelTransientState(openAIModelTransientDefaultMax)
		}
	})
	return s.openaiModelTransient
}

func canonicalOpenAIAccountSchedulingModel(account *Account, requestedModel string) string {
	model := strings.TrimSpace(requestedModel)
	if account == nil || model == "" {
		return model
	}
	if mapped := strings.TrimSpace(account.GetMappedModel(model)); mapped != "" {
		return mapped
	}
	return model
}

func openAIAccountModelTransientModel(canonicalModel string) string {
	return normalizeOpenAIAccountModelTransientModel(canonicalModel)
}

func (s *OpenAIGatewayService) recordOpenAIAccountModelTransientFailure(account *Account, canonicalModel string, now time.Time) openAIAccountModelTransientDecision {
	if s == nil || account == nil {
		return openAIAccountModelTransientDecision{}
	}
	state := s.getOpenAIAccountModelTransientState()
	if state == nil {
		return openAIAccountModelTransientDecision{}
	}
	return state.recordFailure(account.ID, openAIAccountModelTransientModel(canonicalModel), now)
}

func (s *OpenAIGatewayService) clearOpenAIAccountModelTransientState(accountID int64, model string) {
	state := s.getOpenAIAccountModelTransientState()
	if state == nil {
		return
	}
	state.recordSuccess(accountID, model)
}

func (s *OpenAIGatewayService) isOpenAIAccountModelRuntimeBlocked(account *Account, requestedModel string) bool {
	if s == nil || account == nil {
		return false
	}
	state := s.getOpenAIAccountModelTransientState()
	if state == nil {
		return false
	}
	canonicalModel := canonicalOpenAIAccountSchedulingModel(account, requestedModel)
	return state.isBlocked(account.ID, openAIAccountModelTransientModel(canonicalModel), time.Now())
}

func (s *OpenAIGatewayService) isOpenAIAccountRequestRuntimeBlocked(account *Account, requestedModel string) bool {
	return s != nil && (s.isOpenAIAccountRuntimeBlocked(account) || s.isOpenAIAccountModelRuntimeBlocked(account, requestedModel))
}
