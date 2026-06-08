package service

import (
	"regexp"
	"strings"
)

// 用户广场可见性规则（从前端 ModelsView.shouldShowModel + baseModels 海外过滤下沉到后端，
// 使后端口径 = 用户实际所见，避免前后端各算一套）。
//
// 规则：
//   - 全局海外开关 show_overseas_models=false 时，海外模型整体隐藏。
//   - 图片/视频模型不做版本下限。
//   - 国产（非海外前缀）模型始终显示。
//   - 海外文本模型按版本下限：Claude≥4.5 / GPT≥5.2 / Gemini≥3，其余海外隐藏。

// isImageOrVideoModel 与前端 isImageModel 同步（含视频）：mode 为图/视频，或 id 命中图片前缀。
func isImageOrVideoModel(m ModelInfo) bool {
	mode := strings.ToLower(m.Mode)
	if mode == "image" || mode == "image_generation" || mode == "video_generation" {
		return true
	}
	id := strings.ToLower(m.ID)
	switch {
	case strings.HasPrefix(id, "gpt-image-"),
		strings.HasPrefix(id, "dall-e"),
		strings.HasPrefix(id, "imagen-"),
		strings.HasPrefix(id, "gemini-") && strings.Contains(id, "-image"),
		strings.HasPrefix(id, "grok-") && strings.Contains(id, "image"):
		return true
	}
	return false
}

// parseClaudeVersion: claude-(?:\w+-)?(\d+)([-.](\d+))? → major + minor/10。
var claudeVersionRE = regexp.MustCompile(`claude-(?:\w+-)?(\d+)(?:[-.](\d+))?`)

func parseClaudeVersion(id string) float64 { return parseVersionWith(claudeVersionRE, id) }

// parseGPTVersion: gpt-(\d+)(\.(\d+))? ；o1~o4 视为旧版返回 0。
var gptVersionRE = regexp.MustCompile(`gpt-(\d+)(?:\.(\d+))?`)
var oSeriesRE = regexp.MustCompile(`^o[1-4]`)

func parseGPTVersion(id string) float64 {
	if v := parseVersionWith(gptVersionRE, id); v > 0 {
		return v
	}
	if oSeriesRE.MatchString(id) {
		return 0
	}
	return 0
}

// parseGeminiVersion: gemini-(\d+)(\.(\d+))?。
var geminiVersionRE = regexp.MustCompile(`gemini-(\d+)(?:\.(\d+))?`)

func parseGeminiVersion(id string) float64 { return parseVersionWith(geminiVersionRE, id) }

func parseVersionWith(re *regexp.Regexp, id string) float64 {
	m := re.FindStringSubmatch(strings.ToLower(id))
	if m == nil {
		return 0
	}
	major := atoiSafe(m[1])
	minor := 0
	if len(m) > 2 && m[2] != "" {
		minor = atoiSafe(m[2])
	}
	return float64(major) + float64(minor)/10
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// isModelVisibleToUser 综合两层规则，返回该模型是否应出现在用户广场。
func isModelVisibleToUser(m ModelInfo, showOverseas bool) bool {
	overseas := IsOverseasModelID(m.ID)
	// 第一层：全局海外开关
	if overseas && !showOverseas {
		return false
	}
	// 第二层：图片/视频不做版本下限
	if isImageOrVideoModel(m) {
		return true
	}
	// 国产模型始终显示
	if !overseas {
		return true
	}
	// 海外文本模型版本下限
	id := strings.ToLower(m.ID)
	provider := strings.ToLower(m.LiteLLMProvider)
	switch {
	case provider == "anthropic" || strings.HasPrefix(id, "claude-"):
		return parseClaudeVersion(id) >= 4.5
	case provider == "openai" || strings.HasPrefix(id, "gpt-") || oSeriesRE.MatchString(id):
		return parseGPTVersion(id) >= 5.2
	case provider == "google" || strings.HasPrefix(id, "gemini-"):
		return parseGeminiVersion(id) >= 3
	default:
		return false
	}
}

// FilterVisibleModels 对用户可路由的模型列表应用广场可见性规则。
func FilterVisibleModels(models []ModelInfo, showOverseas bool) []ModelInfo {
	out := make([]ModelInfo, 0, len(models))
	for _, m := range models {
		if isModelVisibleToUser(m, showOverseas) {
			out = append(out, m)
		}
	}
	return out
}
