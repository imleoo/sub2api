package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/modelpricing"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelPricingRepository struct {
	client *dbent.Client
}

// NewModelPricingRepository creates a ModelPricingRepository backed by Ent.
func NewModelPricingRepository(client *dbent.Client) service.ModelPricingRepository {
	return &modelPricingRepository{client: client}
}

// UpsertBatch 批量 upsert 来自远端同步的定价数据。
// is_custom=true 的记录不会被覆盖（DO UPDATE … WHERE NOT is_custom）。
// 为了兼容 Ent 的 OnConflict API，我们在 Go 层过滤掉 is_custom=true 的记录
// （它们在 DB 中已存在，ON CONFLICT DO UPDATE 只对已存在记录生效，
//
//	而 Ent 不直接支持 WHERE 子句的 ON CONFLICT，所以先 GetByModelID 过滤）。
//
// 实际上最高效的方式是用 CreateBulk + OnConflict，然后在 Update 函数里
// 不更新 is_custom 为 true 的记录——做法：每批次先查询哪些 model_id 已是 custom，
// 排除后再 bulk upsert。
func (r *modelPricingRepository) UpsertBatch(ctx context.Context, models []*service.DBModelPricing) error {
	if len(models) == 0 {
		return nil
	}

	// 查询已标记为 is_custom=true 的 model_id，以便跳过
	customIDs, err := r.client.ModelPricing.Query().
		Where(modelpricing.IsCustomEQ(true)).
		Select(modelpricing.FieldModelID).
		Strings(ctx)
	if err != nil {
		return fmt.Errorf("upsert batch: query custom models: %w", err)
	}
	customSet := make(map[string]struct{}, len(customIDs))
	for _, id := range customIDs {
		customSet[id] = struct{}{}
	}

	// 过滤掉手动自定义记录
	filtered := make([]*service.DBModelPricing, 0, len(models))
	for _, m := range models {
		if _, isCustom := customSet[m.ModelID]; !isCustom {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	// 分批处理，每批最多 500 条，避免参数超限
	batchSize := 500
	for i := 0; i < len(filtered); i += batchSize {
		end := i + batchSize
		if end > len(filtered) {
			end = len(filtered)
		}
		if err := r.upsertBatchSlice(ctx, filtered[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r *modelPricingRepository) upsertBatchSlice(ctx context.Context, models []*service.DBModelPricing) error {
	now := time.Now()
	builders := make([]*dbent.ModelPricingCreate, 0, len(models))
	for _, m := range models {
		c := r.client.ModelPricing.Create().
			SetModelID(m.ModelID).
			SetProvider(m.Provider).
			SetMode(m.Mode).
			SetSupportsPromptCaching(m.SupportsPromptCaching).
			SetIsCustom(false).
			SetIsEnabled(m.IsEnabled).
			SetNillableDisplayName(m.DisplayName).
			SetNillableDescription(m.Description).
			SetNillableInputCostPerToken(m.InputCostPerToken).
			SetNillableOutputCostPerToken(m.OutputCostPerToken).
			SetNillableCacheCreationInputTokenCost(m.CacheCreationInputTokenCost).
			SetNillableCacheReadInputTokenCost(m.CacheReadInputTokenCost).
			SetNillableOutputCostPerImage(m.OutputCostPerImage).
			SetNillableOutputCostPerImageToken(m.OutputCostPerImageToken).
			SetLastSyncedAt(now)
		builders = append(builders, c)
	}

	return r.client.ModelPricing.CreateBulk(builders...).
		OnConflictColumns(modelpricing.FieldModelID).
		Update(func(u *dbent.ModelPricingUpsert) {
			u.UpdateProvider()
			u.UpdateMode()
			u.UpdateSupportsPromptCaching()
			u.UpdateDisplayName()
			u.UpdateDescription()
			u.UpdateInputCostPerToken()
			u.UpdateOutputCostPerToken()
			u.UpdateCacheCreationInputTokenCost()
			u.UpdateCacheReadInputTokenCost()
			u.UpdateOutputCostPerImage()
			u.UpdateOutputCostPerImageToken()
			u.UpdateLastSyncedAt()
			u.UpdateUpdatedAt()
		}).
		Exec(ctx)
}

// Create 创建一条手动自定义模型定价记录。
func (r *modelPricingRepository) Create(ctx context.Context, m *service.DBModelPricing) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.ModelPricing.Create().
		SetModelID(m.ModelID).
		SetProvider(m.Provider).
		SetMode(m.Mode).
		SetSupportsPromptCaching(m.SupportsPromptCaching).
		SetIsCustom(true).
		SetIsEnabled(m.IsEnabled).
		SetNillableDisplayName(m.DisplayName).
		SetNillableDescription(m.Description).
		SetNillableInputCostPerToken(m.InputCostPerToken).
		SetNillableOutputCostPerToken(m.OutputCostPerToken).
		SetNillableCacheCreationInputTokenCost(m.CacheCreationInputTokenCost).
		SetNillableCacheReadInputTokenCost(m.CacheReadInputTokenCost).
		SetNillableOutputCostPerImage(m.OutputCostPerImage).
		SetNillableOutputCostPerImageToken(m.OutputCostPerImageToken).
		SetNillableCustomInputCost(m.CustomInputCost).
		SetNillableCustomOutputCost(m.CustomOutputCost).
		SetNillableDiscountRate(m.DiscountRate).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	m.ID = created.ID
	m.CreatedAt = created.CreatedAt
	m.UpdatedAt = created.UpdatedAt
	return nil
}

// GetByID 按主键查询。
func (r *modelPricingRepository) GetByID(ctx context.Context, id int64) (*service.DBModelPricing, error) {
	m, err := r.client.ModelPricing.Get(ctx, id)
	if err != nil {
		return nil, translatePersistenceError(err, errModelPricingNotFound, nil)
	}
	return modelPricingEntityToService(m), nil
}

// GetByModelID 按 model_id 查询。
func (r *modelPricingRepository) GetByModelID(ctx context.Context, modelID string) (*service.DBModelPricing, error) {
	m, err := r.client.ModelPricing.Query().
		Where(modelpricing.ModelIDEQ(modelID)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, errModelPricingNotFound, nil)
	}
	return modelPricingEntityToService(m), nil
}

// List 分页查询，支持模糊搜索和多维度过滤。
func (r *modelPricingRepository) List(ctx context.Context, filter service.ModelPricingListFilter) ([]*service.DBModelPricing, int, error) {
	q := r.client.ModelPricing.Query()

	if filter.Query != "" {
		q = q.Where(
			modelpricing.Or(
				modelpricing.ModelIDContainsFold(filter.Query),
				modelpricing.DisplayNameContainsFold(filter.Query),
			),
		)
	}
	if filter.Provider != "" {
		q = q.Where(modelpricing.ProviderEQ(filter.Provider))
	}
	if len(filter.ExcludeProviders) > 0 {
		q = q.Where(modelpricing.ProviderNotIn(filter.ExcludeProviders...))
	}
	if filter.IsCustom != nil {
		q = q.Where(modelpricing.IsCustomEQ(*filter.IsCustom))
	}
	if filter.IsEnabled != nil {
		q = q.Where(modelpricing.IsEnabledEQ(*filter.IsEnabled))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("model pricing list count: %w", err)
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
		Order(dbent.Desc(modelpricing.FieldID)).
		Offset(offset).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("model pricing list query: %w", err)
	}

	out := make([]*service.DBModelPricing, 0, len(items))
	for _, item := range items {
		out = append(out, modelPricingEntityToService(item))
	}
	return out, total, nil
}

// Update 更新已有记录。
func (r *modelPricingRepository) Update(ctx context.Context, m *service.DBModelPricing) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ModelPricing.UpdateOneID(m.ID).
		SetProvider(m.Provider).
		SetMode(m.Mode).
		SetSupportsPromptCaching(m.SupportsPromptCaching).
		SetIsEnabled(m.IsEnabled)

	if m.DisplayName != nil {
		builder.SetDisplayName(*m.DisplayName)
	} else {
		builder.ClearDisplayName()
	}
	if m.Description != nil {
		builder.SetDescription(*m.Description)
	} else {
		builder.ClearDescription()
	}
	if m.InputCostPerToken != nil {
		builder.SetInputCostPerToken(*m.InputCostPerToken)
	} else {
		builder.ClearInputCostPerToken()
	}
	if m.OutputCostPerToken != nil {
		builder.SetOutputCostPerToken(*m.OutputCostPerToken)
	} else {
		builder.ClearOutputCostPerToken()
	}
	if m.CacheCreationInputTokenCost != nil {
		builder.SetCacheCreationInputTokenCost(*m.CacheCreationInputTokenCost)
	} else {
		builder.ClearCacheCreationInputTokenCost()
	}
	if m.CacheReadInputTokenCost != nil {
		builder.SetCacheReadInputTokenCost(*m.CacheReadInputTokenCost)
	} else {
		builder.ClearCacheReadInputTokenCost()
	}
	if m.OutputCostPerImage != nil {
		builder.SetOutputCostPerImage(*m.OutputCostPerImage)
	} else {
		builder.ClearOutputCostPerImage()
	}
	if m.OutputCostPerImageToken != nil {
		builder.SetOutputCostPerImageToken(*m.OutputCostPerImageToken)
	} else {
		builder.ClearOutputCostPerImageToken()
	}
	if m.CustomInputCost != nil {
		builder.SetCustomInputCost(*m.CustomInputCost)
	} else {
		builder.ClearCustomInputCost()
	}
	if m.CustomOutputCost != nil {
		builder.SetCustomOutputCost(*m.CustomOutputCost)
	} else {
		builder.ClearCustomOutputCost()
	}
	if m.DiscountRate != nil {
		builder.SetDiscountRate(*m.DiscountRate)
	} else {
		builder.ClearDiscountRate()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, errModelPricingNotFound, nil)
	}
	m.UpdatedAt = updated.UpdatedAt
	return nil
}

// Delete 删除记录，仅允许删除 is_custom=true 的记录。
func (r *modelPricingRepository) Delete(ctx context.Context, id int64) error {
	m, err := r.client.ModelPricing.Get(ctx, id)
	if err != nil {
		return translatePersistenceError(err, errModelPricingNotFound, nil)
	}
	if !m.IsCustom {
		return fmt.Errorf("cannot delete non-custom model pricing record (id=%d)", id)
	}
	client := clientFromContext(ctx, r.client)
	return client.ModelPricing.DeleteOneID(id).Exec(ctx)
}

// LoadAllEnabled 加载所有启用的记录，供内存缓存构建。
func (r *modelPricingRepository) LoadAllEnabled(ctx context.Context) ([]*service.DBModelPricing, error) {
	items, err := r.client.ModelPricing.Query().
		Where(modelpricing.IsEnabledEQ(true)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load all enabled pricings: %w", err)
	}
	out := make([]*service.DBModelPricing, 0, len(items))
	for _, item := range items {
		out = append(out, modelPricingEntityToService(item))
	}
	return out, nil
}

// BulkUpdateDiscountRates 批量设置折扣率（每条记录用 UpdateOne 更新）。
// 只更新 discount_rate 字段，不影响 is_custom 状态。
func (r *modelPricingRepository) BulkUpdateDiscountRates(ctx context.Context, rates map[string]float64) error {
	if len(rates) == 0 {
		return nil
	}
	for modelID, rate := range rates {
		rate := rate // capture
		err := r.client.ModelPricing.Update().
			Where(modelpricing.ModelIDEQ(modelID)).
			SetDiscountRate(rate).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("bulk update discount for %s: %w", modelID, err)
		}
	}
	return nil
}

