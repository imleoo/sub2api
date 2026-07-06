package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	httppool "github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	openaipkg "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"golang.org/x/sync/errgroup"
)

type UsageLogRepository interface {
	// Create creates a usage log and returns whether it was actually inserted.
	// inserted is false when the insert was skipped due to conflict (idempotent retries).
	Create(ctx context.Context, log *UsageLog) (inserted bool, err error)
	GetByID(ctx context.Context, id int64) (*UsageLog, error)
	Delete(ctx context.Context, id int64) error

	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAPIKey(ctx context.Context, apiKeyID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAccount(ctx context.Context, accountID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error)

	ListByUserAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAPIKeyAndTimeRange(ctx context.Context, apiKeyID int64, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAccountAndTimeRange(ctx context.Context, accountID int64, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)
	ListByModelAndTimeRange(ctx context.Context, modelName string, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)

	GetAccountWindowStats(ctx context.Context, accountID int64, startTime time.Time) (*usagestats.AccountStats, error)
	GetAccountTodayStats(ctx context.Context, accountID int64) (*usagestats.AccountStats, error)

	// Admin dashboard stats
	GetDashboardStats(ctx context.Context) (*usagestats.DashboardStats, error)
	GetUsageTrendWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]usagestats.TrendDataPoint, error)
	GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8) ([]usagestats.ModelStat, error)
	GetEndpointStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]usagestats.EndpointStat, error)
	GetUpstreamEndpointStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]usagestats.EndpointStat, error)
	GetGroupStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8) ([]usagestats.GroupStat, error)
	GetUserBreakdownStats(ctx context.Context, startTime, endTime time.Time, dim usagestats.UserBreakdownDimension, limit int) ([]usagestats.UserBreakdownItem, error)
	GetAllGroupUsageSummary(ctx context.Context, todayStart time.Time) ([]usagestats.GroupUsageSummary, error)
	GetAPIKeyUsageTrend(ctx context.Context, startTime, endTime time.Time, granularity string, limit int) ([]usagestats.APIKeyUsageTrendPoint, error)
	GetUserUsageTrend(ctx context.Context, startTime, endTime time.Time, granularity string, limit int) ([]usagestats.UserUsageTrendPoint, error)
	GetUserSpendingRanking(ctx context.Context, startTime, endTime time.Time, limit int) (*usagestats.UserSpendingRankingResponse, error)
	GetBatchUserUsageStats(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchUserUsageStats, error)
	GetBatchAPIKeyUsageStats(ctx context.Context, apiKeyIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchAPIKeyUsageStats, error)

	// User dashboard stats
	GetUserDashboardStats(ctx context.Context, userID int64) (*usagestats.UserDashboardStats, error)
	GetAPIKeyDashboardStats(ctx context.Context, apiKeyID int64) (*usagestats.UserDashboardStats, error)
	GetUserUsageTrendByUserID(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string) ([]usagestats.TrendDataPoint, error)
	GetUserModelStats(ctx context.Context, userID int64, startTime, endTime time.Time) ([]usagestats.ModelStat, error)

	// Admin usage listing/stats
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]UsageLog, *pagination.PaginationResult, error)
	GetGlobalStats(ctx context.Context, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetStatsWithFilters(ctx context.Context, filters usagestats.UsageLogFilters) (*usagestats.UsageStats, error)

	// Account stats
	GetAccountUsageStats(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error)
	// GetAccountStatsCrossGroup 返回账号跨 group 全量聚合（Phase 1 P1-2）。
	// 与 GetAccountUsageStats 语义一致，但函数名显式表达"不被 group_id 切分"承诺。
	GetAccountStatsCrossGroup(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error)
	// GetStatsByProvider 返回按 provider_key 跨 group 聚合的用量统计（Phase 1 P1-2）。
	// provider 入参必须经 NormalizeProvider 规范化；详见 docs/generic-channel-design.md §5.4。
	GetStatsByProvider(ctx context.Context, providerKey string, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error)

	// User stats
	GetUserUsageStats(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error)

	// Aggregated stats (optimized)
	GetUserStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetAPIKeyStatsAggregated(ctx context.Context, apiKeyID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetAccountStatsAggregated(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetModelStatsAggregated(ctx context.Context, modelName string, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetDailyStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) ([]map[string]any, error)
}

type accountWindowStatsBatchReader interface {
	GetAccountWindowStatsBatch(ctx context.Context, accountIDs []int64, startTime time.Time) (map[int64]*usagestats.AccountStats, error)
}

// windowStatsCache 缓存从本地数据库查询的窗口统计（requests, tokens, cost）
type windowStatsCache struct {
	stats     *WindowStats
	timestamp time.Time
}

// geminiUsageCache 缓存 Gemini 额度数据。
// 额外记录 minuteStart / dayStart：命中时校验窗口边界未变，
// 避免分钟/日窗口跨界后仍返回上一窗口的 RPM/RPD（stale 误导）。
type geminiUsageCache struct {
	usageInfo   *UsageInfo
	timestamp   time.Time
	minuteStart time.Time
	dayStart    time.Time
}

const (
	apiCacheTTL             = 3 * time.Minute
	apiErrorCacheTTL        = 1 * time.Minute        // 负缓存 TTL：429 等错误缓存 1 分钟
	apiQueryMaxJitter       = 800 * time.Millisecond // 用量查询最大随机延迟
	windowStatsCacheTTL     = 1 * time.Minute
	geminiUsageCacheTTL     = 30 * time.Second // Gemini 用量短缓存 TTL（配合窗口边界校验）
	openAIProbeCacheTTL     = 10 * time.Minute
	openAICodexProbeVersion = "0.125.0"
)

// UsageCache 封装账户使用量相关的缓存
type UsageCache struct {
	windowStatsCache sync.Map // accountID -> *windowStatsCache
	geminiCache      sync.Map // accountID -> *geminiUsageCache
	openAIProbeCache sync.Map // accountID -> time.Time
}

// NewUsageCache 创建 UsageCache 实例
func NewUsageCache() *UsageCache {
	return &UsageCache{}
}

// WindowStats 窗口期统计
//
// cost: 账号口径费用（total_cost * account_rate_multiplier）
// standard_cost: 标准费用（total_cost，不含倍率）
// user_cost: 用户/API Key 口径费用（actual_cost，受分组倍率影响）
type WindowStats struct {
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	StandardCost float64 `json:"standard_cost"`
	UserCost     float64 `json:"user_cost"`
}

// UsageProgress 使用量进度
type UsageProgress struct {
	Utilization      float64      `json:"utilization"`            // 使用率百分比 (0-100+，100表示100%)
	ResetsAt         *time.Time   `json:"resets_at"`              // 重置时间
	RemainingSeconds int          `json:"remaining_seconds"`      // 距重置剩余秒数
	WindowStats      *WindowStats `json:"window_stats,omitempty"` // 窗口期统计（从窗口开始到当前的使用量）
	UsedRequests     int64        `json:"used_requests,omitempty"`
	LimitRequests    int64        `json:"limit_requests,omitempty"`
}

// UsageInfo 账号使用量信息
type UsageInfo struct {
	Source             string         `json:"source,omitempty"`               // "passive" or "active"
	UpdatedAt          *time.Time     `json:"updated_at,omitempty"`           // 更新时间
	FiveHour           *UsageProgress `json:"five_hour"`                      // 5小时窗口
	SevenDay           *UsageProgress `json:"seven_day,omitempty"`            // 7天窗口
	SevenDaySonnet     *UsageProgress `json:"seven_day_sonnet,omitempty"`     // 7天Sonnet窗口
	SevenDayFable      *UsageProgress `json:"seven_day_fable,omitempty"`      // 7天Fable窗口（响应头 7d_oi）
	GeminiSharedDaily  *UsageProgress `json:"gemini_shared_daily,omitempty"`  // Gemini shared pool RPD (Google One / Code Assist)
	GeminiProDaily     *UsageProgress `json:"gemini_pro_daily,omitempty"`     // Gemini Pro 日配额
	GeminiFlashDaily   *UsageProgress `json:"gemini_flash_daily,omitempty"`   // Gemini Flash 日配额
	GeminiSharedMinute *UsageProgress `json:"gemini_shared_minute,omitempty"` // Gemini shared pool RPM (Google One / Code Assist)
	GeminiProMinute    *UsageProgress `json:"gemini_pro_minute,omitempty"`    // Gemini Pro RPM
	GeminiFlashMinute  *UsageProgress `json:"gemini_flash_minute,omitempty"`  // Gemini Flash RPM

	// 错误码（机器可读）：unauthenticated / rate_limited / network_error
	ErrorCode string `json:"error_code,omitempty"`

	// 获取 usage 时的错误信息（降级返回，而非 500）
	Error string `json:"error,omitempty"`
}

// AccountUsageService 账号使用量查询服务
type AccountUsageService struct {
	accountRepo         AccountRepository
	usageLogRepo        UsageLogRepository
	geminiQuotaService  *GeminiQuotaService
	cache               *UsageCache
	identityCache       IdentityCache
	tlsFPProfileService *TLSFingerprintProfileService
}

// NewAccountUsageService 创建AccountUsageService实例
func NewAccountUsageService(
	accountRepo AccountRepository,
	usageLogRepo UsageLogRepository,
	geminiQuotaService *GeminiQuotaService,
	cache *UsageCache,
	identityCache IdentityCache,
	tlsFPProfileService *TLSFingerprintProfileService,
) *AccountUsageService {
	return &AccountUsageService{
		accountRepo:         accountRepo,
		usageLogRepo:        usageLogRepo,
		geminiQuotaService:  geminiQuotaService,
		cache:               cache,
		identityCache:       identityCache,
		tlsFPProfileService: tlsFPProfileService,
	}
}

// GetUsage 获取账号使用量
// OAuth账号: 调用Anthropic API获取真实数据（需要profile scope），API响应缓存10分钟，窗口统计缓存1分钟
// Setup Token账号: 根据session_window推算5h窗口，7d数据不可用（没有profile scope）
// API Key账号: 不支持usage查询
func (s *AccountUsageService) GetUsage(ctx context.Context, accountID int64, force ...bool) (*UsageInfo, error) {
	_ = force

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account failed: %w", err)
	}

	if account.Platform == PlatformGemini {
		usage, err := s.getGeminiUsage(ctx, account)
		if err == nil {
			s.tryClearRecoverableAccountError(ctx, account)
		}
		return usage, err
	}

	// API Key账号不支持usage查询
	return nil, fmt.Errorf("account type %s does not support usage query", account.Type)
}

// GetPassiveUsage 从 Account.Extra 中的被动采样数据构建 UsageInfo，不调用外部 API。
// 仅适用于 Anthropic OAuth / SetupToken 账号。
func (s *AccountUsageService) GetPassiveUsage(ctx context.Context, accountID int64) (*UsageInfo, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account failed: %w", err)
	}

	if !account.IsAnthropicOAuthOrSetupToken() {
		return nil, fmt.Errorf("passive usage only supported for Anthropic OAuth/SetupToken accounts")
	}

	// 复用 estimateSetupTokenUsage 构建 5h 窗口（OAuth 和 SetupToken 逻辑一致）
	info := s.estimateSetupTokenUsage(account)
	info.Source = "passive"

	// 设置采样时间
	if raw, ok := account.Extra["passive_usage_sampled_at"]; ok {
		if str, ok := raw.(string); ok {
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				info.UpdatedAt = &t
			}
		}
	}

	// 构建 7d 窗口（从被动采样数据）
	info.SevenDay = buildPassiveUsageWindow(account.Extra, "passive_usage_7d_utilization", "passive_usage_7d_reset")

	// 构建 7d Fable 窗口（从被动采样的 7d_oi 响应头数据）
	info.SevenDayFable = buildPassiveUsageWindow(account.Extra, "passive_usage_7d_oi_utilization", "passive_usage_7d_oi_reset")

	// 添加窗口统计
	s.addWindowStats(ctx, account, info)

	return info, nil
}

