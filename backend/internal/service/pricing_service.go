package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

// LiteLLMModelPricing LiteLLM价格数据结构
// 只保留我们需要的字段，使用指针来处理可能缺失的值
type LiteLLMModelPricing struct {
	InputCostPerToken                   float64 `json:"input_cost_per_token"`
	InputCostPerTokenPriority           float64 `json:"input_cost_per_token_priority"`
	OutputCostPerToken                  float64 `json:"output_cost_per_token"`
	OutputCostPerTokenPriority          float64 `json:"output_cost_per_token_priority"`
	CacheCreationInputTokenCost         float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostAbove1hr float64 `json:"cache_creation_input_token_cost_above_1hr"`
	CacheReadInputTokenCost             float64 `json:"cache_read_input_token_cost"`
	CacheReadInputTokenCostPriority     float64 `json:"cache_read_input_token_cost_priority"`
	LongContextInputTokenThreshold      int     `json:"long_context_input_token_threshold,omitempty"`
	LongContextInputCostMultiplier      float64 `json:"long_context_input_cost_multiplier,omitempty"`
	LongContextOutputCostMultiplier     float64 `json:"long_context_output_cost_multiplier,omitempty"`
	SupportsServiceTier                 bool    `json:"supports_service_tier"`
	LiteLLMProvider                     string  `json:"litellm_provider"`
	Mode                                string  `json:"mode"`
	SupportsPromptCaching               bool    `json:"supports_prompt_caching"`
	OutputCostPerImage                  float64 `json:"output_cost_per_image"`       // 图片生成模型每张图片价格
	OutputCostPerImageToken             float64 `json:"output_cost_per_image_token"` // 图片输出 token 价格
}

// PricingRemoteClient 远程价格数据获取接口
type PricingRemoteClient interface {
	FetchPricingJSON(ctx context.Context, url string) ([]byte, error)
	FetchHashText(ctx context.Context, url string) (string, error)
}

// LiteLLMRawEntry 用于解析原始JSON数据
type LiteLLMRawEntry struct {
	InputCostPerToken                   *float64 `json:"input_cost_per_token"`
	InputCostPerTokenPriority           *float64 `json:"input_cost_per_token_priority"`
	OutputCostPerToken                  *float64 `json:"output_cost_per_token"`
	OutputCostPerTokenPriority          *float64 `json:"output_cost_per_token_priority"`
	CacheCreationInputTokenCost         *float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostAbove1hr *float64 `json:"cache_creation_input_token_cost_above_1hr"`
	CacheReadInputTokenCost             *float64 `json:"cache_read_input_token_cost"`
	CacheReadInputTokenCostPriority     *float64 `json:"cache_read_input_token_cost_priority"`
	SupportsServiceTier                 bool     `json:"supports_service_tier"`
	LiteLLMProvider                     string   `json:"litellm_provider"`
	Mode                                string   `json:"mode"`
	SupportsPromptCaching               bool     `json:"supports_prompt_caching"`
	OutputCostPerImage                  *float64 `json:"output_cost_per_image"`
	OutputCostPerImageToken             *float64 `json:"output_cost_per_image_token"`
}

// PricingService 动态价格服务
type PricingService struct {
	cfg              *config.Config
	remoteClient     PricingRemoteClient
	settingRepo      SettingRepository
	modelPricingRepo ModelPricingRepository
	mu               sync.RWMutex
	lastUpdated      time.Time
	localHash        string

	catalog           map[string]*DBModelPricing
	aliasIdx          map[string]string // normalizedID → canonical model_id
	lastCatalogLoadAt time.Time
	lastRemoteCheckAt time.Time
	catalogLoadErrors int64

	// 停止信号
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewPricingService 创建价格服务
func NewPricingService(cfg *config.Config, remoteClient PricingRemoteClient, settingRepo SettingRepository) *PricingService {
	s := &PricingService{
		cfg:          cfg,
		remoteClient: remoteClient,
		settingRepo:  settingRepo,
		stopCh:       make(chan struct{}),
	}
	return s
}

// SetModelPricingRepo 注入 ModelPricingRepository（可选，Wire 不直接注入时使用）。
func (s *PricingService) SetModelPricingRepo(repo ModelPricingRepository) {
	s.modelPricingRepo = repo
}

// Initialize 初始化价格服务
func (s *PricingService) Initialize() error {
	if err := os.MkdirAll(s.cfg.Pricing.DataDir, 0755); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to create data directory: %v", err)
	}

	// 首次加载远端价格 JSON，解析后写入 DB
	if err := s.checkAndUpdatePricing(); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Initial load failed, using fallback: %v", err)
		if err := s.useFallbackPricing(); err != nil {
			return fmt.Errorf("failed to load pricing data: %w", err)
		}
	}

	// Bootstrap seed（20 条兜底 + 灵境），SeedIfNotExists 跳过已存在行
	if os.Getenv("PRICING_BOOTSTRAP_SEED") != "0" {
		RunBootstrapSeed(context.Background(), s.modelPricingRepo)
	}

	// 从 DB 构建 catalog + aliasIdx
	s.buildCatalogAndAliasIndex(context.Background())

	// 启动定时更新
	s.startUpdateScheduler()

	logger.LegacyPrintf("service.pricing", "[Pricing] Service initialized with %d catalog models", len(s.catalog))
	return nil
}

