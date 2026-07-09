package service

import (
	"context"
	"sort"
	"strings"
)

// ModelRoutingService 计算「账号可路由到哪些模型」——模型广场与后台「可见模型」的唯一口径来源。
// 之前广场在 handler 用 Go、后台在 repo 用 SQL 各实现一套，易漂移（已发生过 generic endpoint 漏算 bug）。
// 此 service 统一逻辑：广场传当前用户可访问账号，后台传全部 active 账号。
type ModelRoutingService struct {
	accountRepo  AccountRepository
	endpointRepo EndpointRepository
	pricing      *PricingService
}

func NewModelRoutingService(accountRepo AccountRepository, endpointRepo EndpointRepository, pricing *PricingService) *ModelRoutingService {
	return &ModelRoutingService{accountRepo: accountRepo, endpointRepo: endpointRepo, pricing: pricing}
}

// RoutableModelInfos 返回给定账号集合可路由到的 ModelInfo 列表（模型广场用）。
func (s *ModelRoutingService) RoutableModelInfos(ctx context.Context, accountIDs []int64) []ModelInfo {
	if s.accountRepo == nil || len(accountIDs) == 0 {
		return []ModelInfo{}
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil || len(accounts) == 0 {
		return []ModelInfo{}
	}
	return s.routableFromAccounts(ctx, accounts)
}

// OperatorRoutableModelInfos 返回全部 active 账号可路由到的 ModelInfo 列表
// （后台「可见模型」/ pricing_health 用）。调用方可据此派生原始 routable 集合，
// 或叠加 FilterVisibleModels（海外开关+版本下限）得到与模型广场一致的可见集。
func (s *ModelRoutingService) OperatorRoutableModelInfos(ctx context.Context) ([]ModelInfo, error) {
	if s.accountRepo == nil {
		return []ModelInfo{}, nil
	}
	active, err := s.accountRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	accounts := make([]*Account, 0, len(active))
	for i := range active {
		accounts = append(accounts, &active[i])
	}
	return s.routableFromAccounts(ctx, accounts), nil
}

// OperatorRoutableModelIDs 返回全部 active 账号可路由到的 model_id 集合（后台 pricing_health orphan 判定用）。
func (s *ModelRoutingService) OperatorRoutableModelIDs(ctx context.Context) (map[string]struct{}, error) {
	infos, err := s.OperatorRoutableModelInfos(ctx)
	if err != nil {
		return nil, err
	}
	return modelInfoIDSet(infos), nil
}

// modelInfoIDSet 把 ModelInfo 列表收敛成 model_id 集合。
func modelInfoIDSet(infos []ModelInfo) map[string]struct{} {
	set := make(map[string]struct{}, len(infos))
	for _, m := range infos {
		set[m.ID] = struct{}{}
	}
	return set
}

// routableFromAccounts 是统一的核心逻辑（原 usage_handler.collectWhitelistedModelsForAccounts）：
//   - generic 账号：取各 endpoint 的 supported_models 并集；某 endpoint 空白名单 → 全部启用模型兜底。
//   - 其它账号：取 credentials.model_mapping（键精确/通配，或映射值）命中 catalog。
func (s *ModelRoutingService) routableFromAccounts(ctx context.Context, accounts []*Account) []ModelInfo {
	if s.pricing == nil || len(accounts) == 0 {
		return []ModelInfo{}
	}
	modelsByID := make(map[string]ModelInfo)
	for _, m := range s.pricing.ListAllModels() {
		modelsByID[m.ID] = m
	}
	if len(modelsByID) == 0 {
		return []ModelInfo{}
	}

	allowed := make(map[string]ModelInfo)
	for _, account := range accounts {
		if account == nil || !account.IsActive() {
			continue
		}
		if account.Platform == PlatformGeneric && s.endpointRepo != nil {
			// 走唯一口径 genericEndpointModelIDs（与 GetAvailableModels / 准入放行同源，防漂移）。
			ids, openEndpoint := genericEndpointModelIDs(ctx, s.endpointRepo, account)
			if openEndpoint {
				// 空白名单 endpoint = 支持全部 → 兜底全部已启用 catalog。
				for _, m := range s.pricing.ListEnabledCatalogModels() {
					allowed[m.ID] = m
				}
			}
			for _, m := range ids {
				addWhitelistedModel(allowed, modelsByID, m, m)
			}
			// 功能 25 增强：generic 也支持账号级 model_mapping（别名 → 上游模型），与 supported_models
			// 并存共同决定可路由集。别名目标命中已启用 catalog 即可路由，广场显示别名（addWhitelistedModel
			// 的 mappedModelID 分支）。网关转发时 account.GetMappedModel 已把别名改回上游模型名。
			for modelID, mappedModelID := range configuredModelWhitelist(account) {
				addWhitelistedModel(allowed, modelsByID, modelID, mappedModelID)
			}
			continue
		}
		for modelID, mappedModelID := range configuredModelWhitelist(account) {
			addWhitelistedModel(allowed, modelsByID, modelID, mappedModelID)
		}
	}

	out := make([]ModelInfo, 0, len(allowed))
	for _, model := range allowed {
		out = append(out, model)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LiteLLMProvider != out[j].LiteLLMProvider {
			return out[i].LiteLLMProvider < out[j].LiteLLMProvider
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func configuredModelWhitelist(account *Account) map[string]string {
	if account == nil || account.Credentials == nil {
		return nil
	}
	switch raw := account.Credentials["model_mapping"].(type) {
	case map[string]any:
		return cleanModelMapping(raw)
	case map[string]string:
		result := make(map[string]string, len(raw))
		for key, value := range raw {
			if key = strings.TrimSpace(key); key != "" {
				result[key] = strings.TrimSpace(value)
			}
		}
		return result
	default:
		return nil
	}
}

func cleanModelMapping(raw map[string]any) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]string, len(raw))
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if s, ok := value.(string); ok {
			result[key] = strings.TrimSpace(s)
		}
	}
	return result
}

func addWhitelistedModel(allowed, modelsByID map[string]ModelInfo, modelID, mappedModelID string) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return
	}
	if strings.HasSuffix(modelID, "*") {
		for candidateID, candidate := range modelsByID {
			if matchModelWhitelistPattern(modelID, candidateID) {
				allowed[candidateID] = candidate
			}
		}
		return
	}
	if model, ok := modelsByID[modelID]; ok {
		allowed[modelID] = model
		return
	}
	mappedModelID = strings.TrimSpace(mappedModelID)
	if mappedModelID == "" {
		return
	}
	if model, ok := modelsByID[mappedModelID]; ok {
		model.ID = modelID
		allowed[modelID] = model
	}
}

func matchModelWhitelistPattern(pattern, modelID string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(modelID, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == modelID
}