// buildPassiveUsageWindow 从 Extra 中的被动采样数据（utilization 为 0-1 小数、reset 为 Unix 秒）
// 构建用量窗口，无数据时返回 nil。
func buildPassiveUsageWindow(extra map[string]any, utilKey, resetKey string) *UsageProgress {
	util := parseExtraFloat64(extra[utilKey])
	resetRaw := parseExtraFloat64(extra[resetKey])
	if util <= 0 && resetRaw <= 0 {
		return nil
	}
	var resetAt *time.Time
	var remaining int
	if resetRaw > 0 {
		t := time.Unix(int64(resetRaw), 0)
		resetAt = &t
		remaining = int(time.Until(t).Seconds())
		if remaining < 0 {
			remaining = 0
		}
	}
	return &UsageProgress{
		Utilization:      util * 100,
		ResetsAt:         resetAt,
		RemainingSeconds: remaining,
	}
}

// syncActiveToPassive 将主动查询的最新数据回写到 Extra 被动缓存，
// 这样下次被动加载时能看到最新值。
func (s *AccountUsageService) syncActiveToPassive(ctx context.Context, accountID int64, usage *UsageInfo) {
	extraUpdates := make(map[string]any, 4)

	if usage.FiveHour != nil {
		extraUpdates["session_window_utilization"] = usage.FiveHour.Utilization / 100
	}
	if usage.SevenDay != nil {
		extraUpdates["passive_usage_7d_utilization"] = usage.SevenDay.Utilization / 100
		if usage.SevenDay.ResetsAt != nil {
			extraUpdates["passive_usage_7d_reset"] = usage.SevenDay.ResetsAt.Unix()
		}
	}
	if usage.SevenDayFable != nil {
		extraUpdates["passive_usage_7d_oi_utilization"] = usage.SevenDayFable.Utilization / 100
		if usage.SevenDayFable.ResetsAt != nil {
			extraUpdates["passive_usage_7d_oi_reset"] = usage.SevenDayFable.ResetsAt.Unix()
		}
	}

	if len(extraUpdates) > 0 {
		extraUpdates["passive_usage_sampled_at"] = time.Now().UTC().Format(time.RFC3339)
		if err := s.accountRepo.UpdateExtra(ctx, accountID, extraUpdates); err != nil {
			slog.Warn("sync_active_to_passive_failed", "account_id", accountID, "error", err)
		}
	}

	// 5h ResetsAt 必须回写到 SessionWindowEnd column，estimateSetupTokenUsage
	// 读这个字段作为窗口结束时间；只塞 Extra 会让 UI 一直拿到上个窗口的过期时间。
	if usage.FiveHour != nil && usage.FiveHour.ResetsAt != nil {
		if err := s.accountRepo.UpdateSessionWindowEnd(ctx, accountID, *usage.FiveHour.ResetsAt); err != nil {
			slog.Warn("sync_active_to_passive_session_window_end_failed", "account_id", accountID, "error", err)
		}
	}
}

