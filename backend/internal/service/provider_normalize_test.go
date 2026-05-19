package service

import "testing"

// TestNormalizeProvider 验证 glossary.md §1.3 别名表的所有规范化规则。
func TestNormalizeProvider(t *testing.T) {
	cases := []struct {
		name   string
		inputs []string // 同一规范键的多种输入形态
		want   string
	}{
		{
			name:   "deepseek 系",
			inputs: []string{"DeepSeek", "deep-seek", "deepseek-official", "  DEEPSEEK  ", "deepseek"},
			want:   "deepseek",
		},
		{
			name:   "siliconflow 含中文",
			inputs: []string{"SiliconFlow", "silicon-flow", "硅基流动", "silicon_flow"},
			want:   "siliconflow",
		},
		{
			name:   "doubao 含中文",
			inputs: []string{"Doubao", "dou-bao", "豆包", "DOUBAO"},
			want:   "doubao",
		},
		{
			name:   "kimi / moonshot 同义",
			inputs: []string{"Kimi", "moonshot", "MOONSHOT", "kimi"},
			want:   "kimi",
		},
		{
			name:   "qwen 系",
			inputs: []string{"Qwen", "qwen-plus", "通义千问"},
			want:   "qwen",
		},
		{
			name:   "wanjie 含中文",
			inputs: []string{"Wanjie", "wan-jie", "万界方舟"},
			want:   "wanjie",
		},
		{
			name:   "anthropic 官方",
			inputs: []string{"Anthropic", "ANTHROPIC", "anthropic"},
			want:   "anthropic",
		},
		{
			name:   "openai 官方",
			inputs: []string{"OpenAI", "openai", "  OPENAI  "},
			want:   "openai",
		},
		{
			name:   "gemini / google 同义",
			inputs: []string{"Gemini", "Google", "GOOGLE"},
			want:   "gemini",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, in := range tc.inputs {
				if got := NormalizeProvider(in); got != tc.want {
					t.Errorf("NormalizeProvider(%q) = %q, want %q", in, got, tc.want)
				}
			}
		})
	}
}

// TestNormalizeProvider_EmptyInput 空输入返回空串（用于 NULL 写入路径）。
func TestNormalizeProvider_EmptyInput(t *testing.T) {
	cases := []string{"", "   ", "\t\n"}
	for _, in := range cases {
		if got := NormalizeProvider(in); got != "" {
			t.Errorf("NormalizeProvider(%q) = %q, want \"\"", in, got)
		}
	}
}

// TestNormalizeProvider_UnknownPassesThrough 未登记别名直接返回小写去空白后的原值。
//
// 这是关键的兜底行为：避免新接入的 provider 因别名未补就丢数据，
// 但小写规范化是保留的（防止 'CustomProvider' 与 'customprovider' 切两个桶）。
func TestNormalizeProvider_UnknownPassesThrough(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"NewProvider", "newprovider"},
		{"  custom_provider  ", "custom_provider"},
		{"未登记厂商", "未登记厂商"},
	}
	for _, c := range cases {
		if got := NormalizeProvider(c.in); got != c.want {
			t.Errorf("NormalizeProvider(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizeProvider_GlossaryParity 用一组 glossary.md §1.3 表格的输入，
// 完整断言别名表与文档同步（守护文档与代码的同步性）。
func TestNormalizeProvider_GlossaryParity(t *testing.T) {
	glossaryExamples := map[string]string{
		// docs/glossary.md §1.3 第一行：DeepSeek/deep-seek/deepseek-official → deepseek
		"DeepSeek":          "deepseek",
		"deep-seek":         "deepseek",
		"deepseek-official": "deepseek",
		// 第二行：SiliconFlow/silicon-flow/硅基流动 → siliconflow
		"SiliconFlow":  "siliconflow",
		"silicon-flow": "siliconflow",
		"硅基流动":         "siliconflow",
		// 第三行：Doubao/dou-bao/豆包 → doubao
		"Doubao":  "doubao",
		"dou-bao": "doubao",
		"豆包":      "doubao",
		// 第四行：Kimi/moonshot → kimi
		"Kimi":     "kimi",
		"moonshot": "kimi",
		// 第五行：Qwen/qwen-plus → qwen
		"Qwen":      "qwen",
		"qwen-plus": "qwen",
		// 第六行：Wanjie/wan-jie/万界方舟 → wanjie
		"Wanjie":  "wanjie",
		"wan-jie": "wanjie",
		"万界方舟":    "wanjie",
		// 第七行：Anthropic（官方）→ anthropic
		"Anthropic": "anthropic",
		// 第八行：OpenAI（官方）→ openai
		"OpenAI": "openai",
		// 第九行：Gemini/Google → gemini
		"Gemini": "gemini",
		"Google": "gemini",
	}
	for in, want := range glossaryExamples {
		if got := NormalizeProvider(in); got != want {
			t.Errorf("glossary parity broken: NormalizeProvider(%q) = %q, want %q", in, got, want)
		}
	}
}
