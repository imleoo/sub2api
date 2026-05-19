package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ProviderPricingHandler 管理上游 Provider 单价表（Phase 0 P0-3）。
//
// 与 ModelPricingHandler（客户售价配置）正交：
//   - ModelPricing：客户售价 / 折扣 / 自定义计费规则
//   - ProviderPricing：上游真实成本快照（写入 UsageLog.upstream_total_cost）
//
// 详见 docs/upstream-cost-snapshot.md §2.2。
type ProviderPricingHandler struct {
	repo service.ProviderPricingRepository
}

// NewProviderPricingHandler 构造 handler。
func NewProviderPricingHandler(repo service.ProviderPricingRepository) *ProviderPricingHandler {
	return &ProviderPricingHandler{repo: repo}
}

// providerPricingResponse 列表 / 详情统一返回格式。
type providerPricingResponse struct {
	ID                 int64   `json:"id"`
	Provider           string  `json:"provider"`
	Model              string  `json:"model"`
	BillingMode        string  `json:"billing_mode"`
	InputPrice         float64 `json:"input_price"`
	OutputPrice        float64 `json:"output_price"`
	CacheCreationPrice float64 `json:"cache_creation_price"`
	CacheReadPrice     float64 `json:"cache_read_price"`
	Currency           string  `json:"currency"`
	EffectiveFrom      int64   `json:"effective_from"`           // unix seconds
	EffectiveTo        *int64  `json:"effective_to,omitempty"`   // unix seconds, NULL = 持续生效
	Source             string  `json:"source"`
	CreatedAt          int64   `json:"created_at"`
	UpdatedAt          int64   `json:"updated_at"`
}

type createProviderPricingRequest struct {
	Provider           string   `json:"provider" binding:"required"`
	Model              string   `json:"model" binding:"required"`
	BillingMode        string   `json:"billing_mode"` // 默认 token
	InputPrice         float64  `json:"input_price"`
	OutputPrice        float64  `json:"output_price"`
	CacheCreationPrice float64  `json:"cache_creation_price"`
	CacheReadPrice     float64  `json:"cache_read_price"`
	Currency           string   `json:"currency"`           // 默认 USD
	EffectiveFrom      *int64   `json:"effective_from"`     // unix seconds，缺省取 now
	EffectiveTo        *int64   `json:"effective_to"`       // unix seconds 或 null
	Source             string   `json:"source"`             // 默认 manual
}

type updateProviderPricingRequest struct {
	Provider           *string  `json:"provider"`
	Model              *string  `json:"model"`
	BillingMode        *string  `json:"billing_mode"`
	InputPrice         *float64 `json:"input_price"`
	OutputPrice        *float64 `json:"output_price"`
	CacheCreationPrice *float64 `json:"cache_creation_price"`
	CacheReadPrice     *float64 `json:"cache_read_price"`
	Currency           *string  `json:"currency"`
	EffectiveFrom      *int64   `json:"effective_from"`
	EffectiveTo        *int64   `json:"effective_to"` // 显式传 0 表示清空（NULL）
	Source             *string  `json:"source"`
}

func dbProviderPricingToResponse(m *service.DBProviderPricing) *providerPricingResponse {
	if m == nil {
		return nil
	}
	resp := &providerPricingResponse{
		ID:                 m.ID,
		Provider:           m.Provider,
		Model:              m.Model,
		BillingMode:        m.BillingMode,
		InputPrice:         m.InputPrice,
		OutputPrice:        m.OutputPrice,
		CacheCreationPrice: m.CacheCreationPrice,
		CacheReadPrice:     m.CacheReadPrice,
		Currency:           m.Currency,
		EffectiveFrom:      m.EffectiveFrom.Unix(),
		Source:             m.Source,
		CreatedAt:          m.CreatedAt.Unix(),
		UpdatedAt:          m.UpdatedAt.Unix(),
	}
	if m.EffectiveTo != nil {
		t := m.EffectiveTo.Unix()
		resp.EffectiveTo = &t
	}
	return resp
}