// Stop 停止价格服务
func (s *PricingService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	logger.LegacyPrintf("service.pricing", "%s", "[Pricing] Service stopped")
}

// TriggerDBSync 手动触发一次远端同步并写入 DB（供 /sync 端点调用）。
func (s *PricingService) TriggerDBSync(ctx context.Context) error {
	if err := s.checkAndUpdatePricing(); err != nil {
		return fmt.Errorf("trigger db sync: remote update: %w", err)
	}
	s.buildCatalogAndAliasIndex(ctx)
	return nil
}

// ReloadFromDB 触发一次 DB → catalog + aliasIdx 的同步刷新，供写路径调用后立即可见。
func (s *PricingService) ReloadFromDB(ctx context.Context) {
	s.buildCatalogAndAliasIndex(ctx)
}

// buildCatalogAndAliasIndex 从 DB 加载启用的全部模型，构建 catalog + aliasIdx 两个临时副本，
// 然后原子替换。任何步骤失败 → 保留旧 catalog，错误计数 +1（fail-open）。
//
// aliasIdx 来源（按优先级覆盖）：
//  1. CodexAliasPairs() 51 条 OpenAI 别名归一化
//  2. DB 每行 model_id 自身 + 小写 + normalizeModelNameForPricing 归一化
//     注：matchByModelFamily 家族 fuzzy 不预计算，在 lookupCatalog 中按需调用，与现有行为一致
//
// 计费路径在 PR-6 切流前仍走 pricingData，本结构仅供 GetModelPricingV2 / 测试用。
func (s *PricingService) buildCatalogAndAliasIndex(ctx context.Context) {
	if s.modelPricingRepo == nil {
		return
	}
	items, err := s.modelPricingRepo.LoadAllEnabled(ctx)
	if err != nil {
		s.mu.Lock()
		s.catalogLoadErrors++
		s.mu.Unlock()
		logger.LegacyPrintf("service.pricing", "[Pricing][Catalog] LoadAllEnabled failed: %v (keeping old catalog)", err)
		return
	}

	newCatalog := make(map[string]*DBModelPricing, len(items))
	for _, item := range items {
		if item == nil || item.ModelID == "" {
			continue
		}
		newCatalog[item.ModelID] = item
	}

	newAlias := buildAliasIndex(newCatalog)

	s.mu.Lock()
	s.catalog = newCatalog
	s.aliasIdx = newAlias
	s.lastCatalogLoadAt = time.Now()
	s.mu.Unlock()

	logger.LegacyPrintf("service.pricing", "[Pricing][Catalog] built catalog=%d alias=%d", len(newCatalog), len(newAlias))
}

// buildAliasIndex 构建 normalizedID → canonical model_id 索引。
// 三路来源（后写覆盖前写，但实际上没有冲突）：
//
//	A. catalog 自身：每行 model_id 的小写形态 / normalizeModelNameForPricing 归一化形态 → 自身
//	B. CodexAliasPairs：OpenAI 变体 → catalog 中的标准 model_id
func buildAliasIndex(catalog map[string]*DBModelPricing) map[string]string {
	if len(catalog) == 0 {
		return map[string]string{}
	}
	idx := make(map[string]string, len(catalog)*3)

	// A. catalog 自身
	for modelID := range catalog {
		lower := strings.ToLower(modelID)
		idx[lower] = modelID
		normalized := normalizeModelNameForPricing(lower)
		if normalized != "" && normalized != lower {
			idx[normalized] = modelID
		}
	}

	// A2. 点/横杠变体别名：让 doubao-seedance-1.5-pro ↔ doubao-seedance-1-5-pro 这类
	//     仅标点不同的拼写都命中同一行（导入器/账号 model_mapping 拼写不一致时仍能查到价）。
	//     只增不减：A 已注册全部真实模型的 lower 形态，下面的 exists 守卫保证变体绝不遮蔽真实模型或已有别名。
	for modelID := range catalog {
		lower := strings.ToLower(modelID)
		for _, variant := range []string{
			strings.ReplaceAll(lower, ".", "-"),
			strings.ReplaceAll(lower, "-", "."),
		} {
			if variant == "" || variant == lower {
				continue
			}
			if _, exists := idx[variant]; exists {
				continue
			}
			idx[variant] = modelID
		}
	}

	// B. CodexAliasPairs — 仅当 target model_id 在 catalog 里才注入，避免悬空别名
	for _, pair := range CodexAliasPairs() {
		variant := strings.ToLower(strings.TrimSpace(pair[0]))
		target := strings.TrimSpace(pair[1])
		if variant == "" || target == "" {
			continue
		}
		if _, ok := catalog[target]; !ok {
			continue
		}
		// 不覆盖已有的"模型自身别名"——variant 形态相同的就跳过
		if _, exists := idx[variant]; exists {
			continue
		}
		idx[variant] = target
	}

	return idx
}

