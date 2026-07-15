package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

func (s *OpenAIGatewayService) handleOpenAIAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, requestedModel ...string) bool {
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()

	if isOpenAIImageRateLimitError(statusCode, responseBody) {
		if s != nil && s.rateLimitService != nil {
			_ = s.rateLimitService.HandleOpenAIImageRateLimit(stateCtx, account, statusCode, headers, responseBody)
		}
		return false
	}

	if s == nil || account == nil || s.rateLimitService == nil {
		return false
	}
	if len(requestedModel) > 0 && s.rateLimitService.HandleUpstreamModelNotFound(stateCtx, account, requestedModel[0], statusCode, responseBody) {
		return true
	}
	shouldDisable := s.rateLimitService.HandleUpstreamError(stateCtx, account, statusCode, headers, responseBody)
	if shouldDisable {
		s.BlockAccountScheduling(account, time.Time{}, "upstream_disable")
	}
	return shouldDisable
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