func (s *AccountUsageService) getOpenAIUsage(ctx context.Context, account *Account, force bool) (*UsageInfo, error) {
	now := time.Now()
	usage := &UsageInfo{UpdatedAt: &now}

	if account == nil {
		return usage, nil
	}

	applyExtraToUsage(usage, account.Extra, now)

	if (force || shouldRefreshOpenAICodexSnapshot(account, usage, now)) && s.shouldProbeOpenAICodexSnapshot(account.ID, now, force) {
		if updates, err := s.probeOpenAICodexSnapshot(ctx, account); err == nil && len(updates) > 0 {
			mergeAccountExtra(account, updates)
			if usage.UpdatedAt == nil {
				usage.UpdatedAt = &now
			}
			applyExtraToUsage(usage, account.Extra, now)
		}
	}

	if s.usageLogRepo == nil {
		return usage, nil
	}

	if stats, err := s.usageLogRepo.GetAccountWindowStats(ctx, account.ID, codexWindowStatsStart(usage.FiveHour, 5*time.Hour, now)); err == nil {
		if usage.FiveHour == nil {
			usage.FiveHour = &UsageProgress{Utilization: 0}
		}
		usage.FiveHour.WindowStats = windowStatsFromAccountStats(stats)
	}

	if stats, err := s.usageLogRepo.GetAccountWindowStats(ctx, account.ID, codexWindowStatsStart(usage.SevenDay, 7*24*time.Hour, now)); err == nil {
		if usage.SevenDay == nil {
			usage.SevenDay = &UsageProgress{Utilization: 0}
		}
		usage.SevenDay.WindowStats = windowStatsFromAccountStats(stats)
	}

	return usage, nil
}

