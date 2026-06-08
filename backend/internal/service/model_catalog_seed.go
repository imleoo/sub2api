package service

// SSOT 重构 PR-3：模型目录 Bootstrap Seed。
//
// 用途：把 billing_service.go:140-261 的 fallbackPrices（15 条独立模型）
// 与 pricing_service.go:280-329 的灵境 seed（5 条）共 20 条价格落入 DB，
// 作为 catalog 切流（PR-6）的硬前置 —— 任何家族锚点缺失会让 matchByModelFamily
// 塌成 nil → 0 元事故。
//
// 数字必须与 billing_service.go 字面价格 1:1 一致；任何漂移由
// billing_service_golden_test.go 与 pricing_service_match_test.go 捕获。
//
// 入库语义：source="bootstrap", pricing_status="priced"。
// SeedIfNotExists 已存在的 model_id 不覆盖，确保运维已编辑的价格不被还原。

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// 价格字面量复用（参考 billing_service.go:140-261 与 pricing_service.go:30-54）。
// 重复字面在这里再写一遍而不是从 billing 服务读 fallbackPrices 字段，因为：
// 1) seed 阶段 billingService 可能还没初始化
// 2) PR-8 之后 fallbackPrices 字段会被删除，seed 还得能独立存在
const (
	bootstrapSourceLabel = "bootstrap"
)

func bootstrapFloatPtr(v float64) *float64 { return &v }
func bootstrapInt64Ptr(v int64) *int64     { return &v }

