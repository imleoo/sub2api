package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ModelPricingSourceWanjie 标识来源为万界 MaaS 平台。
const ModelPricingSourceWanjie = "wanjie"

//go:embed wanjie.json
var wanjieRawJSON []byte

// wanjieResponse 是万界 API 响应的顶层结构。
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
	ModalType    int                  `json:"modalType"`
	DetailGroups []wanjieDetailGroup  `json:"detailGroups"`
}

type wanjieDetailGroup struct {
	Relations []wanjieRelation `json:"relations"`
}

type wanjieRelation struct {
	ModalClass        int    `json:"modalClass"`
	ModalType         int    `json:"modalType"`
	ChargeValue       string `json:"chargeValue"`        // 折后价（万界实际收费）
	OfficeChargeValue string `json:"officeChargeValue"` // 原价（官方定价）
	ModalName         string `json:"modalName"`
	ModalKey          string `json:"modalKey"`
	Unit              string `json:"unit"`
}

// DefaultWanjieCNYRate 是 cnyRate<=0 时的兜底汇率，与 config.Pricing.CNYRate 默认值一致。
const DefaultWanjieCNYRate = 7.0

// ParseWanjieModels 解析内嵌的 wanjie.json，返回可写入 model_pricings 表的记录列表。
// 每条记录对应万界平台一个模型，万界原始单价为 ¥/M token / ¥/张 / ¥/秒，
// 入库统一按 cnyRate 换算为 USD 单位（1 USD = cnyRate CNY）。
func ParseWanjieModels(cnyRate float64) ([]*DBModelPricing, error) {
	return ParseWanjieFromBytes(wanjieRawJSON, cnyRate)
}

// ParseWanjieFromBytes 解析任意来源的万界 API JSON 响应体。
// cnyRate<=0 时回退到 defaultWanjieCNYRate，确保字段语义始终是 USD。
func ParseWanjieFromBytes(data []byte, cnyRate float64) ([]*DBModelPricing, error) {
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
		p := convertWanjieModel(&m, rate)
		if p != nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// FetchAndParseWanjieModels 向万界 MaaS API 发起 GET 请求并解析定价数据。
// client 可为 nil（使用默认 http.Client），url 和 token 不能为空。cnyRate 同 ParseWanjieModels。
func FetchAndParseWanjieModels(ctx context.Context, client *http.Client, url, token string, cnyRate float64) ([]*DBModelPricing, error) {
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
		return nil, fmt.Errorf("fetch wanjie api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wanjie api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return ParseWanjieFromBytes(body, cnyRate)
}

// convertWanjieModel 将单个万界模型转换为 DBModelPricing。
// cnyRate 用于把万界 ¥ 单价换算为 USD（input/output_cost_per_token 等字段语义为 USD）。
func convertWanjieModel(m *wanjieModel, cnyRate float64) *DBModelPricing {
	if strings.TrimSpace(m.ModelName) == "" {
		return nil
	}

	var (
		inputCost    *float64
		outputCost   *float64
		cacheRead    *float64
		cache5m      *float64
		cache1h      *float64
		imgPerImage  *float64
		supportsCache bool
		hasTextIn     bool
		hasTextOut    bool
		hasImgOut     bool
		hasVideoOut   bool
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
					case "2-4-1", "2-4-7": // 视频/音频 ¥/秒
						// 多档（std/pro/master）按 JSON 顺序，取第一档（通常是最低档）作为基准价；
						// 复用 output_cost_per_image 字段存储（schema 注释允许 per image / per second 复用）。
						if imgPerImage == nil && v > 0 {
							imgPerImage = &v
						}
						hasVideoOut = true
					}
				}
			}
		}
	}

	mode := wanjieModelTypeToMode(m.ModelType, hasTextIn, hasTextOut, hasImgOut, hasVideoOut)
	provider := normalizeWanjieProvider(m.OfficialProvider)

	// 价格字段已直接从 officeChargeValue（官方原价）读取，
	// 折扣率单独写入 DiscountRate，系统展示实际收费时再乘以折扣率。
	var discountRate *float64
	if m.DiscountRate > 0 && m.DiscountRate < 1 {
		dr := m.DiscountRate
		discountRate = &dr
	}

	pricingStatus := ModelPricingStatusUnpriced
	if inputCost != nil || outputCost != nil || imgPerImage != nil {
		pricingStatus = ModelPricingStatusPriced
	}

	return &DBModelPricing{
		ModelID:                  m.ModelName,
		Provider:                 provider,
		Mode:                     mode,
		InputCostPerToken:        inputCost,
		OutputCostPerToken:       outputCost,
		CacheReadInputTokenCost:  cacheRead,
		CacheCreation5mTokenCost: cache5m,
		CacheCreation1hTokenCost: cache1h,
		OutputCostPerImage:       imgPerImage,
		SupportsPromptCaching:    supportsCache,
		DiscountRate:             discountRate,
		IsCustom:                 true,
		// 万界平台未给出定价的模型默认禁用，避免被计费链路按 0 元放行。
		IsEnabled:     pricingStatus == ModelPricingStatusPriced,
		Source:        ModelPricingSourceWanjie,
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
	default:
		s := strings.ToLower(strings.TrimSpace(officialProvider))
		s = strings.ReplaceAll(s, " ", "_")
		return s
	}
}
