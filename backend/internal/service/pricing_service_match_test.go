// SSOT 重构 PR-8：matchFamilyInCatalog 家族锚点回归测试。
//
// 用途：固化 PricingService.matchFamilyInCatalog 对每个 Claude 家族锚点的命中关系。
// 与 PR-2 的 matchByModelFamily 测试等价，数据源从 pricingData 改为 catalog。
package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// catalogEntry 快速构造最小 DBModelPricing 锚点。
func catalogEntry(modelID string, input, output float64) *DBModelPricing {
	return &DBModelPricing{ModelID: modelID, InputCostPerToken: &input, OutputCostPerToken: &output}
}

// TestMatchFamilyInCatalog_GoldenAnchors 固化 matchFamilyInCatalog 对 10 个 Claude 家族锚点的命中表。
func TestMatchFamilyInCatalog_GoldenAnchors(t *testing.T) {
	cases := []struct {
		family         string
		catalogMap     map[string]*DBModelPricing
		variants       []string
		wantModelID    string
		wantInputPrice float64
	}{
		{
			family: "opus-4.7",
			catalogMap: map[string]*DBModelPricing{
				"claude-opus-4-7-20251101": catalogEntry("claude-opus-4-7-20251101", 5e-6, 25e-6),
			},
			variants:       []string{"claude-opus-4-7", "claude-opus-4.7-20251201"},
			wantModelID:    "claude-opus-4-7-20251101",
			wantInputPrice: 5e-6,
		},
		{
			family: "opus-4.5",
			catalogMap: map[string]*DBModelPricing{
				"claude-opus-4-5-20251101": catalogEntry("claude-opus-4-5-20251101", 5e-6, 25e-6),
			},
			variants:       []string{"claude-opus-4-5", "claude-opus-4.5-20260301"},
			wantModelID:    "claude-opus-4-5-20251101",
			wantInputPrice: 5e-6,
		},
		{
			family: "sonnet-4.5",
			catalogMap: map[string]*DBModelPricing{
				"claude-sonnet-4-5-20250514": catalogEntry("claude-sonnet-4-5-20250514", 3e-6, 15e-6),
			},
			variants:       []string{"claude-sonnet-4-5", "claude-sonnet-4.5-latest"},
			wantModelID:    "claude-sonnet-4-5-20250514",
			wantInputPrice: 3e-6,
		},
		{
			family: "sonnet-4",
			catalogMap: map[string]*DBModelPricing{
				"claude-sonnet-4-20250514": catalogEntry("claude-sonnet-4-20250514", 3e-6, 15e-6),
			},
			variants:       []string{"claude-sonnet-4-20251001", "claude-sonnet-4"},
			wantModelID:    "claude-sonnet-4-20250514",
			wantInputPrice: 3e-6,
		},
		{
			family: "haiku-3.5",
			catalogMap: map[string]*DBModelPricing{
				"claude-3-5-haiku-20241022": catalogEntry("claude-3-5-haiku-20241022", 1e-6, 5e-6),
			},
			variants:       []string{"claude-3-5-haiku", "claude-3.5-haiku-20260301"},
			wantModelID:    "claude-3-5-haiku-20241022",
			wantInputPrice: 1e-6,
		},
		{
			family: "haiku-3",
			catalogMap: map[string]*DBModelPricing{
				"claude-3-haiku-20240307": catalogEntry("claude-3-haiku-20240307", 0.25e-6, 1.25e-6),
			},
			variants:       []string{"claude-3-haiku", "claude-3-haiku-foo"},
			wantModelID:    "claude-3-haiku-20240307",
			wantInputPrice: 0.25e-6,
		},
	}

	for _, tc := range cases {
		svc := &PricingService{
			catalog:  tc.catalogMap,
			aliasIdx: buildAliasIndex(tc.catalogMap),
		}
		for _, model := range tc.variants {
			t.Run(tc.family+"/"+model, func(t *testing.T) {
				got := svc.matchFamilyInCatalog(model)
				require.NotNil(t, got, "family=%s variant=%s", tc.family, model)
				require.Equal(t, tc.wantModelID, got.ModelID,
					"family=%s variant=%s modelID", tc.family, model)
				require.InDelta(t, tc.wantInputPrice, *got.InputCostPerToken, 1e-12,
					"family=%s variant=%s input price", tc.family, model)
			})
		}
	}
}

// TestMatchFamilyInCatalog_AnchorMissingReturnsNil 验证家族锚点缺失时返回 nil。
func TestMatchFamilyInCatalog_AnchorMissingReturnsNil(t *testing.T) {
	svc := &PricingService{
		catalog:  map[string]*DBModelPricing{},
		aliasIdx: map[string]string{},
	}

	require.Nil(t, svc.matchFamilyInCatalog("claude-opus-4-7-20260101"))
	require.Nil(t, svc.matchFamilyInCatalog("claude-sonnet-4-5-latest"))
	require.Nil(t, svc.matchFamilyInCatalog("gpt-4o"), "non-Claude model must return nil")
}

// TestMatchFamilyInCatalog_Phase1HighVersionFirst 验证 opus-4-7 被识别为 opus-4.7 family 而不是 opus-4。
func TestMatchFamilyInCatalog_Phase1HighVersionFirst(t *testing.T) {
	opus47Entry := catalogEntry("claude-opus-4-7-20251101", 7e-6, 35e-6)
	svc := &PricingService{
		catalog:  map[string]*DBModelPricing{"claude-opus-4-7-20251101": opus47Entry},
		aliasIdx: buildAliasIndex(map[string]*DBModelPricing{"claude-opus-4-7-20251101": opus47Entry}),
	}

	got := svc.matchFamilyInCatalog("claude-opus-4-7-20251201")
	require.NotNil(t, got)
	require.Equal(t, "claude-opus-4-7-20251101", got.ModelID)
}

// TestBuildAliasIndex_DotDashVariants 验证 A5 点/横杠变体别名：
// doubao-seedance 点号/横杠拼写互为别名命中同一 canonical，且真实模型不被变体遮蔽。
func TestBuildAliasIndex_DotDashVariants(t *testing.T) {
	catalog := map[string]*DBModelPricing{
		"Doubao-Seedance-1.5-pro": catalogEntry("Doubao-Seedance-1.5-pro", 0, 0),
		"gpt-4.1":                 catalogEntry("gpt-4.1", 1e-6, 2e-6),
		"gpt-4-1":                 catalogEntry("gpt-4-1", 3e-6, 4e-6), // 真实横杠形，不应被 gpt-4.1 的变体遮蔽
	}
	idx := buildAliasIndex(catalog)

	// 点/横杠变体都归一到 canonical（小写自身形 + 横杠变体）
	require.Equal(t, "Doubao-Seedance-1.5-pro", idx["doubao-seedance-1.5-pro"])
	require.Equal(t, "Doubao-Seedance-1.5-pro", idx["doubao-seedance-1-5-pro"])

	// 真实模型保留自身映射，变体守卫生效（不互相遮蔽）
	require.Equal(t, "gpt-4-1", idx["gpt-4-1"])
	require.Equal(t, "gpt-4.1", idx["gpt-4.1"])
}