// BootstrapPricingSeeds 返回需 seed 到 DB 的全部模型定价记录。
// 字段语义与 ent/schema/model_pricing.go 一致；source/pricing_status 由调用方覆盖。
func BootstrapPricingSeeds() []*DBModelPricing {
	// ---- Anthropic Claude 家族（与 fallbackPrices 一致） ----
	claudeOpus45 := &DBModelPricing{
		ModelID:                     "claude-opus-4.5",
		Provider:                    "anthropic",
		Mode:                        "chat",
		InputCostPerToken:           bootstrapFloatPtr(5e-6),
		OutputCostPerToken:          bootstrapFloatPtr(25e-6),
		CacheCreationInputTokenCost: bootstrapFloatPtr(6.25e-6),
		CacheReadInputTokenCost:     bootstrapFloatPtr(0.5e-6),
		SupportsPromptCaching:       true,
	}
	claudeOpus46 := *claudeOpus45 // 同价
	claudeOpus46.ModelID = "claude-opus-4.6"
	claudeOpus47 := *claudeOpus45
	claudeOpus47.ModelID = "claude-opus-4.7"

	claudeSonnet4 := &DBModelPricing{
		ModelID:                     "claude-sonnet-4",
		Provider:                    "anthropic",
		Mode:                        "chat",
		InputCostPerToken:           bootstrapFloatPtr(3e-6),
		OutputCostPerToken:          bootstrapFloatPtr(15e-6),
		CacheCreationInputTokenCost: bootstrapFloatPtr(3.75e-6),
		CacheReadInputTokenCost:     bootstrapFloatPtr(0.3e-6),
		SupportsPromptCaching:       true,
	}
	claude35Sonnet := *claudeSonnet4
	claude35Sonnet.ModelID = "claude-3-5-sonnet"

	claude35Haiku := &DBModelPricing{
		ModelID:                     "claude-3-5-haiku",
		Provider:                    "anthropic",
		Mode:                        "chat",
		InputCostPerToken:           bootstrapFloatPtr(1e-6),
		OutputCostPerToken:          bootstrapFloatPtr(5e-6),
		CacheCreationInputTokenCost: bootstrapFloatPtr(1.25e-6),
		CacheReadInputTokenCost:     bootstrapFloatPtr(0.1e-6),
		SupportsPromptCaching:       true,
	}
	claude3Opus := &DBModelPricing{
		ModelID:                     "claude-3-opus",
		Provider:                    "anthropic",
		Mode:                        "chat",
		InputCostPerToken:           bootstrapFloatPtr(15e-6),
		OutputCostPerToken:          bootstrapFloatPtr(75e-6),
		CacheCreationInputTokenCost: bootstrapFloatPtr(18.75e-6),
		CacheReadInputTokenCost:     bootstrapFloatPtr(1.5e-6),
		SupportsPromptCaching:       true,
	}
	claude3Haiku := &DBModelPricing{
		ModelID:                     "claude-3-haiku",
		Provider:                    "anthropic",
		Mode:                        "chat",
		InputCostPerToken:           bootstrapFloatPtr(0.25e-6),
		OutputCostPerToken:          bootstrapFloatPtr(1.25e-6),
		CacheCreationInputTokenCost: bootstrapFloatPtr(0.3e-6),
		CacheReadInputTokenCost:     bootstrapFloatPtr(0.03e-6),
		SupportsPromptCaching:       true,
	}

	// ---- Google Gemini ----
	gemini31Pro := &DBModelPricing{
		ModelID:                     "gemini-3.1-pro",
		Provider:                    "google",
		Mode:                        "chat",
		InputCostPerToken:           bootstrapFloatPtr(2e-6),
		OutputCostPerToken:          bootstrapFloatPtr(12e-6),
		CacheCreationInputTokenCost: bootstrapFloatPtr(2e-6),
		CacheReadInputTokenCost:     bootstrapFloatPtr(0.2e-6),
	}

	// ---- OpenAI GPT 系列 ----
	gpt54 := &DBModelPricing{
		ModelID:                         "gpt-5.4",
		Provider:                        "openai",
		Mode:                            "chat",
		InputCostPerToken:               bootstrapFloatPtr(2.5e-6),
		InputCostPerTokenPriority:       bootstrapFloatPtr(5e-6),
		OutputCostPerToken:              bootstrapFloatPtr(15e-6),
		OutputCostPerTokenPriority:      bootstrapFloatPtr(30e-6),
		CacheCreationInputTokenCost:     bootstrapFloatPtr(2.5e-6),
		CacheReadInputTokenCost:         bootstrapFloatPtr(0.25e-6),
		CacheReadInputTokenCostPriority: bootstrapFloatPtr(0.5e-6),
		LongContextInputTokenThreshold:  bootstrapInt64Ptr(272000),
		LongContextInputCostMultiplier:  bootstrapFloatPtr(2.0),
		LongContextOutputCostMultiplier: bootstrapFloatPtr(1.5),
		SupportsPromptCaching:           true,
	}
	gpt55 := *gpt54 // 同价
	gpt55.ModelID = "gpt-5.5"

	gpt54Mini := &DBModelPricing{
		ModelID:                 "gpt-5.4-mini",
		Provider:                "openai",
		Mode:                    "chat",
		InputCostPerToken:       bootstrapFloatPtr(7.5e-7),
		OutputCostPerToken:      bootstrapFloatPtr(4.5e-6),
		CacheReadInputTokenCost: bootstrapFloatPtr(7.5e-8),
		SupportsPromptCaching:   true,
	}
	gpt54Nano := &DBModelPricing{
		ModelID:                 "gpt-5.4-nano",
		Provider:                "openai",
		Mode:                    "chat",
		InputCostPerToken:       bootstrapFloatPtr(2e-7),
		OutputCostPerToken:      bootstrapFloatPtr(1.25e-6),
		CacheReadInputTokenCost: bootstrapFloatPtr(2e-8),
		SupportsPromptCaching:   true,
	}

	gpt52 := &DBModelPricing{
		ModelID:                         "gpt-5.2",
		Provider:                        "openai",
		Mode:                            "chat",
		InputCostPerToken:               bootstrapFloatPtr(1.75e-6),
		InputCostPerTokenPriority:       bootstrapFloatPtr(3.5e-6),
		OutputCostPerToken:              bootstrapFloatPtr(14e-6),
		OutputCostPerTokenPriority:      bootstrapFloatPtr(28e-6),
		CacheCreationInputTokenCost:     bootstrapFloatPtr(1.75e-6),
		CacheReadInputTokenCost:         bootstrapFloatPtr(0.175e-6),
		CacheReadInputTokenCostPriority: bootstrapFloatPtr(0.35e-6),
		SupportsPromptCaching:           true,
	}

	gpt53Codex := &DBModelPricing{
		ModelID:                         "gpt-5.3-codex",
		Provider:                        "openai",
		Mode:                            "chat",
		InputCostPerToken:               bootstrapFloatPtr(1.5e-6),
		InputCostPerTokenPriority:       bootstrapFloatPtr(3e-6),
		OutputCostPerToken:              bootstrapFloatPtr(12e-6),
		OutputCostPerTokenPriority:      bootstrapFloatPtr(24e-6),
		CacheCreationInputTokenCost:     bootstrapFloatPtr(1.5e-6),
		CacheReadInputTokenCost:         bootstrapFloatPtr(0.15e-6),
		CacheReadInputTokenCostPriority: bootstrapFloatPtr(0.3e-6),
		SupportsPromptCaching:           true,
	}
	gpt53CodexSpark := *gpt53Codex // Spark 变体使用与 codex 相同定价
	gpt53CodexSpark.ModelID = "gpt-5.3-codex-spark"

	// 通用化后：灵境/豆包等 MaaS 平台定价不再内置，由运营上传 JSON（sync-maas）管理。
	// 此处仅保留 Claude/GPT/Gemini 基础 catalog 兜底（非 MaaS 平台价）。

	all := []*DBModelPricing{
		claudeOpus45, &claudeOpus46, &claudeOpus47,
		claudeSonnet4, &claude35Sonnet,
		claude35Haiku, claude3Opus, claude3Haiku,
		gemini31Pro,
		gpt54, &gpt55, gpt54Mini, gpt54Nano, gpt52, gpt53Codex, &gpt53CodexSpark,
	}

	// 统一打 source 与 pricing_status 标签；灵境保留 source=lingjing 与现状一致。
	for _, m := range all {
		m.IsEnabled = true
		m.PricingStatus = ModelPricingStatusPriced
		if strings.EqualFold(m.Provider, "lingjing") {
			m.Source = ModelPricingSourceLingjing
		} else {
			m.Source = bootstrapSourceLabel
		}
	}
	return all
}

// BootstrapPricingSeedModelIDs 返回 BootstrapPricingSeeds 的 model_id 列表。
// 供 verify_pricing_coverage CLI / 启动时一致性校验使用。
func BootstrapPricingSeedModelIDs() []string {
	seeds := BootstrapPricingSeeds()
	out := make([]string, 0, len(seeds))
	for _, s := range seeds {
		out = append(out, s.ModelID)
	}
	return out
}

// RunBootstrapSeed 在 Initialize 时调用，把 BootstrapPricingSeeds 写入 model_pricings。
// SeedIfNotExists 已存在 model_id 不覆盖：确保运维已编辑的价格不被还原。
// 失败仅 warn 日志，不阻止启动（黄金值测试 + verify_pricing_coverage 兜底捕获）。
func RunBootstrapSeed(ctx context.Context, repo ModelPricingRepository) {
	if repo == nil {
		return
	}
	seeds := BootstrapPricingSeeds()
	if err := repo.SeedIfNotExists(ctx, seeds); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing][Bootstrap] seed failed: %v", err)
		return
	}
	logger.LegacyPrintf("service.pricing", "[Pricing][Bootstrap] seeded %d models (skip existing)", len(seeds))
}
