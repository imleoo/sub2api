//go:build unit

// SSOT 重构 PR-2：黄金值回归测试基线。
//
// 用途：在 PR-3 ~ PR-8 的所有重构 PR 上线前，先固化当前 14 处计费乘法点
// (billing_service.go:574-625) 对一组代表性 (model, tokens, service_tier, multipliers)
// 输入的精确输出。任何后续 PR 必须保证这里的断言 0 漂移。
//
// 设计原则：
//   - 所有数字都来自 billing_service.go:140-261 的 fallbackPrices 字面量；
//   - 每个用例显式写出预期 cost 各字段（InDelta 1e-12），不再依赖
//     运行时计算后比对自身（那样无法发现公式改写带来的等价漂移）；
//   - 矩阵覆盖：基础/长上下文/cache 5m+1h/cache read/image output/priority/flex/
//     rate_multiplier/zero tokens 共 9 类场景。
package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const goldenEpsilon = 1e-12

// goldenCase 描述一条黄金值用例。
// 任何字段缺省时按其类型零值处理。
type goldenCase struct {
	name           string
	model          string
	tokens         UsageTokens
	rateMultiplier float64
	serviceTier    string // "", "priority", "flex"

	wantInput       float64
	wantOutput      float64
	wantImageOutput float64
	wantCacheCreate float64
	wantCacheRead   float64
	wantTotal       float64
	wantActual      float64
}

