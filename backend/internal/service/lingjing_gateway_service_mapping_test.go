package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLingjingModelCatalog_LookupAllKnownModels 遍历 lingjingModelCatalog 里的每一个
// 已知模型名，验证 lookupLingjingModel 精确匹配返回 canonical 名与完全一致的 entry。
// 这是计费+网关分流的唯一真源，任何一条映射被误改都必须在这里炸掉。
func TestLingjingModelCatalog_LookupAllKnownModels(t *testing.T) {
	require.NotEmpty(t, lingjingModelCatalog, "lingjingModelCatalog must not be empty")

	for model, want := range lingjingModelCatalog {
		want := want
		t.Run(model, func(t *testing.T) {
			canonical, entry := lookupLingjingModel(model)
			require.NotNil(t, entry, "expected catalog entry for %q", model)
			assert.Equal(t, model, canonical, "exact match must return the same canonical name")
			assert.Equal(t, want, *entry, "returned entry must match catalog definition for %q", model)
		})
	}
}

// TestLingjingModelCatalog_ModelCount 锁定当前已知模型总数（27 条，见风险表），
// 防止有人悄悄增删条目而没人注意到——数量变化时必须显式更新本测试并复核 migration。
func TestLingjingModelCatalog_ModelCount(t *testing.T) {
	assert.Len(t, lingjingModelCatalog, 27, "lingjingModelCatalog entry count changed — cross-check migrations 151/152_*.sql before updating this assertion")
}

// TestLingjingModelAliasToCanonical_ResolvesToRealCatalogEntry 确保别名表里的每一个
// canonical 目标都真实存在于 catalog 中（不存在的目标会导致 lookupLingjingModel 静默返回 nil）。
func TestLingjingModelAliasToCanonical_ResolvesToRealCatalogEntry(t *testing.T) {
	require.NotEmpty(t, lingjingModelAliasToCanonical)

	for alias, canonical := range lingjingModelAliasToCanonical {
		alias, canonical := alias, canonical
		t.Run(alias, func(t *testing.T) {
			want, ok := lingjingModelCatalog[canonical]
			require.True(t, ok, "alias %q points to canonical %q which is missing from lingjingModelCatalog", alias, canonical)

			gotCanonical, entry := lookupLingjingModel(alias)
			require.NotNil(t, entry)
			assert.Equal(t, canonical, gotCanonical)
			assert.Equal(t, want, *entry)
		})
	}
}

// TestLookupLingjingModel_CaseInsensitiveFallback 验证大小写不敏感的模糊匹配分支
// （catalog 与 alias 表都精确匹配失败之后才会走到）。
func TestLookupLingjingModel_CaseInsensitiveFallback(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantCanonical string
	}{
		{"catalog key uppercased", strings.ToUpper("kling-v2-5-turbo"), "kling-v2-5-turbo"},
		{"catalog key lowercased when original mixed-case", strings.ToLower("MiniMax-Hailuo-02"), "MiniMax-Hailuo-02"},
		{"alias uppercased", strings.ToUpper("seedream4.0"), "doubao-seedream-4-0-250828"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			canonical, entry := lookupLingjingModel(tt.input)
			require.NotNil(t, entry, "expected fuzzy match for %q", tt.input)
			assert.Equal(t, tt.wantCanonical, canonical)
		})
	}
}

// TestLookupLingjingModel_UnknownModel 未识别模型名必须返回 ("", nil)，
// 调用方（lingjingModelToAPIID / isLingjingVideoModel 等）依赖这一点做兜底判断。
func TestLookupLingjingModel_UnknownModel(t *testing.T) {
	canonical, entry := lookupLingjingModel("this-model-does-not-exist-anywhere")
	assert.Empty(t, canonical)
	assert.Nil(t, entry)
}

// TestLingjingModelToAPIID 验证图片生成模型走各自的 PictureAPIID，
// 纯视频模型（无 PictureAPIID）与未识别模型统一兜底到 Seedream 4.5。
func TestLingjingModelToAPIID(t *testing.T) {
	tests := []struct {
		name   string
		model  string
		wantID string
	}{
		{"pure image-gen model returns its own PictureAPIID", "kling-v2", LingjingAPIIDKlingV2Pic},
		{"image-01 returns its own PictureAPIID", "image-01", LingjingAPIIDHailuoImage01},
		{"video-only model without PictureAPIID falls back to Seedream 4.5", "kling-v2-5-turbo", LingjingAPIIDSeedream45},
		{"unknown model falls back to Seedream 4.5", "totally-unknown-model", LingjingAPIIDSeedream45},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantID, lingjingModelToAPIID(tt.model))
		})
	}
}

// TestLingjingVideoAPIInfo 验证 Seedance 文生/图生视频两条路径的 apiId + modelName。
func TestLingjingVideoAPIInfo(t *testing.T) {
	tests := []struct {
		name      string
		taskType  string
		wantAPIID string
		wantModel string
	}{
		{"text2video", LingjingTaskTypeText2Video, LingjingAPIIDTextToVideo, "Doubao-Seedance-1.5-pro"},
		{"image2video", LingjingTaskTypeImage2Video, LingjingAPIIDImageToVideo, "Doubao-Seedance-1.5-pro"},
		{"unknown task type defaults to text2video branch", "some-other-type", LingjingAPIIDTextToVideo, "Doubao-Seedance-1.5-pro"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			apiID, modelName := lingjingVideoAPIInfo(tt.taskType)
			assert.Equal(t, tt.wantAPIID, apiID)
			assert.Equal(t, tt.wantModel, modelName)
		})
	}
}

// TestIsLingjingVideoModel 视频模型判断的边界：已知视频模型 true，已知纯图片模型 false，
// 未识别模型保守按视频处理（true）——避免账号测试用无意义 prompt 卡住京东云队列 5min+。
func TestIsLingjingVideoModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"known video model", "kling-v2-5-turbo", true},
		{"known video model with multiple routes", "kling-video-o1", true},
		{"known pure image-gen model", "kling-v2", false},
		{"known pure image-gen model image-01", "image-01", false},
		{"known pure image-gen model doubao seedream", "doubao-seedream-4-0-250828", false},
		{"unknown model defaults to video (conservative)", "unknown-model-xyz", true},
		{"alias of a video model resolves through alias table", "kling-v3", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isLingjingVideoModel(tt.model))
		})
	}
}
