package handler

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UsageHandler handles usage-related requests
type UsageHandler struct {
	usageService   *service.UsageService
	apiKeyService  *service.APIKeyService
	pricingService *service.PricingService
	testResultRepo service.ScheduledTestResultRepository
	groupRepo      service.GroupRepository
	accountRepo    service.AccountRepository
	endpointRepo   service.EndpointRepository // 功能 25：generic 账号端点查询
}

// NewUsageHandler creates a new UsageHandler
func NewUsageHandler(
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	pricingService *service.PricingService,
	testResultRepo service.ScheduledTestResultRepository,
	groupRepo service.GroupRepository,
	accountRepo service.AccountRepository,
	endpointRepo service.EndpointRepository,
) *UsageHandler {
	return &UsageHandler{
		usageService:   usageService,
		apiKeyService:  apiKeyService,
		pricingService: pricingService,
		testResultRepo: testResultRepo,
		groupRepo:      groupRepo,
		accountRepo:    accountRepo,
		endpointRepo:   endpointRepo,
	}
}

// List handles listing usage records with pagination
// GET /api/v1/usage
func (h *UsageHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)

	var apiKeyID int64
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}

		// [Security Fix] Verify API Key ownership to prevent horizontal privilege escalation
		apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if apiKey.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to access this API key's usage records")
			return
		}

		apiKeyID = id
	}

	// Parse additional filters
	model := c.Query("model")

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
		bt := int8(val)
		billingType = &bt
	}

	// Parse date range
	var startTime, endTime *time.Time
	userTZ := c.Query("timezone") // Get user's timezone from request
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		startTime = &t
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// Use half-open range [start, end), move to next calendar day start (DST-safe).
		t = t.AddDate(0, 0, 1)
		endTime = &t
	}

	billRequestID := strings.TrimSpace(c.Query("bill_request_id"))

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filters := usagestats.UsageLogFilters{
		UserID:        subject.UserID, // Always filter by current user for security
		APIKeyID:      apiKeyID,
		Model:         model,
		RequestType:   requestType,
		Stream:        stream,
		BillingType:   billingType,
		StartTime:     startTime,
		EndTime:       endTime,
		BillRequestID: billRequestID,
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.UsageLog, 0, len(records))
	for i := range records {
		out = append(out, *dto.UsageLogFromService(&records[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// GetByID handles getting a single usage record
// GET /api/v1/usage/:id
func (h *UsageHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	usageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid usage ID")
		return
	}

	record, err := h.usageService.GetByID(c.Request.Context(), usageID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 验证所有权
	if record.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this record")
		return
	}

	response.Success(c, dto.UsageLogFromService(record))
}

// Stats handles getting usage statistics
// GET /api/v1/usage/stats
func (h *UsageHandler) Stats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var apiKeyID int64
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}

		// [Security Fix] Verify API Key ownership to prevent horizontal privilege escalation
		apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.NotFound(c, "API key not found")
			return
		}
		if apiKey.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to access this API key's statistics")
			return
		}

		apiKeyID = id
	}

	// 获取时间范围参数
	userTZ := c.Query("timezone") // Get user's timezone from request
	now := timezone.NowInUserLocation(userTZ)
	var startTime, endTime time.Time

	// 优先使用 start_date 和 end_date 参数
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr != "" && endDateStr != "" {
		// 使用自定义日期范围
		var err error
		startTime, err = timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		endTime, err = timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// 与 SQL 条件 created_at < end 对齐，使用次日 00:00 作为上边界（DST-safe）。
		endTime = endTime.AddDate(0, 0, 1)
	} else {
		// 使用 period 参数
		period := c.DefaultQuery("period", "today")
		switch period {
		case "today":
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		case "week":
			startTime = now.AddDate(0, 0, -7)
		case "month":
			startTime = now.AddDate(0, -1, 0)
		default:
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		}
		endTime = now
	}

	var stats *service.UsageStats
	var err error
	if apiKeyID > 0 {
		stats, err = h.usageService.GetStatsByAPIKey(c.Request.Context(), apiKeyID, startTime, endTime)
	} else {
		stats, err = h.usageService.GetStatsByUser(c.Request.Context(), subject.UserID, startTime, endTime)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// parseUserTimeRange parses start_date, end_date query parameters for user dashboard
// Uses user's timezone if provided, otherwise falls back to server timezone
func parseUserTimeRange(c *gin.Context) (time.Time, time.Time) {
	userTZ := c.Query("timezone") // Get user's timezone from request
	now := timezone.NowInUserLocation(userTZ)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var startTime, endTime time.Time

	if startDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ); err == nil {
			startTime = t
		} else {
			startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
		}
	} else {
		startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
	}

	if endDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ); err == nil {
			endTime = t.Add(24 * time.Hour) // Include the end date
		} else {
			endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
		}
	} else {
		endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	}

	return startTime, endTime
}

