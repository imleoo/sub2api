package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// ModelCatalogService 统一封装模型定价的写路径，按 source 决定覆盖规则，写完立即触发 catalog 重建。
//
// 覆盖规则：
//   - source="litellm"       → UpsertBatch（跳过 is_custom=true 行，即 manual/bootstrap/lingjing 不被覆盖）
//   - source="upstream_sync" → SeedIfNotExists（仅插入，已存在不动）
//   - source="bootstrap"     → SeedIfNotExists
//   - source="lingjing"      → SeedIfNotExists
//   - source="manual"        → 由 model_pricing_handler 直接调用 repo.Create/Update/Delete，不走本方法
type ModelCatalogService struct {
	repo           ModelPricingRepository
	pricingService *PricingService
}

// NewModelCatalogService 创建 ModelCatalogService。pricingService 可为 nil（测试用）。
func NewModelCatalogService(repo ModelPricingRepository, ps *PricingService) *ModelCatalogService {
	return &ModelCatalogService{repo: repo, pricingService: ps}
}

// UpsertModels 按 source 规则写入 batch，写完异步触发 pricingService.ReloadFromDB。
// 返回操作行数（SeedIfNotExists 场景为上限估算，非实际插入数）。
func (s *ModelCatalogService) UpsertModels(ctx context.Context, batch []*DBModelPricing, source string) (int, error) {
	if len(batch) == 0 {
		return 0, nil
	}

	// 为每条记录打上 source 标签（如果调用方未填）
	for _, m := range batch {
		if m.Source == "" {
			m.Source = source
		}
	}

	var err error
	switch source {
	case ModelPricingSourceLiteLLM:
		err = s.repo.UpsertBatch(ctx, batch)
	default:
		// upstream_sync / bootstrap / lingjing / 其它未知 source → insert-only
		err = s.repo.SeedIfNotExists(ctx, batch)
	}

	if err != nil {
		logger.LegacyPrintf("service.catalog", "[Catalog] UpsertModels source=%s count=%d err=%v", source, len(batch), err)
		return 0, err
	}

	logger.LegacyPrintf("service.catalog", "[Catalog] UpsertModels source=%s count=%d ok", source, len(batch))

	if s.pricingService != nil {
		s.pricingService.ReloadFromDB(ctx)
	}
	return len(batch), nil
}