// LookupCatalogWithFuzzy 先走精确 + alias，再走 Claude 家族 fuzzy。
// 这是 PR-6 切流的主入口：BillingService 用它替换老的 pricingData 路径。
// 同样不读 pricingData，纯依赖 catalog。
func (s *PricingService) LookupCatalogWithFuzzy(model string) *DBModelPricing {
	if entry := s.LookupCatalog(model); entry != nil {
		return entry
	}
	return s.matchFamilyInCatalog(strings.ToLower(strings.TrimSpace(model)))
}

// LookupCatalog 走 SSOT PR-4 影子路径：先精确查 catalog、再走 aliasIdx 归一化。
// 不做 matchByModelFamily fuzzy（PR-6 LookupCatalogWithFuzzy 才接入）。
// 返回 nil 表示 catalog miss — 调用方应保留兼容 fallback（unpriced 在 PR-6 视配置 block）。
func (s *PricingService) LookupCatalog(model string) *DBModelPricing {
	if model == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.catalog == nil {
		return nil
	}
	// 1. 精确命中
	if entry, ok := s.catalog[model]; ok {
		return entry
	}
	// 2. 小写 / 归一化
	lower := strings.ToLower(strings.TrimSpace(model))
	if canonical, ok := s.aliasIdx[lower]; ok {
		if entry, ok := s.catalog[canonical]; ok {
			return entry
		}
	}
	normalized := normalizeModelNameForPricing(lower)
	if normalized != "" && normalized != lower {
		if canonical, ok := s.aliasIdx[normalized]; ok {
			if entry, ok := s.catalog[canonical]; ok {
				return entry
			}
		}
	}
	// 3. normalizeKnownOpenAICodexModel — 处理 GPT 日期版本号等更宽松形式（如 gpt-5.4-2026-03-05）
	if codexNorm := normalizeKnownOpenAICodexModel(lower); codexNorm != "" && codexNorm != lower {
		if entry, ok := s.catalog[codexNorm]; ok {
			return entry
		}
		if canonical, ok := s.aliasIdx[codexNorm]; ok {
			if entry, ok := s.catalog[canonical]; ok {
				return entry
			}
		}
	}
	return nil
}

// CatalogSize 返回当前 catalog 中的模型数量（供监控 / 测试断言用）。
func (s *PricingService) CatalogSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.catalog)
}

// AliasIndexSize 返回当前 aliasIdx 中的 alias 数量。
func (s *PricingService) AliasIndexSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.aliasIdx)
}

// GetDBModelPricing 返回指定模型的 catalog 行（含自定义价格），供 BillingService 使用。
func (s *PricingService) GetDBModelPricing(modelID string) *DBModelPricing {
	return s.LookupCatalog(modelID)
}

// startUpdateScheduler 启动定时更新调度器
func (s *PricingService) startUpdateScheduler() {
	// 定期检查哈希更新
	hashInterval := time.Duration(s.cfg.Pricing.HashCheckIntervalMinutes) * time.Minute
	if hashInterval < time.Minute {
		hashInterval = 10 * time.Minute
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(hashInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx := context.Background()
				s.mu.Lock()
				s.lastRemoteCheckAt = time.Now()
				s.mu.Unlock()
				if err := s.syncWithRemote(); err != nil {
					logger.LegacyPrintf("service.pricing", "[Pricing] Sync failed: %v", err)
				}
				// 即使远端 hash 未变也强制刷新 catalog，保证多实例改价 ≤ 1 个 tick 对齐。
				s.buildCatalogAndAliasIndex(ctx)
			case <-s.stopCh:
				return
			}
		}
	}()

	logger.LegacyPrintf("service.pricing", "[Pricing] Update scheduler started (check every %v)", hashInterval)
}

// checkAndUpdatePricing 检查并更新价格数据
func (s *PricingService) checkAndUpdatePricing() error {
	pricingFile := s.getPricingFilePath()

	// 检查本地文件是否存在
	if _, err := os.Stat(pricingFile); os.IsNotExist(err) {
		logger.LegacyPrintf("service.pricing", "%s", "[Pricing] Local pricing file not found, downloading...")
		return s.downloadPricingData()
	}

	// 先加载本地文件（确保服务可用），再检查是否需要更新
	if err := s.loadPricingData(pricingFile); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to load local file, downloading: %v", err)
		return s.downloadPricingData()
	}

	// 如果配置了哈希URL，通过远程哈希检查是否有更新
	if s.cfg.Pricing.HashURL != "" {
		remoteHash, err := s.fetchRemoteHash()
		if err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Failed to fetch remote hash on startup: %v", err)
			return nil // 已加载本地文件，哈希获取失败不影响启动
		}

		s.mu.RLock()
		localHash := s.localHash
		s.mu.RUnlock()

		if localHash == "" || remoteHash != localHash {
			logger.LegacyPrintf("service.pricing", "[Pricing] Remote hash differs on startup (local=%s remote=%s), downloading...",
				localHash[:min(8, len(localHash))], remoteHash[:min(8, len(remoteHash))])
			if err := s.downloadPricingData(); err != nil {
				logger.LegacyPrintf("service.pricing", "[Pricing] Download failed, using existing file: %v", err)
			}
		}
		return nil
	}

	// 没有哈希URL时，基于文件年龄检查
	info, err := os.Stat(pricingFile)
	if err != nil {
		return nil // 已加载本地文件
	}

	fileAge := time.Since(info.ModTime())
	maxAge := time.Duration(s.cfg.Pricing.UpdateIntervalHours) * time.Hour

	if fileAge > maxAge {
		logger.LegacyPrintf("service.pricing", "[Pricing] Local file is %v old, updating...", fileAge.Round(time.Hour))
		if err := s.downloadPricingData(); err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Download failed, using existing file: %v", err)
		}
	}

	return nil
}