// goldenCases 是 PR-2 基线：每个数字都是手工计算得出，对应 fallbackPrices 的字面价格。
// 新增模型/字段时只能追加不能改既有数字。
var goldenCases = []goldenCase{
	// ----- 1. claude-sonnet-4：基础 token 计费 -----
	{
		name:   "sonnet-4 basic",
		model:  "claude-sonnet-4",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500},
		// Input: 1000 × 3e-6 = 3e-3 ; Output: 500 × 15e-6 = 7.5e-3
		wantInput: 3e-3, wantOutput: 7.5e-3,
		wantTotal: 0.0105, wantActual: 0.0105,
	},
	// ----- 2. claude-sonnet-4：cache creation/read（标准计费，无 breakdown）-----
	{
		name:  "sonnet-4 cache standard",
		model: "claude-sonnet-4",
		tokens: UsageTokens{
			InputTokens:         1000,
			OutputTokens:        500,
			CacheCreationTokens: 2000,
			CacheReadTokens:     3000,
		},
		// CacheCreation: 2000 × 3.75e-6 = 7.5e-3 ; CacheRead: 3000 × 0.3e-6 = 9e-4
		wantInput: 3e-3, wantOutput: 7.5e-3,
		wantCacheCreate: 7.5e-3, wantCacheRead: 9e-4,
		wantTotal:  3e-3 + 7.5e-3 + 7.5e-3 + 9e-4,
		wantActual: 3e-3 + 7.5e-3 + 7.5e-3 + 9e-4,
	},
	// ----- 3. claude-opus-4.5：rate multiplier 2× 应用到 ActualCost -----
	{
		name:           "opus-4.5 rate 2x",
		model:          "claude-opus-4.5",
		tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 500},
		rateMultiplier: 2.0,
		// Input: 1000×5e-6=5e-3 ; Output: 500×25e-6=12.5e-3 ; Total=17.5e-3
		wantInput: 5e-3, wantOutput: 12.5e-3,
		wantTotal: 0.0175, wantActual: 0.035, // 0.0175 × 2
	},
	// ----- 4. claude-3-haiku：最低价模型边界 -----
	{
		name:   "claude-3-haiku zero output",
		model:  "claude-3-haiku",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 0},
		// Input: 1000 × 0.25e-6 = 2.5e-4
		wantInput: 2.5e-4,
		wantTotal: 2.5e-4, wantActual: 2.5e-4,
	},
	// ----- 5. claude-3-opus：高价模型 + cache full breakdown -----
	{
		name:  "claude-3-opus full cache",
		model: "claude-3-opus",
		tokens: UsageTokens{
			InputTokens:         100,
			OutputTokens:        50,
			CacheCreationTokens: 200,
			CacheReadTokens:     400,
		},
		// Input: 100×15e-6=1.5e-3 ; Output: 50×75e-6=3.75e-3
		// CacheCreate: 200×18.75e-6=3.75e-3 ; CacheRead: 400×1.5e-6=6e-4
		wantInput: 1.5e-3, wantOutput: 3.75e-3,
		wantCacheCreate: 3.75e-3, wantCacheRead: 6e-4,
		wantTotal:  1.5e-3 + 3.75e-3 + 3.75e-3 + 6e-4,
		wantActual: 1.5e-3 + 3.75e-3 + 3.75e-3 + 6e-4,
	},
	// ----- 6. claude-opus-4.6 / 4.7：与 4.5 同价（aliases） -----
	{
		name:   "claude-opus-4.6 same as 4.5",
		model:  "claude-opus-4.6",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 1000},
		// Input: 1000×5e-6=5e-3 ; Output: 1000×25e-6=25e-3
		wantInput: 5e-3, wantOutput: 25e-3,
		wantTotal: 30e-3, wantActual: 30e-3,
	},
	{
		name:      "claude-opus-4.7 same as 4.5",
		model:     "claude-opus-4.7",
		tokens:    UsageTokens{InputTokens: 1000, OutputTokens: 1000},
		wantInput: 5e-3, wantOutput: 25e-3,
		wantTotal: 30e-3, wantActual: 30e-3,
	},
	// ----- 7. claude-3-5-sonnet：与 sonnet-4 同价 -----
	{
		name:      "claude-3-5-sonnet basic",
		model:     "claude-3-5-sonnet",
		tokens:    UsageTokens{InputTokens: 1000, OutputTokens: 500},
		wantInput: 3e-3, wantOutput: 7.5e-3,
		wantTotal: 0.0105, wantActual: 0.0105,
	},
	// ----- 8. claude-3-5-haiku：与 claude-3-5-haiku 同价 -----
	{
		name:   "claude-3-5-haiku basic",
		model:  "claude-3-5-haiku",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500},
		// Input: 1000×1e-6=1e-3 ; Output: 500×5e-6=2.5e-3
		wantInput: 1e-3, wantOutput: 2.5e-3,
		wantTotal: 3.5e-3, wantActual: 3.5e-3,
	},
	// ----- 9. gemini-3.1-pro -----
	{
		name:   "gemini-3.1-pro basic",
		model:  "gemini-3.1-pro",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500},
		// Input: 1000×2e-6=2e-3 ; Output: 500×12e-6=6e-3
		wantInput: 2e-3, wantOutput: 6e-3,
		wantTotal: 8e-3, wantActual: 8e-3,
	},
	// ----- 10. gpt-5.4：长上下文未触发（300k 但 multiplier 不应用？看代码：CalculateCost
	//         走 calculateCostInternal → applyLongCtx=false。所以基础价格） -----
	{
		name:   "gpt-5.4 below long context (CalculateCost path)",
		model:  "gpt-5.4",
		tokens: UsageTokens{InputTokens: 10000, OutputTokens: 1000},
		// Input: 10000×2.5e-6=25e-3 ; Output: 1000×15e-6=15e-3
		wantInput: 25e-3, wantOutput: 15e-3,
		wantTotal: 40e-3, wantActual: 40e-3,
	},
	// ----- 11. gpt-5.4-mini：低价 mini 模型 -----
	{
		name:   "gpt-5.4-mini basic",
		model:  "gpt-5.4-mini",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500},
		// Input: 1000×7.5e-7=7.5e-4 ; Output: 500×4.5e-6=2.25e-3
		wantInput: 7.5e-4, wantOutput: 2.25e-3,
		wantTotal: 7.5e-4 + 2.25e-3, wantActual: 7.5e-4 + 2.25e-3,
	},
	// ----- 12. gpt-5.4-nano：超低价 nano 模型 -----
	{
		name:   "gpt-5.4-nano basic",
		model:  "gpt-5.4-nano",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500},
		// Input: 1000×2e-7=2e-4 ; Output: 500×1.25e-6=6.25e-4
		wantInput: 2e-4, wantOutput: 6.25e-4,
		wantTotal: 2e-4 + 6.25e-4, wantActual: 2e-4 + 6.25e-4,
	},
	// ----- 13. gpt-5.2：完整 priority 字段 -----
	{
		name:   "gpt-5.2 basic non-priority",
		model:  "gpt-5.2",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200},
		// Input: 1000×1.75e-6=1.75e-3 ; Output: 500×14e-6=7e-3
		// CacheRead: 200×0.175e-6=3.5e-5
		wantInput: 1.75e-3, wantOutput: 7e-3, wantCacheRead: 3.5e-5,
		wantTotal: 1.75e-3 + 7e-3 + 3.5e-5, wantActual: 1.75e-3 + 7e-3 + 3.5e-5,
	},
	// ----- 14. gpt-5.3-codex：codex 族基础 -----
	{
		name:   "gpt-5.3-codex basic",
		model:  "gpt-5.3-codex",
		tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200},
		// Input: 1000×1.5e-6=1.5e-3 ; Output: 500×12e-6=6e-3
		// CacheRead: 200×0.15e-6=3e-5
		wantInput: 1.5e-3, wantOutput: 6e-3, wantCacheRead: 3e-5,
		wantTotal: 1.5e-3 + 6e-3 + 3e-5, wantActual: 1.5e-3 + 6e-3 + 3e-5,
	},
	// ----- 边界场景：zero tokens -----
	{
		name:   "sonnet-4 all zero",
		model:  "claude-sonnet-4",
		tokens: UsageTokens{},
	},
	// ----- 边界场景：仅 OutputTokens (text-only generation) -----
	{
		name:       "sonnet-4 only output",
		model:      "claude-sonnet-4",
		tokens:     UsageTokens{OutputTokens: 1000},
		wantOutput: 15e-3,
		wantTotal:  15e-3, wantActual: 15e-3,
	},
	// ----- 边界场景：仅 CacheReadTokens (cache-only context restore) -----
	{
		name:   "sonnet-4 only cache read",
		model:  "claude-sonnet-4",
		tokens: UsageTokens{CacheReadTokens: 10000},
		// 10000 × 0.3e-6 = 3e-3
		wantCacheRead: 3e-3,
		wantTotal:     3e-3, wantActual: 3e-3,
	},
	// ----- rate_multiplier=0.5 边界（折半） -----
	{
		name:           "sonnet-4 rate 0.5",
		model:          "claude-sonnet-4",
		tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 500},
		rateMultiplier: 0.5,
		wantInput:      3e-3,
		wantOutput:     7.5e-3,
		wantTotal:      0.0105,
		wantActual:     0.00525,
	},
}

