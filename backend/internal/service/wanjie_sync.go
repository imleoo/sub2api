package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ModelPricingSourceWanjie 标识来源为万界 MaaS 平台（保留为已知来源标签）。
const ModelPricingSourceWanjie = "wanjie"

// wanjieResponse 是 MaaS 定价 JSON 的顶层结构（万界 API 响应格式，doubao/lingjing 同构）。
type wanjieResponse struct {
	Result []wanjieModel `json:"result"`
}

type wanjieModel struct {
	ModelName           string                `json:"modelName"`
	OfficialProvider    string                `json:"officialProvider"`
	ModelType           int                   `json:"modelType"`
	DiscountRate        float64               `json:"discountRate"`
	ModelModalRelations []wanjieModalRelation `json:"modelModalRelations"`
}

type wanjieModalRelation struct {
	ModalClass int               `json:"modalClass"`
	TypeGroups []wanjieTypeGroup `json:"typeGroups"`
}

type wanjieTypeGroup struct {
	ModalType    int                 `json:"modalType"`
	DetailGroups []wanjieDetailGroup `json:"detailGroups"`
}

type wanjieDetailGroup struct {
	Relations []wanjieRelation `json:"relations"`
}

type wanjieRelation struct {
	ModalClass        int    `json:"modalClass"`
	ModalType         int    `json:"modalType"`
	ChargeValue       string `json:"chargeValue"`       // 折后价（万界实际收费）
	OfficeChargeValue string `json:"officeChargeValue"` // 原价（官方定价）
	ModalName         string `json:"modalName"`
	ModalKey          string `json:"modalKey"`
	Unit              string `json:"unit"`
}

// DefaultWanjieCNYRate 是 cnyRate<=0 时的兜底汇率，与 config.Pricing.CNYRate 默认值一致。
const DefaultWanjieCNYRate = 6.8