// syncWithRemote 与远程同步（基于哈希校验）
func (s *PricingService) syncWithRemote() error {
	// 如果配置了哈希URL，从远程获取哈希进行比对
	if s.cfg.Pricing.HashURL != "" {
		remoteHash, err := s.fetchRemoteHash()
		if err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Failed to fetch remote hash: %v", err)
			return nil // 哈希获取失败不影响正常使用
		}

		s.mu.RLock()
		localHash := s.localHash
		s.mu.RUnlock()

		if localHash == "" || remoteHash != localHash {
			logger.LegacyPrintf("service.pricing", "[Pricing] Remote hash differs (local=%s remote=%s), downloading new version...",
				localHash[:min(8, len(localHash))], remoteHash[:min(8, len(remoteHash))])
			return s.downloadPricingData()
		}
		logger.LegacyPrintf("service.pricing", "%s", "[Pricing] Hash check passed, no update needed")
		return nil
	}

	// 没有哈希URL时，基于时间检查
	pricingFile := s.getPricingFilePath()
	info, err := os.Stat(pricingFile)
	if err != nil {
		return s.downloadPricingData()
	}

	fileAge := time.Since(info.ModTime())
	maxAge := time.Duration(s.cfg.Pricing.UpdateIntervalHours) * time.Hour

	if fileAge > maxAge {
		logger.LegacyPrintf("service.pricing", "[Pricing] File is %v old, downloading...", fileAge.Round(time.Hour))
		return s.downloadPricingData()
	}

	return nil
}

// downloadPricingData 从远程下载价格数据
func (s *PricingService) downloadPricingData() error {
	remoteURL, err := s.validatePricingURL(s.cfg.Pricing.RemoteURL)
	if err != nil {
		return err
	}
	logger.LegacyPrintf("service.pricing", "[Pricing] Downloading from %s", remoteURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取远程哈希（用于同步锚点，不作为完整性校验）
	var remoteHash string
	if strings.TrimSpace(s.cfg.Pricing.HashURL) != "" {
		remoteHash, err = s.fetchRemoteHash()
		if err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Failed to fetch remote hash (continuing): %v", err)
		}
	}

	body, err := s.remoteClient.FetchPricingJSON(ctx, remoteURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// 哈希校验：不匹配时仅告警，不阻止更新
	// 远程哈希文件可能与数据文件不同步（如维护者更新了数据但未更新哈希文件）
	dataHash := sha256.Sum256(body)
	dataHashStr := hex.EncodeToString(dataHash[:])
	if remoteHash != "" && !strings.EqualFold(remoteHash, dataHashStr) {
		logger.LegacyPrintf("service.pricing", "[Pricing] Hash mismatch warning: remote=%s data=%s (hash file may be out of sync)",
			remoteHash[:min(8, len(remoteHash))], dataHashStr[:8])
	}

	// 解析JSON数据（使用灵活的解析方式）
	data, err := s.parsePricingData(body)
	if err != nil {
		return fmt.Errorf("parse pricing data: %w", err)
	}

	// 保存到本地文件
	pricingFile := s.getPricingFilePath()
	if err := os.WriteFile(pricingFile, body, 0644); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to save file: %v", err)
	}

	// 使用远程哈希作为同步锚点，防止重复下载
	// 当远程哈希不可用时，回退到数据本身的哈希
	syncHash := dataHashStr
	if remoteHash != "" {
		syncHash = remoteHash
	}
	hashFile := s.getHashFilePath()
	if err := os.WriteFile(hashFile, []byte(syncHash+"\n"), 0644); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to save hash: %v", err)
	}

	// 写入 DB，然后重建 catalog
	if s.modelPricingRepo != nil {
		models := liteLLMMapToDBModels(data)
		if err := s.modelPricingRepo.UpsertBatch(context.Background(), models); err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] UpsertBatch failed: %v", err)
		} else {
			logger.LegacyPrintf("service.pricing", "[Pricing] Upserted %d models to DB", len(models))
		}
	}

	s.mu.Lock()
	s.lastUpdated = time.Now()
	s.localHash = syncHash
	s.mu.Unlock()

	s.buildCatalogAndAliasIndex(context.Background())
	logger.LegacyPrintf("service.pricing", "[Pricing] Downloaded %d models successfully", len(data))
	return nil
}