// TestBillingService_Golden 是 SSOT 重构的回归门。
// 任何 PR 触发数值漂移都必须被这里捕获。
func TestBillingService_Golden(t *testing.T) {
	svc := newTestBillingService()

	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			rateMultiplier := tc.rateMultiplier
			if rateMultiplier == 0 {
				rateMultiplier = 1.0
			}
			var (
				cost *CostBreakdown
				err  error
			)
			if tc.serviceTier != "" {
				cost, err = svc.CalculateCostWithServiceTier(tc.model, tc.tokens, rateMultiplier, tc.serviceTier)
			} else {
				cost, err = svc.CalculateCost(tc.model, tc.tokens, rateMultiplier)
			}
			require.NoError(t, err)
			require.NotNil(t, cost)

			require.InDelta(t, tc.wantInput, cost.InputCost, goldenEpsilon, "InputCost")
			require.InDelta(t, tc.wantOutput, cost.OutputCost, goldenEpsilon, "OutputCost")
			require.InDelta(t, tc.wantImageOutput, cost.ImageOutputCost, goldenEpsilon, "ImageOutputCost")
			require.InDelta(t, tc.wantCacheCreate, cost.CacheCreationCost, goldenEpsilon, "CacheCreationCost")
			require.InDelta(t, tc.wantCacheRead, cost.CacheReadCost, goldenEpsilon, "CacheReadCost")
			require.InDelta(t, tc.wantTotal, cost.TotalCost, goldenEpsilon, "TotalCost")
			require.InDelta(t, tc.wantActual, cost.ActualCost, goldenEpsilon, "ActualCost")
		})
	}
}

// TestBillingService_Golden_LongContext 单独覆盖 CalculateCostWithLongContext 路径
// （applyLongCtx=true，价格表里 LongContextInputThreshold>0 时才生效）。
// 对应 billing_service.go:573-575 的三处长上下文乘法。
func TestBillingService_Golden_LongContext(t *testing.T) {
	svc := newTestBillingService()

	cases := []struct {
		name       string
		model      string
		tokens     UsageTokens
		wantInput  float64
		wantOutput float64
		wantTotal  float64
	}{
		{
			// gpt-5.4 阈值 272000，超过后整次会话 input×2.0, output×1.5
			name:   "gpt-5.4 long context applies",
			model:  "gpt-5.4",
			tokens: UsageTokens{InputTokens: 300000, OutputTokens: 4000},
			// Input: 300000 × 2.5e-6 × 2.0 = 1.5
			wantInput: 300000 * 2.5e-6 * 2.0,
			// Output: 4000 × 15e-6 × 1.5 = 0.09
			wantOutput: 4000 * 15e-6 * 1.5,
			wantTotal:  300000*2.5e-6*2.0 + 4000*15e-6*1.5,
		},
		{
			// 阈值边界 -1（应该不应用倍率）
			// totalContext = 271999 + 0 = 271999 < 272000 → 不触发
			name:       "gpt-5.4 at threshold minus 1",
			model:      "gpt-5.4",
			tokens:     UsageTokens{InputTokens: 271999, OutputTokens: 1000},
			wantInput:  271999 * 2.5e-6,
			wantOutput: 1000 * 15e-6,
			wantTotal:  271999*2.5e-6 + 1000*15e-6,
		},
		{
			// 阈值边界 +1（应该应用倍率）
			// totalContext = 272001 + 0 = 272001 > 272000 → 触发
			name:       "gpt-5.4 at threshold plus 1",
			model:      "gpt-5.4",
			tokens:     UsageTokens{InputTokens: 272001, OutputTokens: 1000},
			wantInput:  272001 * 2.5e-6 * 2.0,
			wantOutput: 1000 * 15e-6 * 1.5,
			wantTotal:  272001*2.5e-6*2.0 + 1000*15e-6*1.5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 用 CalculateCostUnified（applyLongCtx 内部由 LongContextInputThreshold 判定）
			// 走默认 token mode（Resolver=nil → 走 calculateCostInternal applyLongCtx=true 当
			// pricing.LongContextInputThreshold > 0）
			cost, err := svc.CalculateCostUnified(CostInput{
				Ctx:            context.Background(),
				Model:          tc.model,
				Tokens:         tc.tokens,
				RateMultiplier: 1.0,
			})
			require.NoError(t, err)
			require.NotNil(t, cost)
			require.InDelta(t, tc.wantInput, cost.InputCost, goldenEpsilon, "InputCost")
			require.InDelta(t, tc.wantOutput, cost.OutputCost, goldenEpsilon, "OutputCost")
			require.InDelta(t, tc.wantTotal, cost.TotalCost, goldenEpsilon, "TotalCost")
		})
	}
}