// SeedIfNotExists 仅在 model_id 不存在时插入（用于灵境模型 seed）。
func (r *modelPricingRepository) SeedIfNotExists(ctx context.Context, models []*service.DBModelPricing) error {
	for _, m := range models {
		exists, err := r.client.ModelPricing.Query().
			Where(modelpricing.ModelIDEQ(m.ModelID)).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("seed check %s: %w", m.ModelID, err)
		}
		if exists {
			continue
		}
		if err := r.Create(ctx, m); err != nil {
			return fmt.Errorf("seed create %s: %w", m.ModelID, err)
		}
	}
	return nil
}

// --- helpers ---

// errModelPricingNotFound sentinel error – a proper ApplicationError so translatePersistenceError can use it.
var errModelPricingNotFound = infraerrors.New(404, "MODEL_PRICING_NOT_FOUND", "model pricing not found")

// modelPricingEntityToService converts an Ent entity to the service layer struct.
func modelPricingEntityToService(m *dbent.ModelPricing) *service.DBModelPricing {
	if m == nil {
		return nil
	}
	return &service.DBModelPricing{
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
		LastSyncedAt:                m.LastSyncedAt,
		CreatedAt:                   m.CreatedAt,
		UpdatedAt:                   m.UpdatedAt,
	}
}