// parsePricingData 解析价格数据（处理各种格式）
func (s *PricingService) parsePricingData(body []byte) (map[string]*LiteLLMModelPricing, error) {
	// 首先解析为 map[string]json.RawMessage
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("parse raw JSON: %w", err)
	}

	result := make(map[string]*LiteLLMModelPricing)
	skipped := 0

	for modelName, rawEntry := range rawData {
		// 跳过 sample_spec 等文档条目
		if modelName == "sample_spec" {
			continue
		}

		// 尝试解析每个条目
		var entry LiteLLMRawEntry
		if err := json.Unmarshal(rawEntry, &entry); err != nil {
			skipped++
			continue
		}

		// 只保留有有效价格的条目
		if entry.InputCostPerToken == nil && entry.OutputCostPerToken == nil {
			continue
		}

		pricing := &LiteLLMModelPricing{
			LiteLLMProvider:       entry.LiteLLMProvider,
			Mode:                  entry.Mode,
			SupportsPromptCaching: entry.SupportsPromptCaching,
			SupportsServiceTier:   entry.SupportsServiceTier,
		}

		if entry.InputCostPerToken != nil {
			pricing.InputCostPerToken = *entry.InputCostPerToken
		}
		if entry.InputCostPerTokenPriority != nil {
			pricing.InputCostPerTokenPriority = *entry.InputCostPerTokenPriority
		}
		if entry.OutputCostPerToken != nil {
			pricing.OutputCostPerToken = *entry.OutputCostPerToken
		}
		if entry.OutputCostPerTokenPriority != nil {
			pricing.OutputCostPerTokenPriority = *entry.OutputCostPerTokenPriority
		}
		if entry.CacheCreationInputTokenCost != nil {
			pricing.CacheCreationInputTokenCost = *entry.CacheCreationInputTokenCost
		}
		if entry.CacheCreationInputTokenCostAbove1hr != nil {
			pricing.CacheCreationInputTokenCostAbove1hr = *entry.CacheCreationInputTokenCostAbove1hr
		}
		if entry.CacheReadInputTokenCost != nil {
			pricing.CacheReadInputTokenCost = *entry.CacheReadInputTokenCost
		}
		if entry.CacheReadInputTokenCostPriority != nil {
			pricing.CacheReadInputTokenCostPriority = *entry.CacheReadInputTokenCostPriority
		}
		// output_cost_per_image（按张/次的扁平图片价）只对图片/视频模型有意义。
		// LiteLLM 部分 chat 多模态模型会携带它（实为输入侧多模态计价），按本系统 mode 计价口径属污染，过滤掉，
		// 与迁移 155 的存量清洗口径保持一致，防止下次 sync 重新写脏。
		if entry.OutputCostPerImage != nil &&
			(entry.Mode == "image_generation" || entry.Mode == "video_generation") {
			pricing.OutputCostPerImage = *entry.OutputCostPerImage
		}
		// output_cost_per_image_token（图片输出 token 价）对多模态 chat 也合法，保留。
		if entry.OutputCostPerImageToken != nil {
			pricing.OutputCostPerImageToken = *entry.OutputCostPerImageToken
		}

		result[modelName] = pricing
	}

	if skipped > 0 {
		logger.LegacyPrintf("service.pricing", "[Pricing] Skipped %d invalid entries", skipped)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid pricing entries found")
	}

	return result, nil
}

// loadPricingData 从本地文件加载价格数据，写入 DB 后重建 catalog。
func (s *PricingService) loadPricingData(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file failed: %w", err)
	}

	pricingData, err := s.parsePricingData(data)
	if err != nil {
		return fmt.Errorf("parse pricing data: %w", err)
	}

	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	if s.modelPricingRepo != nil {
		models := liteLLMMapToDBModels(pricingData)
		if err := s.modelPricingRepo.UpsertBatch(context.Background(), models); err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] UpsertBatch failed: %v", err)
		}
	}

	s.mu.Lock()
	s.localHash = hashStr
	info, _ := os.Stat(filePath)
	if info != nil {
		s.lastUpdated = info.ModTime()
	} else {
		s.lastUpdated = time.Now()
	}
	s.mu.Unlock()

	s.buildCatalogAndAliasIndex(context.Background())
	logger.LegacyPrintf("service.pricing", "[Pricing] Loaded %d models from %s", len(pricingData), filePath)
	return nil
}

