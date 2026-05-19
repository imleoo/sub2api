package service

import (
	"context"
	"time"
)

// UpstreamUnitPrices 是上游单价快照（Phase 0 P0-4）。
//
// 与 UsageLog.upstream_unit_price_* 4 列字段一一对应；命中时四列同时写入，
// 未命中时一并保持 NULL。详见 docs/upstream-cost-snapshot.md §2.1。
type UpstreamUnitPrices struct {
	InputPrice         float64
	OutputPrice        float64
	CacheCreationPrice float64
	CacheReadPrice     float64
}

// pricingSourceProviderTable 是 UsageLog.pricing_source 的唯一合法值（Phase 0）。
//
// 严格两态语义：要么命中 provider_pricing 表返回此值，要么未命中返回空串。
// **不允许**写入 "litellm"、"fallback"、"estimate" 等估算来源——见
// docs/upstream-cost-snapshot.md §3.1 与 §1。
const pricingSourceProviderTable = "provider_table"

// UpstreamCostResolver 是上游成本快照解析器（Phase 0 P0-4）。
//
// 严格两态：
//   - 命中 provider_pricing 表 → 返回 (cost, unitPrices, "provider_table")
//   - 未命中 / 入参为空 / cost <= 0 → 返回 (nil, nil, "")
//
// **不与 LiteLLM、account_stats_pricing.go 四级回退链交互**。客户售价（actual_cost）
// 仍由现有 resolveAccountStatsCost 决定，与本 Resolver 正交。
//
// 详见 docs/upstream-cost-snapshot.md §3.1 / §3.2。
type UpstreamCostResolver struct {
	repo ProviderPricingRepository
}

// ProvideUpstreamCostResolver 为 wire DI 提供 Resolver。
func ProvideUpstreamCostResolver(repo ProviderPricingRepository) *UpstreamCostResolver {
	return &UpstreamCostResolver{repo: repo}
}

// Resolve 查询给定 (provider, upstreamModel, tokens) 的上游成本快照。
//
// 返回值约定（严格两态）：
//   - 命中且 cost > 0：返回 (&cost, &unitPrices, "provider_table")
//   - provider/model 为空、未命中、cost <= 0：返回 (nil, nil, "")
//
// 单价计算按 BillingMode 区分：
//   - "token"：按四档单价 × token 数累加
//   - "per_request"：按 input_price × 1 次（lingjing Seedance 视频任务）
//   - "image"：按 input_price × ImageOutputTokens（lingjing Seedream 生图等）
//
// 任意 SQL 错误均当作"未命中"处理（返回 (nil, nil, "")），不阻塞 UsageLog 写入热路径。
func (r *UpstreamCostResolver) Resolve(
	ctx context.Context,
	provider, upstreamModel string,
	tokens UsageTokens,
	now time.Time,
) (cost *float64, unitPrices *UpstreamUnitPrices, source string) {
	if r == nil || r.repo == nil {
		return nil, nil, ""
	}
	if provider == "" || upstreamModel == "" {
		return nil, nil, ""
	}

	pricing, err := r.repo.FindEffective(ctx, provider, upstreamModel, now)
	if err != nil || pricing == nil {
		// SQL 错误退化为"未命中"——上游成本快照不阻塞 UsageLog 写入热路径
		return nil, nil, ""
	}

	c := calculateUpstreamCost(pricing, tokens)
	if c <= 0 {
		return nil, nil, ""
	}

	prices := &UpstreamUnitPrices{
		InputPrice:         pricing.InputPrice,
		OutputPrice:        pricing.OutputPrice,
		CacheCreationPrice: pricing.CacheCreationPrice,
		CacheReadPrice:     pricing.CacheReadPrice,
	}
	return &c, prices, pricingSourceProviderTable
}

// calculateUpstreamCost 按 BillingMode 分支计算上游成本。
func calculateUpstreamCost(pricing *DBProviderPricing, tokens UsageTokens) float64 {
	switch pricing.BillingMode {
	case "per_request":
		// 单次计费：按 input_price × 1 次
		return pricing.InputPrice
	case "image":
		// 按张计费：按 input_price × ImageOutputTokens（lingjing 视频/生图虚拟 token=1）
		count := tokens.ImageOutputTokens
		if count <= 0 {
			count = 1
		}
		return pricing.InputPrice * float64(count)
	default:
		// 默认 token 计费（也覆盖空 BillingMode 的兼容情况）
		return float64(tokens.InputTokens)*pricing.InputPrice +
			float64(tokens.OutputTokens)*pricing.OutputPrice +
			float64(tokens.CacheCreationTokens)*pricing.CacheCreationPrice +
			float64(tokens.CacheReadTokens)*pricing.CacheReadPrice
	}
}
