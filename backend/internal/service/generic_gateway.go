package service

import (
	"context"
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

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
