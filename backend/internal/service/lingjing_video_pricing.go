package service

import "strings"

// 视频按官方 token 公式计费（通用，不内置任何价格）。
// 分档单价从 model_pricings.tier_pricing 读取（运营上传的定价 JSON 解析而来），此处只负责：
//   - token 公式：token 数 = 视频宽 × 高 × 帧率 × 时长，按 32×32 patch 分块。
//   - 按请求（分辨率/有声）选档，× ¥/Mtoken ÷ 汇率。
//
// token 公式校准：官方示例 720p 24fps 15s = 308,880 tokens = patch 网格 39×22(=858/帧) × 24fps × 15s。
const lingjingVideoFPS = 24

// lingjingVideoTokensPerFrame 各分辨率档每帧 token 数（32×32 patch 网格）。
//   - 720p 由官方示例精确校准（39×22=858）。
//   - 480p / 1080p 按同一 32-patch 法推算，待逐档核对。
var lingjingVideoTokensPerFrame = map[string]int{
	"480p":  405,
	"720p":  858,
	"1080p": 2040,
}

// lingjingVideoTokens 按官方公式估算 token：tokens/帧 × fps × 时长（秒），mode 未知按 720p 兜底。
func lingjingVideoTokens(mode string, durationSec float64) float64 {
	grid, ok := lingjingVideoTokensPerFrame[strings.ToLower(strings.TrimSpace(mode))]
	if !ok {
		grid = lingjingVideoTokensPerFrame["720p"]
	}
	return float64(grid) * lingjingVideoFPS * durationSec
}

// selectVideoTier 在分档里按「在线推理 + 有声/无声」选 ¥/Mtoken；退化到任一在线档；再退化首档。0 表示无可用档。
func selectVideoTier(tiers []VideoPriceTier, generateAudio bool) float64 {
	online := func(spec string) bool { return !strings.Contains(spec, "离线") }
	wantAudio := func(spec string) bool {
		if generateAudio {
			return strings.Contains(spec, "有声")
		}
		return strings.Contains(spec, "无声")
	}
	for _, t := range tiers {
		if online(t.Spec) && wantAudio(t.Spec) {
			return t.CNYPerMToken
		}
	}
	for _, t := range tiers {
		if online(t.Spec) {
			return t.CNYPerMToken
		}
	}
	if len(tiers) > 0 {
		return tiers[0].CNYPerMToken
	}
	return 0
}

// CalculateVideoTokenCostUSD 按官方 token 公式 + 给定分档单价计算单条视频 USD 成本。
// tiers 来自 model_pricings.tier_pricing；mode 为分辨率档；generateAudio 选有声/无声；cnyRate 把 ¥ 折 USD。
// 返回 0 表示无可用定价（计费链路据此 fail-closed）。
func CalculateVideoTokenCostUSD(tiers []VideoPriceTier, mode string, durationSec float64, generateAudio bool, cnyRate float64) float64 {
	if durationSec <= 0 || len(tiers) == 0 {
		return 0
	}
	if cnyRate <= 0 {
		cnyRate = DefaultWanjieCNYRate
	}
	cnyPerMToken := selectVideoTier(tiers, generateAudio)
	if cnyPerMToken <= 0 {
		return 0
	}
	return lingjingVideoTokens(mode, durationSec) / 1_000_000 * cnyPerMToken / cnyRate
}

// videoTierDisplayPerSecondUSD 返回「展示用」per-second 价（720p 无声基准），供导入器写 output_cost_per_image。
func videoTierDisplayPerSecondUSD(tiers []VideoPriceTier, cnyRate float64) float64 {
	return CalculateVideoTokenCostUSD(tiers, "720p", 1, false, cnyRate)
}