// List 列出所有 provider 单价记录（支持按 provider/model/source 过滤）。
// GET /api/v1/admin/provider-pricings
func (h *ProviderPricingHandler) List(c *gin.Context) {
	if h.repo == nil {
		response.Success(c, gin.H{"items": []any{}, "total": 0})
		return
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

	filter := service.ProviderPricingListFilter{
		Provider: strings.TrimSpace(c.Query("provider")),
		Model:    strings.TrimSpace(c.Query("model")),
		Source:   strings.TrimSpace(c.Query("source")),
		Page:     page,
		PageSize: pageSize,
	}

	items, total, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*providerPricingResponse, 0, len(items))
	for _, item := range items {
		out = append(out, dbProviderPricingToResponse(item))
	}
	response.Paginated(c, out, int64(total), page, pageSize)
}

// Create 新增一条单价。
// POST /api/v1/admin/provider-pricings
func (h *ProviderPricingHandler) Create(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "provider pricing repository not available")
		return
	}

	var req createProviderPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	m := &service.DBProviderPricing{
		Provider:           strings.TrimSpace(req.Provider),
		Model:              strings.TrimSpace(req.Model),
		BillingMode:        defaultIfEmpty(req.BillingMode, "token"),
		InputPrice:         req.InputPrice,
		OutputPrice:        req.OutputPrice,
		CacheCreationPrice: req.CacheCreationPrice,
		CacheReadPrice:     req.CacheReadPrice,
		Currency:           defaultIfEmpty(req.Currency, "USD"),
		Source:             defaultIfEmpty(req.Source, "manual"),
	}

	if req.EffectiveFrom != nil {
		m.EffectiveFrom = time.Unix(*req.EffectiveFrom, 0).UTC()
	} else {
		m.EffectiveFrom = time.Now().UTC()
	}
	if req.EffectiveTo != nil && *req.EffectiveTo > 0 {
		t := time.Unix(*req.EffectiveTo, 0).UTC()
		m.EffectiveTo = &t
	}

	if err := h.repo.Create(c.Request.Context(), m); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dbProviderPricingToResponse(m))
}

// Update 修改单价（patch 语义）。
// PUT /api/v1/admin/provider-pricings/:id
func (h *ProviderPricingHandler) Update(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "provider pricing repository not available")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return
	}

	var req updateProviderPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	existing, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if req.Provider != nil {
		existing.Provider = strings.TrimSpace(*req.Provider)
	}
	if req.Model != nil {
		existing.Model = strings.TrimSpace(*req.Model)
	}
	if req.BillingMode != nil {
		existing.BillingMode = *req.BillingMode
	}
	if req.InputPrice != nil {
		existing.InputPrice = *req.InputPrice
	}
	if req.OutputPrice != nil {
		existing.OutputPrice = *req.OutputPrice
	}
	if req.CacheCreationPrice != nil {
		existing.CacheCreationPrice = *req.CacheCreationPrice
	}
	if req.CacheReadPrice != nil {
		existing.CacheReadPrice = *req.CacheReadPrice
	}
	if req.Currency != nil {
		existing.Currency = *req.Currency
	}
	if req.EffectiveFrom != nil {
		existing.EffectiveFrom = time.Unix(*req.EffectiveFrom, 0).UTC()
	}
	if req.EffectiveTo != nil {
		if *req.EffectiveTo > 0 {
			t := time.Unix(*req.EffectiveTo, 0).UTC()
			existing.EffectiveTo = &t
		} else {
			existing.EffectiveTo = nil
		}
	}
	if req.Source != nil {
		existing.Source = *req.Source
	}

	if err := h.repo.Update(c.Request.Context(), existing); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dbProviderPricingToResponse(existing))
}

// Delete 删除单价记录。
// DELETE /api/v1/admin/provider-pricings/:id
func (h *ProviderPricingHandler) Delete(c *gin.Context) {
	if h.repo == nil {
		response.BadRequest(c, "provider pricing repository not available")
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

// defaultIfEmpty 字符串默认值辅助。
func defaultIfEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
