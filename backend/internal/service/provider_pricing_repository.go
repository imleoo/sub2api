package service

import (
	"context"
	"time"
)

// DBProviderPricing 是上游 Provider 单价表的 service 层 DTO（Phase 0 P0-3 引入）。
//
// 与 UsageLog.upstream_unit_price_* / upstream_total_cost / pricing_source 配套，
// 由 UpstreamCostResolver.Resolve(...) 查询。严格两态：命中返回真实单价快照，
// 未命中保持 NULL，不混入 LiteLLM / account_stats_pricing 估算。
//
// 详见 docs/upstream-cost-snapshot.md §2.2 / §3。
type DBProviderPricing struct {
	ID                 int64
	Provider           string
	Model              string
	BillingMode        string // token | per_request | image
	InputPrice         float64
	OutputPrice        float64
	CacheCreationPrice float64
	CacheReadPrice     float64
	Currency           string
	EffectiveFrom      time.Time
	EffectiveTo        *time.Time
	Source             string // manual | litellm_sync | upstream_billing_api
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ProviderPricingListFilter 列表查询过滤条件
type ProviderPricingListFilter struct {
	Provider string // 按 provider 过滤（已规范化的 provider_key）
	Model    string // 按 model 模糊匹配
	Source   string // 按 source 过滤
	Page     int    // 从 1 开始
	PageSize int    // 默认 20，最大 200
}

// ProviderPricingRepository 接口
//
// FindEffective 是热路径核心方法：给定 (provider, model, now) 返回当前生效单价快照；
// 未命中返回 (nil, nil)。命中规则：
//   - 优先精确 model 匹配，未命中再尝试 "*" 单星通配
//   - effective_from <= now AND (effective_to IS NULL OR now < effective_to)
//   - 多条命中取最新 effective_from
type ProviderPricingRepository interface {
	// Create 创建一条单价记录
	Create(ctx context.Context, m *DBProviderPricing) error

	// Update 更新已有记录
	Update(ctx context.Context, m *DBProviderPricing) error

	// Delete 删除记录
	Delete(ctx context.Context, id int64) error

	// GetByID 按主键查询
	GetByID(ctx context.Context, id int64) (*DBProviderPricing, error)

	// List 分页查询
	List(ctx context.Context, filter ProviderPricingListFilter) ([]*DBProviderPricing, int, error)

	// FindEffective 查询给定 (provider, model, now) 的当前生效单价快照；未命中返回 nil（不报错）
	FindEffective(ctx context.Context, provider, model string, now time.Time) (*DBProviderPricing, error)
}
