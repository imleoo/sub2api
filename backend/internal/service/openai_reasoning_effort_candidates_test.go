package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractOpenAIReasoningEffortFromBodyModelCandidates(t *testing.T) {
	bodyWithoutEffort := []byte(`{"model":"whatever","input":"hello"}`)
	bodyWithMax := []byte(`{"model":"sol","reasoning":{"effort":"max"},"input":"hello"}`)

	tests := []struct {
		name       string
		body       []byte
		candidates []string
		want       string // "" 表示期望 nil
	}{
		{
			name:       "后缀推导回退到原始模型（OAuth 上游模型已剥后缀）",
			body:       bodyWithoutEffort,
			candidates: []string{"gpt-5.4", "gpt-5.4", "gpt-5.4-xhigh"},
			want:       "xhigh",
		},
		{
			name:       "GPT-5.6 后缀 max 经原始模型推导保留",
			body:       bodyWithoutEffort,
			candidates: []string{"gpt-5.6-sol", "gpt-5.6-sol", "gpt-5.6-sol-max"},
			want:       "max",
		},
		{
			name:       "显式 max 用第一个非空候选（映射后模型）判定",
			body:       bodyWithMax,
			candidates: []string{"gpt-5.6-sol", "sol"},
			want:       "max",
		},
		{
			name:       "显式 max 非 5.6 首候选仍折叠为 xhigh",
			body:       bodyWithMax,
			candidates: []string{"gpt-5.4", "sol"},
			want:       "xhigh",
		},
		{
			name:       "所有候选均无后缀时返回 nil",
			body:       bodyWithoutEffort,
			candidates: []string{"gpt-5.4", "gpt-5.4", "gpt-5.4"},
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOpenAIReasoningEffortFromBody(tt.body, tt.candidates...)
			if tt.want == "" {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.want, *got)
		})
	}
}

func TestExtractOpenAIReasoningEffortModelCandidates(t *testing.T) {
	reqBody := map[string]any{"model": "gpt-5.3-codex-high", "input": "hello"}

	got := extractOpenAIReasoningEffort(reqBody, "gpt-5.3-codex", "gpt-5.3-codex-high")

	require.NotNil(t, got)
	require.Equal(t, "high", *got)
}

// 回归：OAuth 账号请求后缀式模型（无显式 reasoning 字段）时，上游模型被
// normalizeCodexModel 剥掉 effort 后缀，用量元数据的 effort 必须仍能从
// 原始模型名后缀推导出来。