const (
	defaultAPIKeyDailyUsageDays = 30
	maxAPIKeyDailyUsageDays     = 90
)

func parseAPIKeyDailyUsageDays(raw string) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return defaultAPIKeyDailyUsageDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 || days > maxAPIKeyDailyUsageDays {
		return 0, false
	}
	return days, true
}

func apiKeyDailyUsageRange(days int, userTZ string) (time.Time, time.Time) {
	now := timezone.NowInUserLocation(userTZ)
	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -(days-1)), userTZ)
	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	return startTime, endTime
}

// DashboardStats handles getting user dashboard statistics
// GET /api/v1/usage/dashboard/stats
func (h *UsageHandler) DashboardStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	stats, err := h.usageService.GetUserDashboardStats(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// DashboardTrend handles getting user usage trend data
// GET /api/v1/usage/dashboard/trend
func (h *UsageHandler) DashboardTrend(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	startTime, endTime := parseUserTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")

	trend, err := h.usageService.GetUserUsageTrendByUserID(c.Request.Context(), subject.UserID, startTime, endTime, granularity)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// DashboardModels handles getting user model usage statistics
// GET /api/v1/usage/dashboard/models
func (h *UsageHandler) DashboardModels(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	startTime, endTime := parseUserTimeRange(c)

	stats, err := h.usageService.GetUserModelStats(c.Request.Context(), subject.UserID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"models":     stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// BatchAPIKeysUsageRequest represents the request for batch API keys usage
type BatchAPIKeysUsageRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
}

// DashboardAPIKeysUsage handles getting usage stats for user's own API keys
// POST /api/v1/usage/dashboard/api-keys-usage
func (h *UsageHandler) DashboardAPIKeysUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req BatchAPIKeysUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.APIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	// Limit the number of API key IDs to prevent SQL parameter overflow
	if len(req.APIKeyIDs) > 100 {
		response.BadRequest(c, "Too many API key IDs (maximum 100 allowed)")
		return
	}

	validAPIKeyIDs, err := h.apiKeyService.VerifyOwnership(c.Request.Context(), subject.UserID, req.APIKeyIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if len(validAPIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	stats, err := h.usageService.GetBatchAPIKeyUsageStats(c.Request.Context(), validAPIKeyIDs, time.Time{}, time.Time{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"stats": stats})
}

// GetMyAPIKeyDailyUsage handles getting daily usage details for the current user's API key.
// GET /api/v1/user/api-keys/:id/usage/daily?days=30
func (h *UsageHandler) GetMyAPIKeyDailyUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	days, ok := parseAPIKeyDailyUsageDays(c.DefaultQuery("days", ""))
	if !ok {
		response.BadRequest(c, "Invalid days, allowed range is 1-90")
		return
	}

	if h.apiKeyService == nil {
		response.InternalError(c, "API key service is not configured")
		return
	}

	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if apiKey.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this API key's usage")
		return
	}

	userTZ := c.Query("timezone")
	startTime, endTime := apiKeyDailyUsageRange(days, userTZ)
	items, err := h.usageService.GetAPIKeyDailyUsage(c.Request.Context(), subject.UserID, apiKeyID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":      items,
		"days":       days,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.AddDate(0, 0, -1).Format("2006-01-02"),
	})
}

// ListModels returns all available models with pricing info for the current user
// GET /api/v1/models
func (h *UsageHandler) ListModels(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	if h.pricingService == nil {
		response.Success(c, gin.H{"models": []any{}, "total": 0, "available_platforms": []string{}})
		return
	}

	// Get user's available groups to determine which platforms/models are accessible
	availablePlatforms := make(map[string]bool)
	var groupIDs []int64
	if h.apiKeyService != nil {
		groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
		if err == nil {
			for _, g := range groups {
				if g.Status == service.StatusActive {
					availablePlatforms[g.Platform] = true
					groupIDs = append(groupIDs, g.ID)
				}
			}
		}
	}

	accountIDs := accountIDsFromGroups(h.groupRepo, c, groupIDs)

	// Get test results for user's available accounts
	testStatusMap := make(map[string]*service.ModelTestStatus)
	if h.testResultRepo != nil && len(accountIDs) > 0 {
		testStatusMap, _ = h.testResultRepo.GetLatestResultsByAccountIDs(c.Request.Context(), accountIDs)
	}

	models := h.pricingService.ListAllModels()
	modelsByID := make(map[string]service.ModelInfo, len(models))
	for _, m := range models {
		modelsByID[m.ID] = m
	}

	allowedModels := h.collectWhitelistedModelsForAccounts(c.Request.Context(), accountIDs, modelsByID)

	// Enrich each model with availability and test status
	type ModelWithAvailability struct {
		service.ModelInfo
		IsAvailable bool                     `json:"is_available"`
		TestStatus  *service.ModelTestStatus `json:"test_status,omitempty"`
	}
	enrichedModels := make([]ModelWithAvailability, 0, len(allowedModels))
	for _, m := range allowedModels {
		enrichedModels = append(enrichedModels, ModelWithAvailability{
			ModelInfo:   m,
			IsAvailable: true,
			TestStatus:  testStatusMap[m.ID],
		})
	}

	// Convert availablePlatforms map to slice for response
	platformSlice := make([]string, 0, len(availablePlatforms))
	for p := range availablePlatforms {
		platformSlice = append(platformSlice, p)
	}

	response.Success(c, gin.H{
		"models":              enrichedModels,
		"total":               len(enrichedModels),
		"available_platforms": platformSlice,
		"cny_rate":            h.pricingService.GetCNYRate(),
		"currency_mode":       h.pricingService.GetCurrencyMode(),
	})
}

func accountIDsFromGroups(groupRepo service.GroupRepository, c *gin.Context, groupIDs []int64) []int64 {
	if groupRepo == nil || len(groupIDs) == 0 {
		return nil
	}
	accountIDs, err := groupRepo.GetAccountIDsByGroupIDs(c.Request.Context(), groupIDs)
	if err != nil {
		return nil
	}
	return accountIDs
}

func (h *UsageHandler) collectWhitelistedModelsForAccounts(ctx context.Context, accountIDs []int64, modelsByID map[string]service.ModelInfo) []service.ModelInfo {
	if h.accountRepo == nil || len(accountIDs) == 0 || len(modelsByID) == 0 {
		return []service.ModelInfo{}
	}

	accounts, err := h.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil || len(accounts) == 0 {
		return []service.ModelInfo{}
	}

	allowed := make(map[string]service.ModelInfo)
	for _, account := range accounts {
		if account == nil || !account.IsActive() {
			continue
		}
		// 功能 25：generic 账号取各 endpoint 的 supported_models 并集（identity 映射）。
		// PR-7：若某个 endpoint 的 supported_models 为空，则以 catalog 中所有 is_enabled 模型兜底，
		// 避免新配置的 generic endpoint 因未显式配置白名单而导致模型广场显示为空。
		if account.Platform == service.PlatformGeneric && h.endpointRepo != nil {
			eps, _ := h.endpointRepo.ListByAccountID(ctx, account.ID)
			for _, ep := range eps {
				if len(ep.SupportedModels) == 0 {
					// 空白名单 → 显示全部 catalog 启用模型
					if h.pricingService != nil {
						for _, m := range h.pricingService.ListEnabledCatalogModels() {
							allowed[m.ID] = m
						}
					}
				} else {
					for _, m := range ep.SupportedModels {
						m = strings.TrimSpace(m)
						if m == "" {
							continue
						}
						addWhitelistedModel(allowed, modelsByID, m, m)
					}
				}
			}
			continue
		}
		for modelID, mappedModelID := range configuredModelWhitelist(account) {
			addWhitelistedModel(allowed, modelsByID, modelID, mappedModelID)
		}
	}

	out := make([]service.ModelInfo, 0, len(allowed))
	for _, model := range allowed {
		out = append(out, model)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LiteLLMProvider != out[j].LiteLLMProvider {
			return out[i].LiteLLMProvider < out[j].LiteLLMProvider
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func configuredModelWhitelist(account *service.Account) map[string]string {
	if account == nil || account.Credentials == nil {
		return nil
	}

	switch raw := account.Credentials["model_mapping"].(type) {
	case map[string]any:
		return cleanModelMapping(raw)
	case map[string]string:
		result := make(map[string]string, len(raw))
		for key, value := range raw {
			if key = strings.TrimSpace(key); key != "" {
				result[key] = strings.TrimSpace(value)
			}
		}
		return result
	default:
		return nil
	}
}

func cleanModelMapping(raw map[string]any) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]string, len(raw))
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if s, ok := value.(string); ok {
			result[key] = strings.TrimSpace(s)
		}
	}
	return result
}

func addWhitelistedModel(allowed map[string]service.ModelInfo, modelsByID map[string]service.ModelInfo, modelID, mappedModelID string) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return
	}

	if strings.HasSuffix(modelID, "*") {
		for candidateID, candidate := range modelsByID {
			if matchModelWhitelistPattern(modelID, candidateID) {
				allowed[candidateID] = candidate
			}
		}
		return
	}

	if model, ok := modelsByID[modelID]; ok {
		allowed[modelID] = model
		return
	}

	mappedModelID = strings.TrimSpace(mappedModelID)
	if mappedModelID == "" {
		return
	}
	if model, ok := modelsByID[mappedModelID]; ok {
		model.ID = modelID
		allowed[modelID] = model
	}
}

func matchModelWhitelistPattern(pattern, modelID string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(modelID, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == modelID
}
