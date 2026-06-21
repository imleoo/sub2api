package service

import "strings"

// NormalizeProvider 把 provider 输入规范化成 provider_key（Phase 1 P1-1 引入）。
//
// 规则（docs/glossary.md §1.3 单一权威源）：
//  1. strings.ToLower + strings.TrimSpace
//  2. 别名合并（见下方 providerAliases 表）
//  3. 别名未命中时返回小写去空白后的原值
//
// 新增 provider 必须先 PR 扩 glossary §1.3 别名表，再扩 providerAliases，
// 否则统计页会按未规范化字符串切分。
func NormalizeProvider(s string) string {
	key := strings.ToLower(strings.TrimSpace(s))
	if key == "" {
		return ""
	}
	if canonical, ok := providerAliases[key]; ok {
		return canonical
	}
	return key
}

// providerAliases 是 provider 别名 → 规范键的映射（docs/glossary.md §1.3）。
//
// 维护规约：新增条目必须先改 glossary.md，再加这里，再上线写入路径。
var providerAliases = map[string]string{
	// deepseek 系
	"deepseek":          "deepseek",
	"deep-seek":         "deepseek",
	"deepseek-official": "deepseek",
	"deepseekofficial":  "deepseek",

	// siliconflow 系（含中文）
	"siliconflow":  "siliconflow",
	"silicon-flow": "siliconflow",
	"silicon_flow": "siliconflow",
	"硅基流动":         "siliconflow",

	// doubao 系（火山引擎豆包，含中文）
	"doubao":  "doubao",
	"dou-bao": "doubao",
	"dou_bao": "doubao",
	"豆包":      "doubao",

	// kimi / moonshot
	"kimi":     "kimi",
	"moonshot": "kimi",

	// qwen（通义千问）
	"qwen":      "qwen",
	"qwen-plus": "qwen",
	"qwen_plus": "qwen",
	"通义千问":      "qwen",

	// wanjie（万界方舟，含中文）
	"wanjie":  "wanjie",
	"wan-jie": "wanjie",
	"wan_jie": "wanjie",
	"万界方舟":    "wanjie",

	// 内置 4 平台（fork 12 lingjing 已在 platform 常量中，不需再走别名）
	"anthropic": "anthropic",
	"openai":    "openai",
	"gemini":    "gemini",
	"google":    "gemini",
}