// liteLLMMapToDBModels 将 LiteLLM 解析结果转为 DB 写入格式（source=litellm）。
func liteLLMMapToDBModels(data map[string]*LiteLLMModelPricing) []*DBModelPricing {
	now := time.Now()
	models := make([]*DBModelPricing, 0, len(data))
	for modelID, p := range data {
		m := &DBModelPricing{
			ModelID:               modelID,
			Provider:              p.LiteLLMProvider,
			Mode:                  p.Mode,
			SupportsPromptCaching: p.SupportsPromptCaching,
			IsCustom:              false,
			IsEnabled:             true,
			Source:                ModelPricingSourceLiteLLM,
			LastSyncedAt:          &now,
		}
		if p.InputCostPerToken != 0 {
			v := p.InputCostPerToken
			m.InputCostPerToken = &v
		}
		if p.OutputCostPerToken != 0 {
			v := p.OutputCostPerToken
			m.OutputCostPerToken = &v
		}
		if p.InputCostPerTokenPriority != 0 {
			v := p.InputCostPerTokenPriority
			m.InputCostPerTokenPriority = &v
		}
		if p.OutputCostPerTokenPriority != 0 {
			v := p.OutputCostPerTokenPriority
			m.OutputCostPerTokenPriority = &v
		}
		if p.CacheCreationInputTokenCost != 0 {
			v := p.CacheCreationInputTokenCost
			m.CacheCreationInputTokenCost = &v
		}
		if p.CacheCreationInputTokenCostAbove1hr != 0 {
			v := p.CacheCreationInputTokenCostAbove1hr
			m.CacheCreation1hTokenCost = &v
		}
		if p.CacheReadInputTokenCost != 0 {
			v := p.CacheReadInputTokenCost
			m.CacheReadInputTokenCost = &v
		}
		if p.CacheReadInputTokenCostPriority != 0 {
			v := p.CacheReadInputTokenCostPriority
			m.CacheReadInputTokenCostPriority = &v
		}
		if p.OutputCostPerImage != 0 {
			v := p.OutputCostPerImage
			m.OutputCostPerImage = &v
		}
		if p.OutputCostPerImageToken != 0 {
			v := p.OutputCostPerImageToken
			m.OutputCostPerImageToken = &v
		}
		models = append(models, m)
	}
	return models
}

// useFallbackPricing 使用回退价格文件
func (s *PricingService) useFallbackPricing() error {
	fallbackFile := s.cfg.Pricing.FallbackFile

	if _, err := os.Stat(fallbackFile); os.IsNotExist(err) {
		return fmt.Errorf("fallback file not found: %s", fallbackFile)
	}

	logger.LegacyPrintf("service.pricing", "[Pricing] Using fallback file: %s", fallbackFile)

	// 复制到数据目录
	data, err := os.ReadFile(fallbackFile)
	if err != nil {
		return fmt.Errorf("read fallback failed: %w", err)
	}

	pricingFile := s.getPricingFilePath()
	if err := os.WriteFile(pricingFile, data, 0644); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to copy fallback: %v", err)
	}

	return s.loadPricingData(fallbackFile)
}

// fetchRemoteHash 从远程获取哈希值
func (s *PricingService) fetchRemoteHash() (string, error) {
	hashURL, err := s.validatePricingURL(s.cfg.Pricing.HashURL)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hash, err := s.remoteClient.FetchHashText(ctx, hashURL)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(hash), nil
}

func (s *PricingService) validatePricingURL(raw string) (string, error) {
	if s.cfg != nil && !s.cfg.Security.URLAllowlist.Enabled {
		normalized, err := urlvalidator.ValidateURLFormat(raw, s.cfg.Security.URLAllowlist.AllowInsecureHTTP)
		if err != nil {
			return "", fmt.Errorf("invalid pricing url: %w", err)
		}
		return normalized, nil
	}
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		AllowedHosts:     s.cfg.Security.URLAllowlist.PricingHosts,
		RequireAllowlist: true,
		AllowPrivate:     s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
	if err != nil {
		return "", fmt.Errorf("invalid pricing url: %w", err)
	}
	return normalized, nil
}

func normalizeModelNameForPricing(model string) string {
	// Common Gemini/VertexAI forms:
	// - models/gemini-2.0-flash-exp
	// - publishers/google/models/gemini-2.5-pro
	// - projects/.../locations/.../publishers/google/models/gemini-2.5-pro
	model = strings.TrimSpace(model)
	model = strings.TrimLeft(model, "/")
	model = strings.TrimPrefix(model, "models/")
	model = strings.TrimPrefix(model, "publishers/google/models/")

	if idx := strings.LastIndex(model, "/publishers/google/models/"); idx != -1 {
		model = model[idx+len("/publishers/google/models/"):]
	}
	if idx := strings.LastIndex(model, "/models/"); idx != -1 {
		model = model[idx+len("/models/"):]
	}

	model = strings.TrimLeft(model, "/")
	if canonical := canonicalizeOpenAIModelAliasSpelling(model); canonical != "" {
		return canonical
	}
	return model
}

