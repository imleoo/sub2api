package service

import (
	"context"
	"time"
)

// DBModelPricing 定价数据库记录（service 层结构体）
type DBModelPricing struct {
	ID                          int64
	ModelID                     string
	DisplayName                 *string
	Description                 *string
	Provider                    string
	Mode                        string
	PricingUnit                 string
	InputCostPerToken           *float64
	OutputCostPerToken          *float64
	CacheCreationInputTokenCost *float64
	CacheReadInputTokenCost     *float64
	OutputCostPerImage          *float64
	OutputCostPerImageToken     *float64
	SupportsPromptCaching       bool
	CustomInputCost             *float64
	CustomOutputCost            *float64
	DiscountRate                *float64
	IsCustom                    bool
	IsEnabled                   bool
	LastSyncedAt                *time.Time

	// SSOT 重构 PR-1：扩展字段
	InputCostPerTokenPriority       *float64
	OutputCostPerTokenPriority      *float64
	CacheReadInputTokenCostPriority *float64
	CacheCreation5mTokenCost        *float64
	CacheCreation1hTokenCost        *float64
	SupportsCacheBreakdown          bool
	ImageOutputPricePerToken        *float64
	LongContextInputTokenThreshold  *int64
	LongContextInputCostMultiplier  *float64
	LongContextOutputCostMultiplier *float64
	// SSOT 元数据
	Source          string // 定价来源标签（任意字符串：litellm / 上传的 MaaS source 名 / manual ...）
	SourceProvider  string // upstream_sync 时记录账号 name
	SourceAccountID *int64 // 触发入库的账号 id（仅 upstream_sync）
	PricingStatus   string // priced / unpriced / disabled

	// 视频按 token 计费的分档单价（来自上传的定价 JSON；¥/百万 token 原值，计费时按汇率折 USD）。
	// 通用化后视频分档不再内置，统一从此字段读取。
	TierPricing []VideoPriceTier

	CreatedAt time.Time
	UpdatedAt time.Time
}

// VideoPriceTier 视频按 token 计费的一个档位（来自上传的定价 JSON）。
type VideoPriceTier struct {
	Spec         string  `json:"spec"`            // 规格描述，如 "在线推理-无声视频" / "在线推理-1080p-输入包含视频"
	CNYPerMToken float64 `json:"cny_per_m_token"` // ¥/百万 token 原值
}

// Source 常量（用于 DBModelPricing.Source 字段）。
const (
	ModelPricingSourceLiteLLM      = "litellm"
	ModelPricingSourceUpstreamSync = "upstream_sync"
	ModelPricingSourceManual       = "manual"
	ModelPricingSourceBootstrap    = "bootstrap"
	ModelPricingSourceLingjing     = "lingjing"
)

// PricingUnit 常量。
const (
	ModelPricingUnitToken  = "token"
	ModelPricingUnitSecond = "second"
	ModelPricingUnitImage  = "image_generation"
	ModelPricingUnitVideo  = "video_generation"
)

// PricingStatus 常量。
const (
	ModelPricingStatusPriced   = "priced"
	ModelPricingStatusUnpriced = "unpriced"
	ModelPricingStatusDisabled = "disabled"
)

// ModelPricingListFilter 列表查询过滤条件
type ModelPricingListFilter struct {
	Query                 string   // 搜索关键词（model_id/display_name 模糊匹配）
	Provider              string   // 按提供商过滤
	ExcludeProviders      []string // 排除这些 provider，空=不排除（保留兼容；新过滤建议用 ExcludeOverseasModels）
	ExcludeOverseasModels bool     // 按 model_id 前缀排除海外模型（与 OverseasModelIDPrefixes 同步）
	IsCustom              *bool    // 按来源过滤，nil=全部
	IsEnabled             *bool    // 按启用状态过滤，nil=全部
	VisibleOnly           bool     // 仅返回可路由模型（启用 + 在 RoutableModelIDs 内）
	RoutableModelIDs      []string // VisibleOnly=true 时的 allowlist（由 ModelRoutingService 计算，与模型广场同口径）
	Page                  int      // 从 1 开始
	PageSize              int      // 默认 20，最大 200
}

// ModelPricingRepository 接口
type ModelPricingRepository interface {
	// UpsertBatch 批量 upsert（同步远端 JSON 数据用）
	// is_custom=true 的记录会被跳过（不覆盖）
	UpsertBatch(ctx context.Context, models []*DBModelPricing) error

	// Create 创建自定义模型
	Create(ctx context.Context, m *DBModelPricing) error

	// GetByID 按 ID 查询
	GetByID(ctx context.Context, id int64) (*DBModelPricing, error)

	// GetByModelID 按 model_id 查询
	GetByModelID(ctx context.Context, modelID string) (*DBModelPricing, error)

	// List 分页查询（支持搜索/过滤）
	List(ctx context.Context, filter ModelPricingListFilter) ([]*DBModelPricing, int, error)

	// Update 更新（折扣、自定义价格、描述、启用状态等）
	Update(ctx context.Context, m *DBModelPricing) error

	// Delete 删除（仅允许删除 is_custom=true 的记录）
	Delete(ctx context.Context, id int64) error

	// LoadAllEnabled 加载全部启用的记录（供内存 map 构建）
	LoadAllEnabled(ctx context.Context) ([]*DBModelPricing, error)

	// BulkUpdateDiscountRates 批量设置折扣率（迁移旧 settings.model_discounts 数据）
	BulkUpdateDiscountRates(ctx context.Context, rates map[string]float64) error

	// ClearAllDiscountRates 将全表所有记录的 discount_rate 清空（置为 NULL）
	ClearAllDiscountRates(ctx context.Context) error

	// SeedIfNotExists 如果 model_id 不存在则插入（用于灵境模型 seed）
	SeedIfNotExists(ctx context.Context, models []*DBModelPricing) error

	// BulkUpsertWanjie 是 BulkUpsertMaas 的兼容包装（万界来源）。
	BulkUpsertWanjie(ctx context.Context, models []*DBModelPricing) error

	// BulkUpsertMaas 将万界/豆包 MaaS 平台定价批量写入（source 由各记录 m.Source 决定）：
	// - is_custom=true 的已有记录：更新 mode、provider（若为空）及全部定价字段
	// - is_custom=false 的已有记录（LiteLLM 来源）：跳过，保留 USD 定价
	// - 不存在的记录：新建（is_custom=true，source 取 m.Source）
	BulkUpsertMaas(ctx context.Context, models []*DBModelPricing) error

	// ListDistinctProviders 返回当前模型定价表中出现过的全部 provider（去重、按字母排序），供前端筛选下拉框使用。
	ListDistinctProviders(ctx context.Context) ([]string, error)
}

// ModelPricingService contains admin model-pricing business logic.
type ModelPricingService struct {
	repo ModelPricingRepository
}

// NewModelPricingService creates a model-pricing service.
func NewModelPricingService(repo ModelPricingRepository) *ModelPricingService {
	return &ModelPricingService{repo: repo}
}

// List returns model pricing rows using the repository-level filters.
func (s *ModelPricingService) List(ctx context.Context, filter ModelPricingListFilter) ([]*DBModelPricing, int, error) {
	if s == nil || s.repo == nil {
		return []*DBModelPricing{}, 0, nil
	}
	return s.repo.List(ctx, filter)
}