func shouldRefreshOpenAICodexSnapshot(account *Account, usage *UsageInfo, now time.Time) bool {
	if account == nil {
		return false
	}
	if usage == nil {
		return true
	}
	if usage.FiveHour == nil || usage.SevenDay == nil {
		return true
	}
	if account.IsRateLimited() {
		return true
	}
	return isOpenAICodexSnapshotStale(account, now)
}

func isOpenAICodexSnapshotStale(account *Account, now time.Time) bool {
	if account == nil || !account.IsOpenAIOAuth() || !account.IsOpenAIResponsesWebSocketV2Enabled() {
		return false
	}
	if account.Extra == nil {
		return true
	}
	raw, ok := account.Extra["codex_usage_updated_at"]
	if !ok {
		return true
	}
	ts, err := parseTime(fmt.Sprint(raw))
	if err != nil {
		return true
	}
	return now.Sub(ts) >= openAIProbeCacheTTL
}

func (s *AccountUsageService) shouldProbeOpenAICodexSnapshot(accountID int64, now time.Time, force ...bool) bool {
	if s == nil || s.cache == nil || accountID <= 0 {
		return true
	}
	forceProbe := len(force) > 0 && force[0]
	if !forceProbe {
		if cached, ok := s.cache.openAIProbeCache.Load(accountID); ok {
			if ts, ok := cached.(time.Time); ok && now.Sub(ts) < openAIProbeCacheTTL {
				return false
			}
		}
	}
	s.cache.openAIProbeCache.Store(accountID, now)
	return true
}