// matchFamilyInCatalog 在 catalog 上执行 Claude 家族 fuzzy 匹配。
func (s *PricingService) matchFamilyInCatalog(model string) *DBModelPricing {
	if model == "" {
		return nil
	}

	type claudeFamily struct {
		name    string
		match   []string
		pricing []string
	}
	// 与 matchByModelFamily 保持一致的家族切片顺序（高版本优先）。
	// opus-3 须单独列出，避免 Phase-3 用 "claude-opus-4" 子串误命中 claude-opus-4.x 条目。
	families := []claudeFamily{
		{name: "opus-4.7", match: []string{"claude-opus-4-7", "claude-opus-4.7"}, pricing: []string{"claude-opus-4-7", "claude-opus-4.7", "claude-opus-4-6"}},
		{name: "opus-4.6", match: []string{"claude-opus-4-6", "claude-opus-4.6"}},
		{name: "opus-4.5", match: []string{"claude-opus-4-5", "claude-opus-4.5"}},
		{name: "opus-4", match: []string{"claude-opus-4"}},
		{name: "opus-3", match: []string{"claude-3-opus"}, pricing: []string{"claude-3-opus"}},
		{name: "sonnet-4.5", match: []string{"claude-sonnet-4-5", "claude-sonnet-4.5"}},
		{name: "sonnet-4", match: []string{"claude-sonnet-4", "claude-3-5-sonnet"}},
		{name: "sonnet-3.5", match: []string{"claude-3-5-sonnet", "claude-3.5-sonnet"}},
		{name: "sonnet-3", match: []string{"claude-3-sonnet"}},
		{name: "haiku-3.5", match: []string{"claude-3-5-haiku", "claude-3.5-haiku"}},
		{name: "haiku-3", match: []string{"claude-3-haiku"}},
	}

	// Phase 1: 按有序切片归类
	var matched *claudeFamily
	for i := range families {
		for _, pattern := range families[i].match {
			if strings.Contains(model, pattern) || strings.Contains(model, strings.ReplaceAll(pattern, "-", "")) {
				matched = &families[i]
				break
			}
		}
		if matched != nil {
			break
		}
	}

	// Phase 2: 关键字粗分兜底
	if matched == nil {
		var fallbackName string
		switch {
		case strings.Contains(model, "opus"):
			switch {
			case strings.Contains(model, "4.7") || strings.Contains(model, "4-7"):
				fallbackName = "opus-4.7"
			case strings.Contains(model, "4.6") || strings.Contains(model, "4-6"):
				fallbackName = "opus-4.6"
			case strings.Contains(model, "4.5") || strings.Contains(model, "4-5"):
				fallbackName = "opus-4.5"
			default:
				fallbackName = "opus-4"
			}
		case strings.Contains(model, "sonnet"):
			switch {
			case strings.Contains(model, "4.5") || strings.Contains(model, "4-5"):
				fallbackName = "sonnet-4.5"
			case strings.Contains(model, "3-5") || strings.Contains(model, "3.5"):
				fallbackName = "sonnet-3.5"
			default:
				fallbackName = "sonnet-4"
			}
		case strings.Contains(model, "haiku"):
			switch {
			case strings.Contains(model, "3-5") || strings.Contains(model, "3.5"):
				fallbackName = "haiku-3.5"
			default:
				fallbackName = "haiku-3"
			}
		case strings.HasPrefix(model, "claude"):
			// 未知 claude-* 模型（无 opus/sonnet/haiku 关键词）兜底到 sonnet-4 价格。
			fallbackName = "sonnet-4"
		}
		if fallbackName != "" {
			for i := range families {
				if families[i].name == fallbackName {
					matched = &families[i]
					break
				}
			}
		}
	}

	if matched == nil {
		return nil
	}

	// Phase 3: 在 catalog 上子串查找
	s.mu.RLock()
	defer s.mu.RUnlock()

	lookups := matched.pricing
	if lookups == nil {
		lookups = matched.match
	}
	for _, pattern := range lookups {
		for key, entry := range s.catalog {
			keyLower := strings.ToLower(key)
			if strings.Contains(keyLower, pattern) {
				logger.LegacyPrintf("service.pricing", "[Pricing][Catalog] Fuzzy matched %s -> %s", model, key)
				return entry
			}
		}
	}

	return nil
}

// GetStatus 获取服务状态
func (s *PricingService) GetStatus() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]any{
		"last_updated":             s.lastUpdated,
		"local_hash":               s.localHash[:min(8, len(s.localHash))],
		"catalog_size":             len(s.catalog),
		"alias_index_size":         len(s.aliasIdx),
		"catalog_last_loaded_at":   s.lastCatalogLoadAt,
		"remote_last_check_at":     s.lastRemoteCheckAt,
		"catalog_load_error_count": s.catalogLoadErrors,
	}
}

// ForceUpdate 强制更新
func (s *PricingService) ForceUpdate() error {
	return s.downloadPricingData()
}

// getPricingFilePath 获取价格文件路径
func (s *PricingService) getPricingFilePath() string {
	return filepath.Join(s.cfg.Pricing.DataDir, "model_pricing.json")
}

// getHashFilePath 获取哈希文件路径
func (s *PricingService) getHashFilePath() string {
	return filepath.Join(s.cfg.Pricing.DataDir, "model_pricing.sha256")
}