// ParseMaasFromBytes 解析 MaaS 定价 JSON（万界 API 响应格式，doubao/lingjing 等同构），
// 按 source 标记来源（任意字符串）。cnyRate<=0 回退 DefaultWanjieCNYRate，字段语义统一 USD（视频分档存 ¥ 原值）。
func ParseMaasFromBytes(data []byte, cnyRate float64, source string) ([]*DBModelPricing, error) {
	var resp wanjieResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	rate := cnyRate
	if rate <= 0 {
		rate = DefaultWanjieCNYRate
	}
	out := make([]*DBModelPricing, 0, len(resp.Result))
	for _, m := range resp.Result {
		p := convertMaasModel(&m, rate, source)
		if p != nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// FetchAndParseWanjieModels 向万界 MaaS API 发起 GET 请求并解析定价数据。
// client 可为 nil（使用默认 http.Client），url 和 token 不能为空。cnyRate 同 ParseWanjieModels。
func FetchAndParseMaasModels(ctx context.Context, client *http.Client, url, token string, cnyRate float64, source string) ([]*DBModelPricing, error) {
	body, err := fetchMaasJSON(ctx, client, url, token)
	if err != nil {
		return nil, err
	}
	return ParseMaasFromBytes(body, cnyRate, source)
}

// fetchMaasJSON 向万界/豆包 MaaS API 发起 GET 请求并返回原始响应体（两平台同协议）。
func fetchMaasJSON(ctx context.Context, client *http.Client, url, token string) ([]byte, error) {
	if client == nil {
		client = &http.Client{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-access-token", token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch maas api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("maas api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return body, nil
}

// convertMaasModel 将单个万界/豆包模型转换为 DBModelPricing（两者 JSON 同构）。
// cnyRate 用于把 ¥ 单价换算为 USD（字段语义为 USD）；source 标记来源（wanjie/doubao）。
func convertMaasModel(m *wanjieModel, cnyRate float64, source string) *DBModelPricing {
	if strings.TrimSpace(m.ModelName) == "" {
		return nil
	}

	var (
		inputCost     *float64
		outputCost    *float64
		cacheRead     *float64
		cache5m       *float64
		cache1h       *float64
		imgPerImage   *float64
		supportsCache bool
		hasTextIn     bool
		hasTextOut    bool
		hasImgOut     bool
		hasVideoOut   bool
		videoTiers    []VideoPriceTier // 按 token 计费的视频分档（¥/百万 token 原值）
		lastVideoSpec string
	)

	for _, rel := range m.ModelModalRelations {
		for _, tg := range rel.TypeGroups {
			for _, dg := range tg.DetailGroups {
				for _, r := range dg.Relations {
					// 优先使用官方原价；原价缺失时回退到折后价
					rawVal := strings.TrimSpace(r.OfficeChargeValue)
					if rawVal == "" {
						rawVal = strings.TrimSpace(r.ChargeValue)
					}
					cnyVal, _ := strconv.ParseFloat(rawVal, 64)
					// 万界单价为人民币，统一换算为 USD（schema 字段语义全部是 USD）。
					v := cnyVal / cnyRate
					switch r.ModalKey {
					case "1-1-1", "1-1-6": // 输入文本单价 ¥/M token
						if inputCost == nil && v > 0 {
							perToken := v / 1_000_000
							inputCost = &perToken
							hasTextIn = true
						}
					case "1-1-2": // 缓存命中单价（输入侧）¥/M token
						if cacheRead == nil && v > 0 {
							perToken := v / 1_000_000
							cacheRead = &perToken
							supportsCache = true
						}
					case "1-1-8": // 5 分钟缓存创建 ¥/M token
						if cache5m == nil && v > 0 {
							perToken := v / 1_000_000
							cache5m = &perToken
							supportsCache = true
						}
					case "1-1-9": // 1 小时缓存创建 ¥/M token
						if cache1h == nil && v > 0 {
							perToken := v / 1_000_000
							cache1h = &perToken
							supportsCache = true
						}
					case "2-1-1", "2-1-6": // 输出文本单价 ¥/M token
						if outputCost == nil && v > 0 {
							perToken := v / 1_000_000
							outputCost = &perToken
							hasTextOut = true
						}
					case "2-1-7", "2-1-2": // 输出侧缓存命中单价（Claude/Qwen 使用）
						if cacheRead == nil && v > 0 {
							perToken := v / 1_000_000
							cacheRead = &perToken
							supportsCache = true
						}
					case "2-2-2": // 图片输出单价 ¥/张
						if imgPerImage == nil && v > 0 {
							imgPerImage = &v
							hasImgOut = true
						}
					case "2-4-spec": // 视频档位规格描述（在线/离线、有声/无声、分辨率）
						lastVideoSpec = strings.TrimSpace(firstNonEmpty(r.ChargeValue, r.OfficeChargeValue, r.ModalName))
					case "2-4-1", "2-4-7": // 视频/音频
						hasVideoOut = true
						if strings.Contains(r.Unit, "token") {
							// 按 token 计费（¥/百万 token）：收集为分档，存 ¥ 原值（计费时按汇率折 USD）。
							if cnyVal > 0 {
								videoTiers = append(videoTiers, VideoPriceTier{Spec: lastVideoSpec, CNYPerMToken: cnyVal})
							}
						} else if imgPerImage == nil && v > 0 {
							// 按秒计费（¥/秒）：取首档存 output_cost_per_image（USD/秒）。
							imgPerImage = &v
						}
					}
				}
			}
		}
	}

	mode := wanjieModelTypeToMode(m.ModelType, hasTextIn, hasTextOut, hasImgOut, hasVideoOut)
	provider := normalizeWanjieProvider(m.OfficialProvider)

	// 按 token 计费的视频：分档存 TierPricing；output_cost_per_image 写「展示用」per-second 价（720p 无声基准）。
	if len(videoTiers) > 0 && imgPerImage == nil {
		if perSec := videoTierDisplayPerSecondUSD(videoTiers, cnyRate); perSec > 0 {
			imgPerImage = &perSec
		}
	}

	// 价格字段已直接从 officeChargeValue（官方原价）读取，
	// 折扣率单独写入 DiscountRate，系统展示实际收费时再乘以折扣率。
	var discountRate *float64
	if m.DiscountRate > 0 && m.DiscountRate < 1 {
		dr := m.DiscountRate
		discountRate = &dr
	}

	pricingStatus := ModelPricingStatusUnpriced
	if inputCost != nil || outputCost != nil || imgPerImage != nil || len(videoTiers) > 0 {
		pricingStatus = ModelPricingStatusPriced
	}

	// 分列约定的 pricing_unit：图片（按次价存 output_cost_per_image）标 image_generation；
	// 视频（按秒价复用 output_cost_per_image）标 video_generation；其余按 token。
	pricingUnit := ModelPricingUnitToken
	switch {
	case mode == "image_generation" && imgPerImage != nil:
		pricingUnit = ModelPricingUnitImage
	case mode == "video_generation":
		pricingUnit = ModelPricingUnitVideo
	}

	return &DBModelPricing{
		ModelID:                  m.ModelName,
		Provider:                 provider,
		Mode:                     mode,
		PricingUnit:              pricingUnit,
		InputCostPerToken:        inputCost,
		OutputCostPerToken:       outputCost,
		CacheReadInputTokenCost:  cacheRead,
		CacheCreation5mTokenCost: cache5m,
		CacheCreation1hTokenCost: cache1h,
		OutputCostPerImage:       imgPerImage,
		TierPricing:              videoTiers,
		SupportsPromptCaching:    supportsCache,
		DiscountRate:             discountRate,
		IsCustom:                 true,
		// MaaS 平台未给出定价的模型默认禁用，避免被计费链路按 0 元放行。
		IsEnabled:     pricingStatus == ModelPricingStatusPriced,
		Source:        source,
		PricingStatus: pricingStatus,
	}
}

// wanjieModelTypeToMode 将万界 modelType 转换为 mode 字符串。
// modelType:
//
//	0 = 通用（根据定价结构细分）
//	2 = 图片生成
//	3 = 文本（含缓存/推理）
//	4 = 语音/TTS
//	5 = 视频生成
func wanjieModelTypeToMode(modelType int, hasTextIn, hasTextOut, hasImgOut, hasVideoOut bool) string {
	switch modelType {
	case 2:
		return "image_generation"
	case 4:
		return "audio"
	case 5:
		return "video_generation"
	default:
		// modelType=0 或 3：按定价结构推断
		if hasTextIn || hasTextOut {
			return "chat"
		}
		if hasImgOut {
			return "image_generation"
		}
		if hasVideoOut {
			return "video_generation"
		}
		return "chat"
	}
}

// normalizeWanjieProvider 将万界平台的 officialProvider 转换为系统内部 provider 标识符。
func normalizeWanjieProvider(officialProvider string) string {
	switch strings.TrimSpace(officialProvider) {
	case "OpenAI":
		return "openai"
	case "Claude":
		return "anthropic"
	case "Gemini":
		return "google"
	case "DeepSeek":
		return "deepseek"
	case "Qwen":
		return "qwen"
	case "Flux":
		return "flux"
	case "即梦":
		return "jimeng"
	case "可灵":
		return "kling"
	case "Kimi":
		return "moonshot"
	case "MiMo":
		return "mimo"
	case "MiniMax":
		return "minimax"
	case "智谱":
		return "zhipu"
	case "腾讯混元":
		return "hunyuan"
	case "豆包":
		return "doubao"
	case "灵境":
		return "lingjing"
	default:
		s := strings.ToLower(strings.TrimSpace(officialProvider))
		s = strings.ReplaceAll(s, " ", "_")
		return s
	}
}
