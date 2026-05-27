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
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// ModelPricingListFilter 列表查询过滤条件
type ModelPricingListFilter struct {
	Query            string   // 搜索关键词（model_id/display_name 模糊匹配）
	Provider         string   // 按提供商过滤
	ExcludeProviders        []string // 排除这些 provider，空=不排除（保留兼容；新过滤建议用 ExcludeOverseasModels）
	ExcludeOverseasModels   bool     // 按 model_id 前缀排除海外模型（与 OverseasModelIDPrefixes 同步）
	IsCustom         *bool    // 按来源过滤，nil=全部
	IsEnabled        *bool    // 按启用状态过滤，nil=全部
	Page             int      // 从 1 开始
	PageSize         int      // 默认 20，最大 200
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

	// SeedIfNotExists 如果 model_id 不存在则插入（用于灵境模型 seed）
	SeedIfNotExists(ctx context.Context, models []*DBModelPricing) error
}