// ListModelNamesByProvider returns all model names in the catalog whose
// Provider matches the given provider string (case-insensitive).
// The returned slice is sorted alphabetically.
func (s *PricingService) ListModelNamesByProvider(provider string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider = strings.ToLower(strings.TrimSpace(provider))
	names := make([]string, 0)
	for name, entry := range s.catalog {
		if strings.ToLower(entry.Provider) == provider {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ModelInfo 公开的模型信息（用于用户端展示）
type ModelInfo struct {
	ID                             string   `json:"id"`
	LiteLLMProvider                string   `json:"provider"`
	Mode                           string   `json:"mode"`
	PricingUnit                    string   `json:"pricing_unit"`
	InputCostPerToken              float64  `json:"input_cost_per_token"`
	OutputCostPerToken             float64  `json:"output_cost_per_token"`
	OutputCostPerImage             *float64 `json:"output_cost_per_image,omitempty"`       // 图片/视频按次价（USD/张 或 USD/秒）
	OutputCostPerImageToken        *float64 `json:"output_cost_per_image_token,omitempty"` // 图片/视频按 token 价（USD/token）
	SupportsPromptCaching          bool     `json:"supports_prompt_caching"`
	LongContextInputTokenThreshold int      `json:"long_context_input_token_threshold,omitempty"`
	DiscountRate                   float64  `json:"discount_rate,omitempty"`
}

// GetDiscount 返回模型折扣率。优先读 catalog 中的 DiscountRate，无则返回 1.0。
func (s *PricingService) GetDiscount(model string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if entry, ok := s.catalog[strings.ToLower(model)]; ok && entry.DiscountRate != nil && *entry.DiscountRate > 0 {
		return *entry.DiscountRate
	}
	return 1.0
}

// GetCNYRate 返回人民币汇率（优先读 DB settingRepo，fallback 到 config）
func (s *PricingService) GetCNYRate() float64 {
	if s.settingRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if val, err := s.settingRepo.GetValue(ctx, SettingKeyCNYRate); err == nil {
			if rate, err := strconv.ParseFloat(val, 64); err == nil && rate > 0 {
				return rate
			}
		}
	}
	if s.cfg.Pricing.CNYRate > 0 {
		return s.cfg.Pricing.CNYRate
	}
	// 与 config.Pricing.CNYRate viper 默认值（6.8）以及 DefaultWanjieCNYRate 保持一致。
	return 6.8
}

// GetCurrencyMode 返回货币模式（优先读 DB settingRepo，fallback "usd"）
func (s *PricingService) GetCurrencyMode() string {
	if s.settingRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if val, err := s.settingRepo.GetValue(ctx, SettingKeyCurrencyMode); err == nil {
			if val == "usd" || val == "cny" {
				return val
			}
		}
	}
	return "usd"
}

// GetShowOverseasModels 返回是否在用户广场展示海外模型（优先读 DB settingRepo，默认 true）。
func (s *PricingService) GetShowOverseasModels() bool {
	if s.settingRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if val, err := s.settingRepo.GetValue(ctx, SettingKeyShowOverseasModels); err == nil {
			return strings.TrimSpace(val) != "false"
		}
	}
	return true
}

// ListEnabledCatalogModels 从 catalog（DB 驱动内存快照）返回所有 is_enabled=true 的模型信息。
// 用于"模型广场"在 generic 账号无 supported_models 白名单时的兜底展示。
func (s *PricingService) ListEnabledCatalogModels() []ModelInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ModelInfo, 0, len(s.catalog))
	for _, entry := range s.catalog {
		if !entry.IsEnabled {
			continue
		}
		info := ModelInfo{
			ID:                    entry.ModelID,
			LiteLLMProvider:       entry.Provider,
			Mode:                  entry.Mode,
			PricingUnit:           ModelPricingUnitToken,
			SupportsPromptCaching: entry.SupportsPromptCaching,
			DiscountRate:          1.0,
		}
		// pricing_unit 以 mode 为准（image/video），兼容 second，其余 token。
		switch {
		case entry.Mode == "image_generation":
			info.PricingUnit = ModelPricingUnitImage
		case entry.Mode == "video_generation":
			info.PricingUnit = ModelPricingUnitVideo
		case entry.PricingUnit == ModelPricingUnitSecond:
			info.PricingUnit = ModelPricingUnitSecond
		}
		if entry.DiscountRate != nil && *entry.DiscountRate > 0 {
			info.DiscountRate = *entry.DiscountRate
		}
		if entry.InputCostPerToken != nil {
			info.InputCostPerToken = *entry.InputCostPerToken
		}
		if entry.PricingUnit == ModelPricingUnitSecond && entry.CustomInputCost != nil {
			info.InputCostPerToken = *entry.CustomInputCost
		}
		if entry.OutputCostPerToken != nil {
			info.OutputCostPerToken = *entry.OutputCostPerToken
		}
		// 图片/视频价格透传（分列：按次价 output_cost_per_image / 按 token 价 output_cost_per_image_token），
		// 前端据此 + pricing_unit 渲染单张/单视频价，不再坍缩成 ¥0.00。
		if info.PricingUnit == ModelPricingUnitImage || info.PricingUnit == ModelPricingUnitVideo {
			info.OutputCostPerImage = entry.OutputCostPerImage
			info.OutputCostPerImageToken = entry.OutputCostPerImageToken
		}
		if entry.LongContextInputTokenThreshold != nil {
			info.LongContextInputTokenThreshold = int(*entry.LongContextInputTokenThreshold)
		}
		result = append(result, info)
	}
	return result
}

// ListAllModels 返回全部模型的基本信息和定价（供用户端模型列表页使用）。
// 委托给 ListEnabledCatalogModels（catalog 作为 SSOT）。
func (s *PricingService) ListAllModels() []ModelInfo {
	return s.ListEnabledCatalogModels()
}
