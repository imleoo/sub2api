package service

import (
	"context"
	"slices"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// genericEndpointModelIDs 是「generic 账号暴露哪些上游模型」的**唯一口径**：返回所有 endpoint 的
// supported_models 并集（去空去重），并回报是否存在「空白名单 endpoint」（openEndpoint，语义=支持
// 全部，由调用方决定是否兜底全 catalog）。
//
// routableFromAccounts（广场/后台可路由集）、GetAvailableModels（网关 /v1/models）、
// genericEndpointSupportsModel（准入放行）三处共用此函数，避免 generic 模型来源再次漂移
// （历史 bug：/v1/models 只读 model_mapping 漏了 supported_models）。各路径的 catalog 交集 / 兜底
// 语义按用途在各自 wrapper 里处理，本函数只负责「原始暴露集」这一层。
func genericEndpointModelIDs(ctx context.Context, repo EndpointRepository, account *Account) (models []string, openEndpoint bool) {
	if account == nil || repo == nil {
		return nil, false
	}
	eps, _ := repo.ListByAccountID(ctx, account.ID)
	seen := make(map[string]struct{})
	for _, ep := range eps {
		if len(ep.SupportedModels) == 0 {
			openEndpoint = true
			continue
		}
		for _, m := range ep.SupportedModels {
			if m = strings.TrimSpace(m); m != "" {
				if _, ok := seen[m]; !ok {
					seen[m] = struct{}{}
					models = append(models, m)
				}
			}
		}
	}
	return models, openEndpoint
}

// genericEndpointSupportsModel 查 generic 账号是否能服务该请求模型（命中 supported_models，或存在
// 空白名单 endpoint = 支持全部）。走唯一口径 genericEndpointModelIDs。
//
// 用途（功能 25 增强）：generic 账号一旦配置了账号级 model_mapping（别名），Account.IsModelSupported
// 会转为「仅认映射内模型」，从而误挡该账号其余 supported_models 的直连请求。各网关准入点在
// IsModelSupported 未命中后 OR 上本函数，把 supported_models 直连放行补回。纯增量、单调放宽：
// 非 generic 账号直接返回 false，不改变任何既有行为。
func genericEndpointSupportsModel(ctx context.Context, repo EndpointRepository, account *Account, requestedModel string) bool {
	if account == nil || account.Platform != PlatformGeneric || repo == nil {
		return false
	}
	rm := strings.TrimSpace(requestedModel)
	if rm == "" {
		return false
	}
	ids, openEndpoint := genericEndpointModelIDs(ctx, repo, account)
	return openEndpoint || slices.Contains(ids, rm)
}

// generic 各入站协议可直通的出站协议族（直通 = 入站==出站，无需协议桥）。
var (
	genericOpenAIChatProtocols = []string{"openai_chat", "openai_responses"}
	genericAnthropicProtocols  = []string{"anthropic_messages"}
	genericGeminiProtocols     = []string{"gemini_v1beta"}
)

// selectGenericEndpointForProtocols 从端点列表中选出 health=healthy 且
// outbound_protocol 命中 protocols 的最高优先级端点（priority ASC，ID 次之）。
// 未命中返回 nil。
func selectGenericEndpointForProtocols(endpoints []*DBEndpoint, protocols []string) *DBEndpoint {
	healthy := SelectHealthyEndpoints(endpoints)
	for _, ep := range healthy {
		if slices.Contains(protocols, ep.OutboundProtocol) {
			return ep
		}
	}
	return nil
}

// resolveGenericEndpointVia 加载 generic 账号端点并选出匹配 protocols 的目标端点。
// repo 未注入、账号非 generic 或无匹配端点时返回 nil。供各网关服务共用。
func resolveGenericEndpointVia(ctx context.Context, repo EndpointRepository, account *Account, protocols []string) *DBEndpoint {
	if repo == nil || account == nil || !account.IsGeneric() {
		return nil
	}
	eps, err := repo.ListByAccountID(ctx, account.ID)
	if err != nil || len(eps) == 0 {
		return nil
	}
	return selectGenericEndpointForProtocols(eps, protocols)
}

// --- OpenAI 网关侧 ---

func (s *OpenAIGatewayService) resolveGenericEndpoint(ctx context.Context, account *Account, protocols []string) *DBEndpoint {
	return resolveGenericEndpointVia(ctx, s.endpointRepo, account, protocols)
}

func (s *OpenAIGatewayService) genericAccountHasOpenAIEndpoint(ctx context.Context, account *Account) bool {
	return s.resolveGenericEndpoint(ctx, account, genericOpenAIChatProtocols) != nil
}

func (s *OpenAIGatewayService) genericOpenAIBaseURL(ctx context.Context, account *Account) string {
	ep := s.resolveGenericEndpoint(ctx, account, genericOpenAIChatProtocols)
	if ep == nil {
		return ""
	}
	return ep.BaseURL
}

// --- Anthropic / Gemini 网关侧（GatewayService）---

func (s *GatewayService) genericRuntimeEnabled() bool {
	return s.cfg != nil && s.cfg.Gateway.Scheduling.GenericRuntimeEnabled
}

// genericProtocolsForPlatform 返回入站 platform 对应的 generic 出站协议族。
func genericProtocolsForPlatform(platform string) []string {
	switch platform {
	case PlatformAnthropic:
		return genericAnthropicProtocols
	case PlatformGemini:
		return genericGeminiProtocols
	default:
		return nil
	}
}

// resolveGenericEndpointForPlatform 解析 generic 账号在指定入站 platform 下的目标端点。
func (s *GatewayService) resolveGenericEndpointForPlatform(ctx context.Context, account *Account, platform string) *DBEndpoint {
	protocols := genericProtocolsForPlatform(platform)
	if protocols == nil {
		return nil
	}
	return resolveGenericEndpointVia(ctx, s.endpointRepo, account, protocols)
}

// genericAnthropicBaseURL 返回 generic 账号 anthropic 端点 base_url；无则 ""。
func (s *GatewayService) genericAnthropicBaseURL(ctx context.Context, account *Account) string {
	ep := s.resolveGenericEndpointForPlatform(ctx, account, PlatformAnthropic)
	if ep == nil {
		return ""
	}
	return ep.BaseURL
}

// --- Gemini 网关侧（GeminiMessagesCompatService）---

// genericGeminiBaseURL 返回 generic 账号 gemini 端点 base_url；无则 ""。
func (s *GeminiMessagesCompatService) genericGeminiBaseURL(ctx context.Context, account *Account) string {
	ep := resolveGenericEndpointVia(ctx, s.endpointRepo, account, genericGeminiProtocols)
	if ep == nil {
		return ""
	}
	return ep.BaseURL
}

// listGenericAccountsForPlatform 返回组内（或全局）具备 platform 对应协议健康端点的
// generic 账号；flag 关闭或 endpointRepo 未注入时返回 nil。
func (s *GatewayService) listGenericAccountsForPlatform(ctx context.Context, groupID *int64, platform string) []Account {
	if !s.genericRuntimeEnabled() || s.endpointRepo == nil || s.accountRepo == nil {
		return nil
	}
	if genericProtocolsForPlatform(platform) == nil {
		return nil
	}
	var generic []Account
	var err error
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		generic, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformGeneric)
	} else if groupID != nil {
		generic, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformGeneric)
	} else {
		generic, err = s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformGeneric)
	}
	if err != nil || len(generic) == 0 {
		return nil
	}
	out := make([]Account, 0, len(generic))
	for i := range generic {
		if s.resolveGenericEndpointForPlatform(ctx, &generic[i], platform) != nil {
			out = append(out, generic[i])
		}
	}
	return out
}