func (s *AccountUsageService) probeOpenAICodexSnapshot(ctx context.Context, account *Account) (map[string]any, error) {
	if account == nil {
		return nil, nil
	}
	accessToken := account.GetOpenAIAccessToken()
	if accessToken == "" {
		return nil, fmt.Errorf("no access token available")
	}
	modelID := openaipkg.DefaultTestModel
	payload := createOpenAITestPayload(modelID, true)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openai probe payload: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, chatgptCodexURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("create openai probe request: %w", err)
	}
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("Originator", "codex_cli_rs")
	req.Header.Set("Version", openAICodexProbeVersion)
	req.Header.Set("User-Agent", codexCLIUserAgent)
	if s.identityCache != nil {
		if fp, fpErr := s.identityCache.GetFingerprint(reqCtx, account.ID); fpErr == nil && fp != nil && strings.TrimSpace(fp.UserAgent) != "" {
			req.Header.Set("User-Agent", strings.TrimSpace(fp.UserAgent))
		}
	}
	if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
		req.Header.Set("chatgpt-account-id", chatgptAccountID)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	client, err := httppool.GetClient(httppool.Options{
		ProxyURL:              proxyURL,
		Timeout:               15 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("build openai probe client: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai codex probe request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	updates, err := extractOpenAICodexProbeUpdates(resp)
	if err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		s.persistOpenAICodexProbeSnapshot(account.ID, updates)
		return updates, nil
	}
	return nil, nil
}

func (s *AccountUsageService) persistOpenAICodexProbeSnapshot(accountID int64, updates map[string]any) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return
	}
	if len(updates) == 0 {
		return
	}

	go func() {
		updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer updateCancel()
		_ = s.accountRepo.UpdateExtra(updateCtx, accountID, updates)
	}()
}

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

func mergeAccountExtra(account *Account, updates map[string]any) {
	if account == nil || len(updates) == 0 {
		return
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any, len(updates))
	}
	for k, v := range updates {
		account.Extra[k] = v
	}
}

// applyExtraToUsage rebuilds the codex 5h/7d windows in usage from the
// account's Extra map.  Called after mergeAccountExtra to make the in-memory
// UsageInfo consistent with the just-persisted Extra values.
func applyExtraToUsage(usage *UsageInfo, extra map[string]any, now time.Time) {
	if usage == nil {
		return
	}
	if progress := buildCodexUsageProgressFromExtra(extra, "5h", now); progress != nil {
		usage.FiveHour = progress
	}
	if progress := buildCodexUsageProgressFromExtra(extra, "7d", now); progress != nil {
		usage.SevenDay = progress
	}
}