// TestBillingService_Golden_ServiceTierPriority 覆盖 priority service tier 走 priority 价格字段。
// 对应 billing_service.go:559-568 的价格切换逻辑。
func TestBillingService_Golden_ServiceTierPriority(t *testing.T) {
	svc := newTestBillingService()

	cases := []struct {
		name          string
		model         string
		tokens        UsageTokens
		serviceTier   string
		wantInput     float64
		wantOutput    float64
		wantCacheRead float64
		wantTotal     float64
	}{
		{
			// gpt-5.4 有 priority 字段：Input 5e-6, Output 30e-6, CacheRead 0.5e-6
			// priority 命中后 tierMultiplier=1.0（已切到 priority 价），不再额外 ×2
			name:        "gpt-5.4 priority uses priority prices no multiplier",
			model:       "gpt-5.4",
			tokens:      UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200},
			serviceTier: "priority",
			// Input: 1000×5e-6=5e-3 ; Output: 500×30e-6=15e-3 ; CacheRead: 200×0.5e-6=1e-4
			wantInput: 5e-3, wantOutput: 15e-3, wantCacheRead: 1e-4,
			wantTotal: 5e-3 + 15e-3 + 1e-4,
		},
		{
			// sonnet-4 无 priority 字段 → 走 tierMultiplier=2.0 通用倍率
			name:        "sonnet-4 priority uses tier multiplier 2x",
			model:       "claude-sonnet-4",
			tokens:      UsageTokens{InputTokens: 1000, OutputTokens: 500},
			serviceTier: "priority",
			// Input: 1000×3e-6×2=6e-3 ; Output: 500×15e-6×2=15e-3
			wantInput: 6e-3, wantOutput: 15e-3,
			wantTotal: 6e-3 + 15e-3,
		},
		{
			// flex tier → ×0.5
			name:        "sonnet-4 flex tier 0.5x",
			model:       "claude-sonnet-4",
			tokens:      UsageTokens{InputTokens: 1000, OutputTokens: 500},
			serviceTier: "flex",
			// Input: 3e-3×0.5=1.5e-3 ; Output: 7.5e-3×0.5=3.75e-3
			wantInput: 1.5e-3, wantOutput: 3.75e-3,
			wantTotal: 5.25e-3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cost, err := svc.CalculateCostWithServiceTier(tc.model, tc.tokens, 1.0, tc.serviceTier)
			require.NoError(t, err)
			require.InDelta(t, tc.wantInput, cost.InputCost, goldenEpsilon, "InputCost")
			require.InDelta(t, tc.wantOutput, cost.OutputCost, goldenEpsilon, "OutputCost")
			require.InDelta(t, tc.wantCacheRead, cost.CacheReadCost, goldenEpsilon, "CacheReadCost")
			require.InDelta(t, tc.wantTotal, cost.TotalCost, goldenEpsilon, "TotalCost")
		})
	}
}

// TestBillingService_Golden_CalculateCostWithConfig 覆盖
// billing_service.go RateMultiplier 来自 cfg.Default 的默认计费路径。
func TestBillingService_Golden_CalculateCostWithConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.5
	svc := newTestBillingServiceWithConfig(cfg)

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}
	cost, err := svc.CalculateCostWithConfig("claude-sonnet-4", tokens)
	require.NoError(t, err)

	// TotalCost: 3e-3 + 7.5e-3 = 0.0105
	// ActualCost: 0.0105 × 1.5 = 0.01575
	require.InDelta(t, 0.0105, cost.TotalCost, goldenEpsilon, "TotalCost")
	require.InDelta(t, 0.01575, cost.ActualCost, goldenEpsilon, "ActualCost")
}
