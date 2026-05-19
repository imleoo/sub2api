package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/providerpricing"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// providerPricingRepository 是 ProviderPricingRepository 的 ent 实现（Phase 0 P0-3）。
//
// 数据语义见 docs/upstream-cost-snapshot.md §2.2。FindEffective 是热路径核心方法，
// 由 UpstreamCostResolver.Resolve 在每次 UsageLog 装配时调用。
type providerPricingRepository struct {
	client *dbent.Client
}

// NewProviderPricingRepository 创建 ent 实现。
func NewProviderPricingRepository(client *dbent.Client) service.ProviderPricingRepository {
	return &providerPricingRepository{client: client}
}

// Create 创建一条单价记录。
func (r *providerPricingRepository) Create(ctx context.Context, m *service.DBProviderPricing) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ProviderPricing.Create().
		SetProvider(m.Provider).
		SetModel(m.Model).
		SetInputPrice(m.InputPrice).
		SetOutputPrice(m.OutputPrice).
		SetCacheCreationPrice(m.CacheCreationPrice).
		SetCacheReadPrice(m.CacheReadPrice)

	if m.BillingMode != "" {
		builder.SetBillingMode(m.BillingMode)
	}
	if m.Currency != "" {
		builder.SetCurrency(m.Currency)
	}
	if !m.EffectiveFrom.IsZero() {
		builder.SetEffectiveFrom(m.EffectiveFrom)
	}
	if m.EffectiveTo != nil {
		builder.SetEffectiveTo(*m.EffectiveTo)
	}
	if m.Source != "" {
		builder.SetSource(m.Source)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	m.ID = created.ID
	m.CreatedAt = created.CreatedAt
	m.UpdatedAt = created.UpdatedAt
	m.EffectiveFrom = created.EffectiveFrom
	if created.BillingMode != "" {
		m.BillingMode = created.BillingMode
	}
	if created.Currency != "" {
		m.Currency = created.Currency
	}
	if created.Source != "" {
		m.Source = created.Source
	}
	return nil
}

// Update 更新已有记录。
func (r *providerPricingRepository) Update(ctx context.Context, m *service.DBProviderPricing) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ProviderPricing.UpdateOneID(m.ID).
		SetProvider(m.Provider).
		SetModel(m.Model).
		SetBillingMode(m.BillingMode).
		SetInputPrice(m.InputPrice).
		SetOutputPrice(m.OutputPrice).
		SetCacheCreationPrice(m.CacheCreationPrice).
		SetCacheReadPrice(m.CacheReadPrice).
		SetCurrency(m.Currency).
		SetEffectiveFrom(m.EffectiveFrom).
		SetSource(m.Source)

	if m.EffectiveTo != nil {
		builder.SetEffectiveTo(*m.EffectiveTo)
	} else {
		builder.ClearEffectiveTo()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, errProviderPricingNotFound, nil)
	}
	m.UpdatedAt = updated.UpdatedAt
	return nil
}

// Delete 删除记录。
func (r *providerPricingRepository) Delete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	return translatePersistenceError(
		client.ProviderPricing.DeleteOneID(id).Exec(ctx),
		errProviderPricingNotFound,
		nil,
	)
}

// GetByID 按主键查询。
func (r *providerPricingRepository) GetByID(ctx context.Context, id int64) (*service.DBProviderPricing, error) {
	m, err := r.client.ProviderPricing.Get(ctx, id)
	if err != nil {
		return nil, translatePersistenceError(err, errProviderPricingNotFound, nil)
	}
	return providerPricingEntityToService(m), nil
}

// List 分页查询。
func (r *providerPricingRepository) List(ctx context.Context, filter service.ProviderPricingListFilter) ([]*service.DBProviderPricing, int, error) {
	q := r.client.ProviderPricing.Query()

	if filter.Provider != "" {
		q = q.Where(providerpricing.ProviderEQ(filter.Provider))
	}
	if filter.Model != "" {
		q = q.Where(providerpricing.ModelContainsFold(filter.Model))
	}
	if filter.Source != "" {
		q = q.Where(providerpricing.SourceEQ(filter.Source))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("provider pricing list count: %w", err)
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize

	items, err := q.
		Order(dbent.Desc(providerpricing.FieldEffectiveFrom), dbent.Desc(providerpricing.FieldID)).
		Offset(offset).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("provider pricing list query: %w", err)
	}

	out := make([]*service.DBProviderPricing, 0, len(items))
	for _, item := range items {
		out = append(out, providerPricingEntityToService(item))
	}
	return out, total, nil
}

// FindEffective 查询给定 (provider, model, now) 的当前生效单价快照。
//
// 命中规则（按文档 §2.2 / §3.2）：
//  1. 先按 (provider, model) 精确匹配 + effective_from <= now AND (effective_to IS NULL OR now < effective_to)
//  2. 未命中再尝试 (provider, "*") 通配匹配（同样的时间窗口）
//  3. 多条命中时取最新 effective_from
//
// 未命中返回 (nil, nil)，不报错。
func (r *providerPricingRepository) FindEffective(
	ctx context.Context,
	provider, model string,
	now time.Time,
) (*service.DBProviderPricing, error) {
	if provider == "" || model == "" {
		return nil, nil
	}

	if m, err := r.findEffectiveFor(ctx, provider, model, now); err != nil {
		return nil, err
	} else if m != nil {
		return m, nil
	}

	// 通配匹配 "*"
	if m, err := r.findEffectiveFor(ctx, provider, "*", now); err != nil {
		return nil, err
	} else if m != nil {
		return m, nil
	}

	return nil, nil
}

// findEffectiveFor 给定精确 model 名（含 "*"）查询当前生效单价。
func (r *providerPricingRepository) findEffectiveFor(
	ctx context.Context,
	provider, model string,
	now time.Time,
) (*service.DBProviderPricing, error) {
	rows, err := r.client.ProviderPricing.Query().
		Where(
			providerpricing.ProviderEQ(provider),
			providerpricing.ModelEQ(model),
			providerpricing.EffectiveFromLTE(now),
			providerpricing.Or(
				providerpricing.EffectiveToIsNil(),
				providerpricing.EffectiveToGT(now),
			),
		).
		Order(dbent.Desc(providerpricing.FieldEffectiveFrom)).
		Limit(1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("provider pricing find effective: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return providerPricingEntityToService(rows[0]), nil
}

// --- helpers ---

// errProviderPricingNotFound sentinel.
var errProviderPricingNotFound = infraerrors.New(404, "PROVIDER_PRICING_NOT_FOUND", "provider pricing not found")

// providerPricingEntityToService converts an Ent entity to the service layer struct.
func providerPricingEntityToService(m *dbent.ProviderPricing) *service.DBProviderPricing {
	if m == nil {
		return nil
	}
	return &service.DBProviderPricing{
		ID:                 m.ID,
		Provider:           m.Provider,
		Model:              m.Model,
		BillingMode:        m.BillingMode,
		InputPrice:         m.InputPrice,
		OutputPrice:        m.OutputPrice,
		CacheCreationPrice: m.CacheCreationPrice,
		CacheReadPrice:     m.CacheReadPrice,
		Currency:           m.Currency,
		EffectiveFrom:      m.EffectiveFrom,
		EffectiveTo:        m.EffectiveTo,
		Source:             m.Source,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}