func (s *AccountUsageService) getGeminiUsage(ctx context.Context, account *Account) (*UsageInfo, error) {
	now := time.Now()
	usage := &UsageInfo{
		UpdatedAt: &now,
	}

	if s.geminiQuotaService == nil || s.usageLogRepo == nil {
		return usage, nil
	}

	quota, ok := s.geminiQuotaService.QuotaForAccount(ctx, account)
	if !ok {
		return usage, nil
	}

	dayStart := geminiDailyWindowStart(now)
	minuteStart := now.Truncate(time.Minute)

	// 短缓存命中：仅当分钟/日窗口边界均未变时复用，避免跨界返回上一窗口的 RPM/RPD。
	if cached, ok := s.cache.geminiCache.Load(account.ID); ok {
		if gc, ok := cached.(*geminiUsageCache); ok &&
			time.Since(gc.timestamp) < geminiUsageCacheTTL &&
			gc.minuteStart.Equal(minuteStart) &&
			gc.dayStart.Equal(dayStart) {
			return cloneGeminiUsage(gc.usageInfo), nil
		}
	}

	// daily / minute 两次聚合查询无依赖，errgroup 并行。
	var dayStats, minuteStats []usagestats.ModelStat
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		stats, err := s.usageLogRepo.GetModelStatsWithFilters(gctx, dayStart, now, 0, 0, account.ID, 0, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("get gemini usage stats failed: %w", err)
		}
		dayStats = stats
		return nil
	})
	g.Go(func() error {
		stats, err := s.usageLogRepo.GetModelStatsWithFilters(gctx, minuteStart, now, 0, 0, account.ID, 0, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("get gemini minute usage stats failed: %w", err)
		}
		minuteStats = stats
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	dayTotals := geminiAggregateUsage(dayStats)
	dailyResetAt := geminiDailyResetTime(now)

	// Daily window (RPD)
	if quota.SharedRPD > 0 {
		totalReq := dayTotals.ProRequests + dayTotals.FlashRequests
		totalTokens := dayTotals.ProTokens + dayTotals.FlashTokens
		totalCost := dayTotals.ProCost + dayTotals.FlashCost
		usage.GeminiSharedDaily = buildGeminiUsageProgress(totalReq, quota.SharedRPD, dailyResetAt, totalTokens, totalCost, now)
	} else {
		usage.GeminiProDaily = buildGeminiUsageProgress(dayTotals.ProRequests, quota.ProRPD, dailyResetAt, dayTotals.ProTokens, dayTotals.ProCost, now)
		usage.GeminiFlashDaily = buildGeminiUsageProgress(dayTotals.FlashRequests, quota.FlashRPD, dailyResetAt, dayTotals.FlashTokens, dayTotals.FlashCost, now)
	}

	// Minute window (RPM) - fixed-window approximation: current minute [truncate(now), truncate(now)+1m)
	minuteResetAt := minuteStart.Add(time.Minute)
	minuteTotals := geminiAggregateUsage(minuteStats)

	if quota.SharedRPM > 0 {
		totalReq := minuteTotals.ProRequests + minuteTotals.FlashRequests
		totalTokens := minuteTotals.ProTokens + minuteTotals.FlashTokens
		totalCost := minuteTotals.ProCost + minuteTotals.FlashCost
		usage.GeminiSharedMinute = buildGeminiUsageProgress(totalReq, quota.SharedRPM, minuteResetAt, totalTokens, totalCost, now)
	} else {
		usage.GeminiProMinute = buildGeminiUsageProgress(minuteTotals.ProRequests, quota.ProRPM, minuteResetAt, minuteTotals.ProTokens, minuteTotals.ProCost, now)
		usage.GeminiFlashMinute = buildGeminiUsageProgress(minuteTotals.FlashRequests, quota.FlashRPM, minuteResetAt, minuteTotals.FlashTokens, minuteTotals.FlashCost, now)
	}

	s.cache.geminiCache.Store(account.ID, &geminiUsageCache{
		usageInfo:   usage,
		timestamp:   now,
		minuteStart: minuteStart,
		dayStart:    dayStart,
	})

	return cloneGeminiUsage(usage), nil
}

// cloneGeminiUsage 返回 info 的顶层浅拷贝。
// Gemini 各窗口 progress 字段在缓存后不再被原地修改，故顶层浅拷贝即可避免
// 并发请求共享同一 *UsageInfo 导致的写竞争。
func cloneGeminiUsage(info *UsageInfo) *UsageInfo {
	if info == nil {
		return nil
	}
	cp := *info
	return &cp
}

// addWindowStats 为 usage 数据添加窗口期统计
// 使用独立缓存（1 分钟），与 API 缓存分离
func (s *AccountUsageService) addWindowStats(ctx context.Context, account *Account, usage *UsageInfo) {
	// 修复：即使 FiveHour 为 nil，也要尝试获取统计数据
	// 因为 SevenDay/SevenDaySonnet/SevenDayFable 可能需要
	if usage.FiveHour == nil && usage.SevenDay == nil && usage.SevenDaySonnet == nil && usage.SevenDayFable == nil {
		return
	}

	// 检查窗口统计缓存（1 分钟）
	var windowStats *WindowStats
	if cached, ok := s.cache.windowStatsCache.Load(account.ID); ok {
		if cache, ok := cached.(*windowStatsCache); ok && time.Since(cache.timestamp) < windowStatsCacheTTL {
			windowStats = cache.stats
		}
	}

	// 如果没有缓存，从数据库查询
	if windowStats == nil {
		// 使用统一的窗口开始时间计算逻辑（考虑窗口过期情况）
		startTime := account.GetCurrentWindowStartTime()

		stats, err := s.usageLogRepo.GetAccountWindowStats(ctx, account.ID, startTime)
		if err != nil {
			log.Printf("Failed to get window stats for account %d: %v", account.ID, err)
			return
		}

		windowStats = &WindowStats{
			Requests:     stats.Requests,
			Tokens:       stats.Tokens,
			Cost:         stats.Cost,
			StandardCost: stats.StandardCost,
			UserCost:     stats.UserCost,
		}

		// 缓存窗口统计（1 分钟）
		s.cache.windowStatsCache.Store(account.ID, &windowStatsCache{
			stats:     windowStats,
			timestamp: time.Now(),
		})
	}

	// 为 FiveHour 添加 WindowStats（5h 窗口统计）
	if usage.FiveHour != nil {
		usage.FiveHour.WindowStats = windowStats
	}
}

// GetTodayStats 获取账号今日统计
func (s *AccountUsageService) GetTodayStats(ctx context.Context, accountID int64) (*WindowStats, error) {
	stats, err := s.usageLogRepo.GetAccountTodayStats(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get today stats failed: %w", err)
	}

	return &WindowStats{
		Requests:     stats.Requests,
		Tokens:       stats.Tokens,
		Cost:         stats.Cost,
		StandardCost: stats.StandardCost,
		UserCost:     stats.UserCost,
	}, nil
}

// GetTodayStatsBatch 批量获取账号今日统计，优先走批量 SQL，失败时回退单账号查询。
func (s *AccountUsageService) GetTodayStatsBatch(ctx context.Context, accountIDs []int64) (map[int64]*WindowStats, error) {
	uniqueIDs := make([]int64, 0, len(accountIDs))
	seen := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		uniqueIDs = append(uniqueIDs, accountID)
	}

	result := make(map[int64]*WindowStats, len(uniqueIDs))
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	startTime := timezone.Today()
	if batchReader, ok := s.usageLogRepo.(accountWindowStatsBatchReader); ok {
		statsByAccount, err := batchReader.GetAccountWindowStatsBatch(ctx, uniqueIDs, startTime)
		if err == nil {
			for _, accountID := range uniqueIDs {
				result[accountID] = windowStatsFromAccountStats(statsByAccount[accountID])
			}
			return result, nil
		}
	}

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)

	for _, accountID := range uniqueIDs {
		id := accountID
		g.Go(func() error {
			stats, err := s.usageLogRepo.GetAccountWindowStats(gctx, id, startTime)
			if err != nil {
				return nil
			}
			mu.Lock()
			result[id] = windowStatsFromAccountStats(stats)
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()

	for _, accountID := range uniqueIDs {
		if _, ok := result[accountID]; !ok {
			result[accountID] = &WindowStats{}
		}
	}
	return result, nil
}

func windowStatsFromAccountStats(stats *usagestats.AccountStats) *WindowStats {
	if stats == nil {
		return &WindowStats{}
	}
	return &WindowStats{
		Requests:     stats.Requests,
		Tokens:       stats.Tokens,
		Cost:         stats.Cost,
		StandardCost: stats.StandardCost,
		UserCost:     stats.UserCost,
	}
}

func buildCodexUsageProgressFromExtra(extra map[string]any, window string, now time.Time) *UsageProgress {
	if len(extra) == 0 {
		return nil
	}

	var (
		usedPercentKey string
		resetAfterKey  string
		resetAtKey     string
	)

	switch window {
	case "5h":
		usedPercentKey = "codex_5h_used_percent"
		resetAfterKey = "codex_5h_reset_after_seconds"
		resetAtKey = "codex_5h_reset_at"
	case "7d":
		usedPercentKey = "codex_7d_used_percent"
		resetAfterKey = "codex_7d_reset_after_seconds"
		resetAtKey = "codex_7d_reset_at"
	default:
		return nil
	}

	usedRaw, ok := extra[usedPercentKey]
	if !ok {
		return nil
	}

	progress := &UsageProgress{Utilization: parseExtraFloat64(usedRaw)}
	if resetAtRaw, ok := extra[resetAtKey]; ok {
		if resetAt, err := parseTime(fmt.Sprint(resetAtRaw)); err == nil {
			progress.ResetsAt = &resetAt
			progress.RemainingSeconds = int(time.Until(resetAt).Seconds())
			if progress.RemainingSeconds < 0 {
				progress.RemainingSeconds = 0
			}
		}
	}
	if progress.ResetsAt == nil {
		if resetAfterSeconds := parseExtraInt(extra[resetAfterKey]); resetAfterSeconds > 0 {
			base := now
			if updatedAtRaw, ok := extra["codex_usage_updated_at"]; ok {
				if updatedAt, err := parseTime(fmt.Sprint(updatedAtRaw)); err == nil {
					base = updatedAt
				}
			}
			resetAt := base.Add(time.Duration(resetAfterSeconds) * time.Second)
			progress.ResetsAt = &resetAt
			progress.RemainingSeconds = int(time.Until(resetAt).Seconds())
			if progress.RemainingSeconds < 0 {
				progress.RemainingSeconds = 0
			}
		}
	}

	// 窗口已过期（resetAt 在 now 之前）→ 额度已重置，归零
	if progress.ResetsAt != nil && !now.Before(*progress.ResetsAt) {
		progress.Utilization = 0
	}

	return progress
}

func codexWindowStatsStart(progress *UsageProgress, fallbackWindow time.Duration, now time.Time) time.Time {
	if progress != nil && progress.ResetsAt != nil && now.Before(*progress.ResetsAt) {
		return progress.ResetsAt.Add(-fallbackWindow)
	}
	return now.Add(-fallbackWindow)
}

func (s *AccountUsageService) GetAccountUsageStats(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	stats, err := s.usageLogRepo.GetAccountUsageStats(ctx, accountID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get account usage stats failed: %w", err)
	}
	return stats, nil
}

func (s *AccountUsageService) GetUserUsageStats(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	stats, err := s.usageLogRepo.GetUserUsageStats(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get user usage stats failed: %w", err)
	}
	return stats, nil
}

// GetAccountStatsCrossGroup 返回账号跨 group 全量聚合（Phase 1 P1-2 / P1-4）。
//
// 与 GetAccountUsageStats 行为一致——账号挂多 group 时跨 group 求和；
// 函数名显式表达"不被 group_id 切分"承诺。详见 docs/generic-channel-design.md §5.4。
func (s *AccountUsageService) GetAccountStatsCrossGroup(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	stats, err := s.usageLogRepo.GetAccountStatsCrossGroup(ctx, accountID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get account stats cross group failed: %w", err)
	}
	return stats, nil
}

// GetStatsByProvider 返回按 provider_key 跨 group 聚合的用量统计（Phase 1 P1-2 / P1-4）。
//
// providerKey 必须是 NormalizeProvider 规范化后的值；为空返回 nil。
// 调用栈不出现 group_id 强过滤；详见 docs/generic-channel-design.md §5.4 第 2 条。
func (s *AccountUsageService) GetStatsByProvider(ctx context.Context, providerKey string, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	providerKey = NormalizeProvider(providerKey)
	if providerKey == "" {
		return nil, fmt.Errorf("provider key is required")
	}
	stats, err := s.usageLogRepo.GetStatsByProvider(ctx, providerKey, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get stats by provider failed: %w", err)
	}
	return stats, nil
}

// parseTime 尝试多种格式解析时间
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

func (s *AccountUsageService) tryClearRecoverableAccountError(ctx context.Context, account *Account) {
	if account == nil || account.Status != StatusError {
		return
	}

	msg := strings.ToLower(strings.TrimSpace(account.ErrorMessage))
	if msg == "" {
		return
	}

	if !strings.Contains(msg, "token refresh failed") &&
		!strings.Contains(msg, "invalid_client") &&
		!strings.Contains(msg, "missing_project_id") &&
		!strings.Contains(msg, "unauthenticated") {
		return
	}

	if err := s.accountRepo.ClearError(ctx, account.ID); err != nil {
		log.Printf("[usage] failed to clear recoverable account error for account %d: %v", account.ID, err)
		return
	}

	account.Status = StatusActive
	account.ErrorMessage = ""
}

// buildUsageInfo 构建UsageInfo
// estimateSetupTokenUsage 根据session_window推算Setup Token账号的使用量
func (s *AccountUsageService) estimateSetupTokenUsage(account *Account) *UsageInfo {
	info := &UsageInfo{}

	// 如果有session_window信息
	if account.SessionWindowEnd != nil {
		remaining := int(time.Until(*account.SessionWindowEnd).Seconds())
		if remaining < 0 {
			remaining = 0
		}

		// 优先使用响应头中存储的真实 utilization 值（0-1 小数，转为 0-100 百分比）
		var utilization float64
		var found bool
		if stored, ok := account.Extra["session_window_utilization"]; ok {
			switch v := stored.(type) {
			case float64:
				utilization = v * 100
				found = true
			case json.Number:
				if f, err := v.Float64(); err == nil {
					utilization = f * 100
					found = true
				}
			}
		}

		// 如果没有存储的 utilization，回退到状态估算
		if !found {
			switch account.SessionWindowStatus {
			case "rejected":
				utilization = 100.0
			case "allowed_warning":
				utilization = 80.0
			}
		}

		info.FiveHour = &UsageProgress{
			Utilization:      utilization,
			ResetsAt:         account.SessionWindowEnd,
			RemainingSeconds: remaining,
		}

		// 窗口已过期（resetAt 在 now 之前）→ 额度已重置，归零；
		// 与 Codex 分支 buildCodexUsageProgressFromExtra 保持一致，避免
		// UI 在 active poll 没回写 SessionWindowEnd 时渲染矛盾状态。
		if info.FiveHour.ResetsAt != nil && !time.Now().Before(*info.FiveHour.ResetsAt) {
			info.FiveHour.Utilization = 0
			info.FiveHour.ResetsAt = nil
			info.FiveHour.RemainingSeconds = 0
		}
	} else {
		// 没有窗口信息，返回空数据
		info.FiveHour = &UsageProgress{
			Utilization:      0,
			RemainingSeconds: 0,
		}
	}

	// Setup Token无法获取7d数据
	return info
}

func buildGeminiUsageProgress(used, limit int64, resetAt time.Time, tokens int64, cost float64, now time.Time) *UsageProgress {
	// limit <= 0 means "no local quota window" (unknown or unlimited).
	if limit <= 0 {
		return nil
	}
	utilization := (float64(used) / float64(limit)) * 100
	remainingSeconds := int(resetAt.Sub(now).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	resetCopy := resetAt
	return &UsageProgress{
		Utilization:      utilization,
		ResetsAt:         &resetCopy,
		RemainingSeconds: remainingSeconds,
		UsedRequests:     used,
		LimitRequests:    limit,
		WindowStats: &WindowStats{
			Requests: used,
			Tokens:   tokens,
			Cost:     cost,
		},
	}
}

// GetAccountWindowStats 获取账号在指定时间窗口内的使用统计
// 用于账号列表页面显示当前窗口费用
func (s *AccountUsageService) GetAccountWindowStats(ctx context.Context, accountID int64, startTime time.Time) (*usagestats.AccountStats, error) {
	return s.usageLogRepo.GetAccountWindowStats(ctx, accountID, startTime)
}
