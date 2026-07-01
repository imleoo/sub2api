package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPricingHandler handles admin model pricing management.
type ModelPricingHandler struct {
	modelPricingService *service.ModelPricingService
	repo                service.ModelPricingRepository
	pricingService      *service.PricingService
	settingService      *service.SettingService
	accountTestService  *service.AccountTestService
	modelRouting        *service.ModelRoutingService // 可路由模型计算（与模型广场同口径）
}

// NewModelPricingHandler creates a new ModelPricingHandler.
func NewModelPricingHandler(modelPricingService *service.ModelPricingService, repo service.ModelPricingRepository, ps *service.PricingService, ss *service.SettingService, ats *service.AccountTestService, mr *service.ModelRoutingService) *ModelPricingHandler {
	return &ModelPricingHandler{modelPricingService: modelPricingService, repo: repo, pricingService: ps, settingService: ss, accountTestService: ats, modelRouting: mr}
}

// listModelPricingResponse is the JSON shape returned per record.
type listModelPricingResponse struct {
	ID                          int64    `json:"id"`
	ModelID                     string   `json:"model_id"`
	DisplayName                 *string  `json:"display_name,omitempty"`
	Description                 *string  `json:"description,omitempty"`
	Provider                    string   `json:"provider"`
	Mode                        string   `json:"mode"`
	PricingUnit                 string   `json:"pricing_unit"`
	InputCostPerToken           *float64 `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken          *float64 `json:"output_cost_per_token,omitempty"`
	CacheCreationInputTokenCost *float64 `json:"cache_creation_input_token_cost,omitempty"`
	CacheReadInputTokenCost     *float64 `json:"cache_read_input_token_cost,omitempty"`
	OutputCostPerImage          *float64 `json:"output_cost_per_image,omitempty"`
	OutputCostPerImageToken     *float64 `json:"output_cost_per_image_token,omitempty"`
	SupportsPromptCaching       bool     `json:"supports_prompt_caching"`
	CustomInputCost             *float64 `json:"custom_input_cost,omitempty"`
	CustomOutputCost            *float64 `json:"custom_output_cost,omitempty"`
	DiscountRate                *float64 `json:"discount_rate,omitempty"`
	IsCustom                    bool     `json:"is_custom"`
	IsEnabled                   bool     `json:"is_enabled"`
	PricingHealth               string   `json:"pricing_health"`
	CreatedAt                   int64    `json:"created_at"`
	UpdatedAt                   int64    `json:"updated_at"`
}

// pricing_health 派生状态（mode-aware），供后台置顶高亮异常行。
const (
	pricingHealthOK               = "ok"
	pricingHealthMissingPricing   = "missing_pricing"
	pricingHealthUnitContaminated = "unit_contamination"
	pricingHealthOrphan           = "orphan_no_active_account"
)

func nilOrZero(v *float64) bool { return v == nil || *v == 0 }

// computePricingHealth 计算单行健康度。routable 为「active 账号可路由」的 model_id 集合（一次性物化）。
// 优先级：单位污染 > 未配价 > 孤儿 > ok。
func computePricingHealth(m *service.DBModelPricing, routable map[string]struct{}) string {
	if m == nil {
		return pricingHealthOK
	}
	isVisual := m.Mode == "image_generation" || m.Mode == "video_generation"
	// 单位污染：非图片/视频模型却带扁平图片价
	if !isVisual && m.OutputCostPerImage != nil {
		return pricingHealthUnitContaminated
	}
	// 未配价（mode-aware）
	var missing bool
	if isVisual {
		missing = nilOrZero(m.OutputCostPerImage) && nilOrZero(m.OutputCostPerImageToken) && nilOrZero(m.CustomOutputCost)
	} else {
		missing = nilOrZero(m.InputCostPerToken) && nilOrZero(m.OutputCostPerToken) && nilOrZero(m.CustomInputCost) && nilOrZero(m.CustomOutputCost)
	}
	if missing {
		return pricingHealthMissingPricing
	}
	// 孤儿：启用但无任何 active 账号可路由
	if m.IsEnabled {
		if _, ok := routable[m.ModelID]; !ok {
			return pricingHealthOrphan
		}
	}
	return pricingHealthOK
}

type createModelPricingRequest struct {
	ModelID                     string   `json:"model_id" binding:"required"`
	DisplayName                 *string  `json:"display_name"`
	Description                 *string  `json:"description"`
	Provider                    string   `json:"provider"`
	Mode                        string   `json:"mode"`
	PricingUnit                 string   `json:"pricing_unit"`
	InputCostPerToken           *float64 `json:"input_cost_per_token"`
	OutputCostPerToken          *float64 `json:"output_cost_per_token"`
	CacheCreationInputTokenCost *float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     *float64 `json:"cache_read_input_token_cost"`
	OutputCostPerImage          *float64 `json:"output_cost_per_image"`
	OutputCostPerImageToken     *float64 `json:"output_cost_per_image_token"`
	SupportsPromptCaching       bool     `json:"supports_prompt_caching"`
	CustomInputCost             *float64 `json:"custom_input_cost"`
	CustomOutputCost            *float64 `json:"custom_output_cost"`
	DiscountRate                *float64 `json:"discount_rate"`
	IsEnabled                   *bool    `json:"is_enabled"`
}

type updateModelPricingRequest struct {
	DisplayName                 *string  `json:"display_name"`
	Description                 *string  `json:"description"`
	Provider                    *string  `json:"provider"`
	Mode                        *string  `json:"mode"`
	PricingUnit                 *string  `json:"pricing_unit"`
	InputCostPerToken           *float64 `json:"input_cost_per_token"`
	OutputCostPerToken          *float64 `json:"output_cost_per_token"`
	CacheCreationInputTokenCost *float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     *float64 `json:"cache_read_input_token_cost"`
	OutputCostPerImage          *float64 `json:"output_cost_per_image"`
	OutputCostPerImageToken     *float64 `json:"output_cost_per_image_token"`
	SupportsPromptCaching       *bool    `json:"supports_prompt_caching"`
	CustomInputCost             *float64 `json:"custom_input_cost"`
	CustomOutputCost            *float64 `json:"custom_output_cost"`
	DiscountRate                *float64 `json:"discount_rate"`
	IsEnabled                   *bool    `json:"is_enabled"`
}

func dbModelPricingToResponse(m *service.DBModelPricing) *listModelPricingResponse {
	if m == nil {
		return nil
	}
	return &listModelPricingResponse{
		ID:                          m.ID,
		ModelID:                     m.ModelID,
		DisplayName:                 m.DisplayName,
		Description:                 m.Description,
		Provider:                    m.Provider,
		Mode:                        m.Mode,
		PricingUnit:                 normalizePricingUnit(m.PricingUnit),
		InputCostPerToken:           m.InputCostPerToken,
		OutputCostPerToken:          m.OutputCostPerToken,
		CacheCreationInputTokenCost: m.CacheCreationInputTokenCost,
		CacheReadInputTokenCost:     m.CacheReadInputTokenCost,
		OutputCostPerImage:          m.OutputCostPerImage,
		OutputCostPerImageToken:     m.OutputCostPerImageToken,
		SupportsPromptCaching:       m.SupportsPromptCaching,
		CustomInputCost:             m.CustomInputCost,
		CustomOutputCost:            m.CustomOutputCost,
		DiscountRate:                m.DiscountRate,
		IsCustom:                    m.IsCustom,
		IsEnabled:                   m.IsEnabled,
		CreatedAt:                   m.CreatedAt.Unix(),
		UpdatedAt:                   m.UpdatedAt.Unix(),
	}
}

func normalizePricingUnit(unit string) string {
	switch strings.TrimSpace(unit) {
	case service.ModelPricingUnitSecond:
		return service.ModelPricingUnitSecond
	case service.ModelPricingUnitImage:
		return service.ModelPricingUnitImage
	case service.ModelPricingUnitVideo:
		return service.ModelPricingUnitVideo
	default:
		return service.ModelPricingUnitToken
	}
}

// List handles listing model pricings with optional filters.
// GET /api/v1/admin/model-pricings
func (h *ModelPricingHandler) List(c *gin.Context) {
	if h.modelPricingService == nil {
		response.Success(c, gin.H{"items": []any{}, "total": 0})
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	provider := strings.TrimSpace(c.Query("provider"))

	var isCustom *bool
	if v := c.Query("is_custom"); v != "" {
		b := v == "true" || v == "1"
		isCustom = &b
	}
	var isEnabled *bool
	if v := c.Query("is_enabled"); v != "" {
		b := v == "true" || v == "1"
		isEnabled = &b
	}
	page := 1
	if v, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && v > 0 {
		page = v
	}
	pageSize := 20
	if v, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil && v > 0 {
		if v > 200 {
			v = 200
		}
		pageSize = v
	}
	// for_whitelist=true：账号编辑弹窗的模型白名单选择器专用，跳过下方的广场可路由集交集过滤。
	// 该选择器要展示的恰恰是「已同步定价数据、但还没被任何账号引用过、因此不在可路由集合里」的
	// 模型——如果也套用广场口径会形成循环依赖（新模型永远搜不到，因为它还没被任何账号选中过）。
	forWhitelist := c.Query("for_whitelist") == "true" || c.Query("for_whitelist") == "1"

	// 后台「模型折扣」默认 = 用户模型广场口径，使运营所见 = 用户所见（不再提供「只看可见」手工开关）。
	// 一次性物化「active 账号可路由」的 ModelInfo（与广场同口径，ModelRoutingService），防 N+1，派生：
	//   - routable：原始可路由 model_id 集，供 pricing_health orphan 判定（不受海外开关影响，
	//     海外但可路由的模型不算孤儿）。
	//   - visible：叠加 FilterVisibleModels（海外开关+版本下限）后的广场可见集，作为列表过滤 allowlist。
	var (
		routable   map[string]struct{}
		visible    map[string]struct{}
		hasRouting bool
	)
	if h.modelRouting != nil {
		if infos, rErr := h.modelRouting.OperatorRoutableModelInfos(c.Request.Context()); rErr == nil {
			hasRouting = true
			routable = make(map[string]struct{}, len(infos))
			for _, m := range infos {
				routable[m.ID] = struct{}{}
			}
			showOverseas := true
			if h.pricingService != nil {
				showOverseas = h.pricingService.GetShowOverseasModels()
			}
			visible = make(map[string]struct{})
			for _, m := range service.FilterVisibleModels(infos, showOverseas) {
				visible[m.ID] = struct{}{}
			}
		}
	}

	filter := service.ModelPricingListFilter{
		Query:     q,
		Provider:  provider,
		IsCustom:  isCustom,
		IsEnabled: isEnabled,
		Page:      page,
		PageSize:  pageSize,
	}
	// 默认始终按广场可见集过滤（= 用户所见）。路由信息不可用时（理论上不会发生）退回全集，避免后台空白。
	// for_whitelist=true 时跳过这层过滤，见上方注释。
	if hasRouting && !forWhitelist {
		filter.VisibleOnly = true
		filter.RoutableModelIDs = make([]string, 0, len(visible))
		for id := range visible {
			filter.RoutableModelIDs = append(filter.RoutableModelIDs, id)
		}
	}

	items, total, err := h.modelPricingService.List(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*listModelPricingResponse, 0, len(items))
	for _, item := range items {
		resp := dbModelPricingToResponse(item)
		resp.PricingHealth = computePricingHealth(item, routable)
		out = append(out, resp)
	}
	response.Paginated(c, out, int64(total), page, pageSize)
}

// Create handles creating a custom model pricing record.
// POST /api/v1/admin/model-pricings
func (h *ModelPricingHandler) Create(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "model pricing repository not available")
		return
	}

	var req createModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "chat"
	}
	pricingUnit := normalizePricingUnit(req.PricingUnit)

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	m := &service.DBModelPricing{
		ModelID:                     req.ModelID,
		DisplayName:                 req.DisplayName,
		Description:                 req.Description,
		Provider:                    req.Provider,
		Mode:                        mode,
		PricingUnit:                 pricingUnit,
		InputCostPerToken:           req.InputCostPerToken,
		OutputCostPerToken:          req.OutputCostPerToken,
		CacheCreationInputTokenCost: req.CacheCreationInputTokenCost,
		CacheReadInputTokenCost:     req.CacheReadInputTokenCost,
		OutputCostPerImage:          req.OutputCostPerImage,
		OutputCostPerImageToken:     req.OutputCostPerImageToken,
		SupportsPromptCaching:       req.SupportsPromptCaching,
		CustomInputCost:             req.CustomInputCost,
		CustomOutputCost:            req.CustomOutputCost,
		DiscountRate:                req.DiscountRate,
		IsCustom:                    true,
		IsEnabled:                   isEnabled,
	}

	if err := h.repo.Create(c.Request.Context(), m); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.pricingService != nil {
		h.pricingService.ReloadFromDB(c.Request.Context())
	}

	response.Success(c, dbModelPricingToResponse(m))
}

// Update handles updating a model pricing record.
// PUT /api/v1/admin/model-pricings/:id
func (h *ModelPricingHandler) Update(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "model pricing repository not available")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return
	}

	var req updateModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	existing, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Apply patch fields selectively
	if req.DisplayName != nil {
		existing.DisplayName = req.DisplayName
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.Provider != nil {
		existing.Provider = *req.Provider
	}
	if req.Mode != nil {
		existing.Mode = *req.Mode
	}
	if req.PricingUnit != nil {
		existing.PricingUnit = normalizePricingUnit(*req.PricingUnit)
	}
	if req.InputCostPerToken != nil {
		existing.InputCostPerToken = req.InputCostPerToken
	}
	if req.OutputCostPerToken != nil {
		existing.OutputCostPerToken = req.OutputCostPerToken
	}
	if req.CacheCreationInputTokenCost != nil {
		existing.CacheCreationInputTokenCost = req.CacheCreationInputTokenCost
	}
	if req.CacheReadInputTokenCost != nil {
		existing.CacheReadInputTokenCost = req.CacheReadInputTokenCost
	}
	if req.OutputCostPerImage != nil {
		existing.OutputCostPerImage = req.OutputCostPerImage
	}
	if req.OutputCostPerImageToken != nil {
		existing.OutputCostPerImageToken = req.OutputCostPerImageToken
	}
	if req.SupportsPromptCaching != nil {
		existing.SupportsPromptCaching = *req.SupportsPromptCaching
	}
	if req.CustomInputCost != nil {
		existing.CustomInputCost = req.CustomInputCost
	}
	if req.CustomOutputCost != nil {
		existing.CustomOutputCost = req.CustomOutputCost
	}
	if req.DiscountRate != nil {
		existing.DiscountRate = req.DiscountRate
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}

	if err := h.repo.Update(c.Request.Context(), existing); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.pricingService != nil {
		h.pricingService.ReloadFromDB(c.Request.Context())
	}

	response.Success(c, dbModelPricingToResponse(existing))
}

// Delete handles deleting a custom model pricing record.
// DELETE /api/v1/admin/model-pricings/:id
func (h *ModelPricingHandler) Delete(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "model pricing repository not available")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "deleted"})
}

// TriggerSync triggers a remote pricing data sync and writes results to DB.
// POST /api/v1/admin/model-pricings/sync
func (h *ModelPricingHandler) TriggerSync(c *gin.Context) {
	if h.pricingService == nil {
		response.BadRequest(c, "pricing service not available")
		return
	}

	if err := h.pricingService.TriggerDBSync(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "sync triggered"})
}

type syncFromUpstreamRequest struct {
	BaseURL    string `json:"base_url" binding:"required"`
	APIKey     string `json:"api_key" binding:"required"`
	AuthHeader string `json:"auth_header"`
	AuthScheme string `json:"auth_scheme"`
	Provider   string `json:"provider"`
	Mode       string `json:"mode"`
}

// SyncFromUpstream fetches the model list from a base URL + API key (newapi/OpenAI compatible)
// and inserts new models into the pricing table (existing records are preserved).
// POST /api/v1/admin/model-pricings/sync-from-upstream
func (h *ModelPricingHandler) SyncFromUpstream(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "model pricing repository not available")
		return
	}
	if h.accountTestService == nil {
		response.InternalError(c, "account test service is not configured")
		return
	}

	var req syncFromUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	models, err := h.accountTestService.FetchModelsByConfig(
		c.Request.Context(),
		strings.TrimSpace(req.BaseURL),
		strings.TrimSpace(req.APIKey),
		req.AuthHeader,
		req.AuthScheme,
		"",
	)
	if err != nil {
		var syncErr *service.UpstreamModelSyncError
		if errors.As(err, &syncErr) {
			switch syncErr.Kind {
			case service.UpstreamModelSyncErrorConfiguration, service.UpstreamModelSyncErrorUnsupported:
				response.BadRequest(c, syncErr.SafeMessage())
			default:
				slog.Warn("model_pricing_sync_from_upstream_failed", "kind", syncErr.Kind)
				response.Error(c, http.StatusBadGateway, syncErr.SafeMessage())
			}
			return
		}
		slog.Warn("model_pricing_sync_from_upstream_failed")
		response.Error(c, http.StatusBadGateway, "Failed to sync models from upstream")
		return
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "chat"
	}
	provider := strings.TrimSpace(req.Provider)

	seeds := make([]*service.DBModelPricing, 0, len(models))
	for _, modelID := range models {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		seeds = append(seeds, &service.DBModelPricing{
			ModelID:  modelID,
			Provider: provider,
			Mode:     mode,
			IsCustom: true,
			// 同步进来的自定义模型尚未配置定价，默认禁用，等待管理员补价格后再启用。
			IsEnabled:     false,
			PricingStatus: service.ModelPricingStatusUnpriced,
		})
	}

	if err := h.repo.SeedIfNotExists(c.Request.Context(), seeds); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 写完立即刷新 pricingService 内存映射，让新模型在「模型广场」立即可见。
	if h.pricingService != nil {
		h.pricingService.ReloadFromDB(c.Request.Context())
	}

	response.Success(c, gin.H{
		"models":  models,
		"fetched": len(models),
	})
}

type syncMaasRequest struct {
	Source          string `json:"source"`
	URL             string `json:"url"`
	AccessToken     string `json:"access_token"`
	SaveCredentials bool   `json:"save_credentials"`
	JsonData        string `json:"json_data"`
}

// SyncMaas 通用 MaaS 定价同步：运营自管的任意来源（wanjie/doubao/lingjing/minimax/glm/...）。
// 价格不内置——必须提供 json_data（上传）或 url+access_token（在线拉取）。
// POST /api/v1/admin/model-pricings/sync-maas
func (h *ModelPricingHandler) SyncMaas(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "model pricing repository not available")
		return
	}
	var req syncMaasRequest
	_ = c.ShouldBindJSON(&req)
	source := strings.TrimSpace(req.Source)
	if source == "" {
		response.BadRequest(c, "source is required")
		return
	}

	ctx := c.Request.Context()
	apiURL := strings.TrimSpace(req.URL)
	token := strings.TrimSpace(req.AccessToken)
	urlKey := "maas_url:" + source
	tokenKey := "maas_token:" + source

	if h.settingService != nil && req.SaveCredentials {
		updates := map[string]string{}
		if apiURL != "" {
			updates[urlKey] = apiURL
		}
		if token != "" {
			updates[tokenKey] = token
		}
		if len(updates) > 0 {
			_ = h.settingService.SetMultiple(ctx, updates)
		}
	}
	if (apiURL == "" || token == "") && h.settingService != nil {
		if saved, err := h.settingService.GetMultiple(ctx, []string{urlKey, tokenKey}); err == nil {
			if apiURL == "" {
				apiURL = strings.TrimSpace(saved[urlKey])
			}
			if token == "" {
				token = strings.TrimSpace(saved[tokenKey])
			}
		}
	}

	cnyRate := service.DefaultWanjieCNYRate
	if h.pricingService != nil {
		cnyRate = h.pricingService.GetCNYRate()
	}

	var (
		parsed []*service.DBModelPricing
		err    error
		mode   string
	)
	switch {
	case strings.TrimSpace(req.JsonData) != "":
		parsed, err = service.ParseMaasFromBytes([]byte(req.JsonData), cnyRate, source)
		mode = "upload"
	case apiURL != "" && token != "":
		parsed, err = service.FetchAndParseMaasModels(ctx, nil, apiURL, token, cnyRate, source)
		mode = "live"
	default:
		response.BadRequest(c, "provide json_data or url+access_token (no built-in pricing)")
		return
	}
	if err != nil {
		response.Error(c, http.StatusBadGateway, "failed to fetch/parse maas data: "+err.Error())
		return
	}
	if err := h.repo.BulkUpsertMaas(ctx, parsed); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.pricingService != nil {
		h.pricingService.ReloadFromDB(ctx)
	}
	response.Success(c, gin.H{"message": "sync completed", "total": len(parsed), "source": source, "mode": mode})
}

// ListProviders 返回模型定价表里出现过的全部 provider，供前端筛选下拉框使用。
// GET /api/v1/admin/model-pricings/providers
func (h *ModelPricingHandler) ListProviders(c *gin.Context) {
	if h.repo == nil {
		response.Success(c, gin.H{"providers": []string{}})
		return
	}
	providers, err := h.repo.ListDistinctProviders(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if providers == nil {
		providers = []string{}
	}
	response.Success(c, gin.H{"providers": providers})
}

// ClearAllDiscounts 将全表所有 discount_rate 置为 NULL。
// POST /api/v1/admin/model-pricings/clear-discounts
func (h *ModelPricingHandler) ClearAllDiscounts(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "model pricing repository not available")
		return
	}
	if err := h.repo.ClearAllDiscountRates(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.pricingService != nil {
		h.pricingService.ReloadFromDB(c.Request.Context())
	}
	response.Success(c, gin.H{"message": "all discount rates cleared"})
}
