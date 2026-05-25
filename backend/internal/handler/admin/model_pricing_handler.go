package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPricingHandler handles admin model pricing management.
type ModelPricingHandler struct {
	repo           service.ModelPricingRepository
	pricingService *service.PricingService
	settingService *service.SettingService
}

// NewModelPricingHandler creates a new ModelPricingHandler.
func NewModelPricingHandler(repo service.ModelPricingRepository, ps *service.PricingService, ss *service.SettingService) *ModelPricingHandler {
	return &ModelPricingHandler{repo: repo, pricingService: ps, settingService: ss}
}

// listModelPricingResponse is the JSON shape returned per record.
type listModelPricingResponse struct {
	ID                          int64    `json:"id"`
	ModelID                     string   `json:"model_id"`
	DisplayName                 *string  `json:"display_name,omitempty"`
	Description                 *string  `json:"description,omitempty"`
	Provider                    string   `json:"provider"`
	Mode                        string   `json:"mode"`
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
	CreatedAt                   int64    `json:"created_at"`
	UpdatedAt                   int64    `json:"updated_at"`
}

type createModelPricingRequest struct {
	ModelID                     string   `json:"model_id" binding:"required"`
	DisplayName                 *string  `json:"display_name"`
	Description                 *string  `json:"description"`
	Provider                    string   `json:"provider"`
	Mode                        string   `json:"mode"`
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

// List handles listing model pricings with optional filters.
// GET /api/v1/admin/model-pricings
func (h *ModelPricingHandler) List(c *gin.Context) {
	if h.repo == nil {
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

	filter := service.ModelPricingListFilter{
		Query:     q,
		Provider:  provider,
		IsCustom:  isCustom,
		IsEnabled: isEnabled,
		Page:      page,
		PageSize:  pageSize,
	}

	// show_overseas_models=false 时隐藏海外 provider 的模型，与模型广场保持一致。
	if h.settingService != nil {
		if ps, err := h.settingService.GetPublicSettings(c.Request.Context()); err == nil && ps != nil && !ps.ShowOverseasModels {
			filter.ExcludeProviders = service.OverseasModelProviders
		}
	}

	items, total, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*listModelPricingResponse, 0, len(items))
	for _, item := range items {
		out = append(out, dbModelPricingToResponse(item))
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
