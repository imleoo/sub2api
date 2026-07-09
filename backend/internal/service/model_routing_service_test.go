//go:build unit

package service

import (
	"context"
	"sort"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// routingAccountStub 仅实现 ModelRoutingService 用到的 GetByIDs / ListActive。
type routingAccountStub struct {
	AccountRepository
	accounts []*Account
}

func (r routingAccountStub) GetByIDs(context.Context, []int64) ([]*Account, error) {
	return r.accounts, nil
}
func (r routingAccountStub) ListActive(context.Context) ([]Account, error) {
	out := make([]Account, 0, len(r.accounts))
	for _, a := range r.accounts {
		out = append(out, *a)
	}
	return out, nil
}

func routedIDs(infos []ModelInfo) []string {
	ids := make([]string, 0, len(infos))
	for _, m := range infos {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	return ids
}

func enabledCatalog(ids map[string]string) []*DBModelPricing {
	out := make([]*DBModelPricing, 0, len(ids))
	for id, provider := range ids {
		out = append(out, &DBModelPricing{ModelID: id, Provider: provider, IsEnabled: true})
	}
	return out
}

func TestRoutableModelInfos_HidesNonWhitelisted(t *testing.T) {
	svc, _ := newCatalogTestService(enabledCatalog(map[string]string{
		"gpt-5.4": "openai", "claude-sonnet-4": "anthropic", "gemini-3-pro": "google",
		"gpt-5.4-openai": "openai", "gpt-5.4-mini": "openai", "gpt-5.4-mini-alt": "openai",
	}))
	mr := NewModelRoutingService(routingAccountStub{accounts: []*Account{
		{ID: 1, Status: StatusActive, Platform: PlatformOpenAI, Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4-openai"},
		}},
		{ID: 2, Status: StatusActive, Platform: PlatformAnthropic, Credentials: map[string]any{}},
	}}, nil, svc)

	got := mr.RoutableModelInfos(context.Background(), []int64{1, 2})
	require.Equal(t, []string{"gpt-5.4"}, routedIDs(got))
}

// 后台「模型折扣」勾选「只看可见」时须与用户模型广场口径一致：
// OperatorRoutableModelInfos（全 active 账号可路由）叠加 FilterVisibleModels（海外开关）
// 后，关闭海外模型应同步隐藏海外模型，仅留国产 —— 与广场关闭海外后显示完全相同。
func TestOperatorRoutableModelInfos_VisibleFilterMatchesSquare(t *testing.T) {
	svc, _ := newCatalogTestService(enabledCatalog(map[string]string{
		"gpt-5.4": "openai", "deepseek-chat": "deepseek",
	}))
	mr := NewModelRoutingService(routingAccountStub{accounts: []*Account{
		{ID: 1, Status: StatusActive, Platform: PlatformOpenAI, Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4", "deepseek-chat": "deepseek-chat"},
		}},
	}}, nil, svc)

	infos, err := mr.OperatorRoutableModelInfos(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"deepseek-chat", "gpt-5.4"}, routedIDs(infos))

	// 海外开启：全部可见（与广场一致）。
	require.Equal(t, []string{"deepseek-chat", "gpt-5.4"}, routedIDs(FilterVisibleModels(infos, true)))
	// 海外关闭：海外模型隐藏，仅留国产（= 广场关闭海外后的显示）。
	require.Equal(t, []string{"deepseek-chat"}, routedIDs(FilterVisibleModels(infos, false)))
}

// 功能 25 增强：generic 账号的可路由集 = endpoint supported_models ∪ 账号级 model_mapping（别名）。
// 别名目标命中已启用 catalog 时以别名 ID 进入可路由集，与 supported_models 并存。
func TestRoutableModelInfos_GenericFoldsSupportedModelsAndMapping(t *testing.T) {
	svc, _ := newCatalogTestService(enabledCatalog(map[string]string{
		"yi-large": "nvidia", "aliased-target": "nvidia",
	}))
	epRepo := &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{
		7: {{ID: 1, OutboundProtocol: "openai_chat", SupportedModels: []string{"yi-large"}}},
	}}
	mr := NewModelRoutingService(routingAccountStub{accounts: []*Account{
		{ID: 7, Status: StatusActive, Platform: PlatformGeneric, Credentials: map[string]any{
			"model_mapping": map[string]any{"my-alias": "aliased-target"},
		}},
	}}, epRepo, svc)

	got := mr.RoutableModelInfos(context.Background(), []int64{7})
	require.Equal(t, []string{"my-alias", "yi-large"}, routedIDs(got))
}

// GetAvailableModels（网关 /v1/models）按运行模式分口径：
//   - 标准模式：委托 routableFromAccounts → 与广场收敛（catalog 交集，未定价模型被滤）。
//   - simple 模式：raw（计费关闭，未定价也可调）。
func TestGetAvailableModels_ModeAwareCatalogIntersection(t *testing.T) {
	groupID := int64(21)
	// catalog 只启用 yi-large；nemotron-4 不在 catalog（未定价）。
	pricingSvc, _ := newCatalogTestService(enabledCatalog(map[string]string{"yi-large": "nvidia"}))
	epRepo := &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{
		7: {{ID: 1, OutboundProtocol: "openai_chat", SupportedModels: []string{"yi-large", "nemotron-4"}}},
	}}
	// GetAvailableModels 通过 accountRepo.ListSchedulableByGroupID 取账号，再传给 routableFromAccounts；
	// modelRouting 的 accountRepo 不被 routableFromAccounts 使用（它直接收 accounts），置 nil 即可。
	accountRepo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{
		groupID: {{ID: 7, Status: StatusActive, Platform: PlatformGeneric, Credentials: map[string]any{}}},
	}}
	mr := NewModelRoutingService(nil, epRepo, pricingSvc)

	newGw := func(runMode string) *GatewayService {
		return &GatewayService{
			accountRepo:        accountRepo,
			endpointRepo:       epRepo,
			modelRouting:       mr,
			cfg:                &config.Config{RunMode: runMode},
			modelsListCache:    gocache.New(time.Minute, time.Minute),
			modelsListCacheTTL: time.Minute,
		}
	}

	// 标准模式：catalog 交集 → 只剩已启用的 yi-large（nemotron-4 未定价被滤）。
	std := newGw(config.RunModeStandard).GetAvailableModels(context.Background(), &groupID, PlatformGeneric)
	require.Equal(t, []string{"yi-large"}, std)

	// simple 模式：raw → supported_models 全出（含未定价 nemotron-4）。
	simple := newGw(config.RunModeSimple).GetAvailableModels(context.Background(), &groupID, PlatformGeneric)
	require.Equal(t, []string{"nemotron-4", "yi-large"}, simple)
}

// raw 口径（simple 模式）下「空白名单 endpoint = 支持全部」无法枚举全部，用已启用 catalog 兜底
// （此前丢弃 openEndpoint 标志 → 一个模型都列不出来 → handler 回退误导性默认列表）。
func TestGetAvailableModels_SimpleModeOpenEndpointFallsBackToEnabledCatalog(t *testing.T) {
	groupID := int64(22)
	pricingSvc, _ := newCatalogTestService(enabledCatalog(map[string]string{"yi-large": "nvidia"}))
	epRepo := &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{
		// SupportedModels 为空 = 空白名单 endpoint。
		8: {{ID: 2, OutboundProtocol: "openai_chat"}},
	}}
	accountRepo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{
		groupID: {{ID: 8, Status: StatusActive, Platform: PlatformGeneric, Credentials: map[string]any{}}},
	}}
	gw := &GatewayService{
		accountRepo:        accountRepo,
		endpointRepo:       epRepo,
		modelRouting:       NewModelRoutingService(nil, epRepo, pricingSvc),
		cfg:                &config.Config{RunMode: config.RunModeSimple},
		modelsListCache:    gocache.New(time.Minute, time.Minute),
		modelsListCacheTTL: time.Minute,
	}

	got := gw.GetAvailableModels(context.Background(), &groupID, PlatformGeneric)
	require.Equal(t, []string{"yi-large"}, got)
}

// genericEndpointSupportsModel 补回「generic 配了别名映射后，其余 supported_models 直连仍可服务」。
func TestGenericEndpointSupportsModel_EligibilityFallback(t *testing.T) {
	epRepo := &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{
		7: {{ID: 1, OutboundProtocol: "openai_chat", SupportedModels: []string{"yi-large"}}},
	}}
	generic := &Account{ID: 7, Status: StatusActive, Platform: PlatformGeneric, Credentials: map[string]any{
		"model_mapping": map[string]any{"my-alias": "aliased-target"},
	}}
	ctx := context.Background()

	// mapping 非空 → IsModelSupported 挡掉 supported_models 直连；helper 补回。
	require.False(t, generic.IsModelSupported("yi-large"))
	require.True(t, genericEndpointSupportsModel(ctx, epRepo, generic, "yi-large"))
	// mapping 内别名 IsModelSupported 直接放行。
	require.True(t, generic.IsModelSupported("my-alias"))
	// supported_models 外的模型仍拒绝。
	require.False(t, genericEndpointSupportsModel(ctx, epRepo, generic, "not-served"))
	// 非 generic 账号：helper 一律 false（不改变既有行为）。
	nonGeneric := &Account{ID: 8, Status: StatusActive, Platform: PlatformOpenAI}
	require.False(t, genericEndpointSupportsModel(ctx, epRepo, nonGeneric, "yi-large"))
}

func TestRoutableModelInfos_ExpandsWildcardAndMappedAlias(t *testing.T) {
	svc, _ := newCatalogTestService(enabledCatalog(map[string]string{
		"gpt-5.4-mini": "openai", "gpt-5.4-mini-alt": "openai",
		"gpt-5.3-codex-spark": "openai", "gpt-5.4": "openai",
	}))
	mr := NewModelRoutingService(routingAccountStub{accounts: []*Account{
		{ID: 1, Status: StatusActive, Platform: PlatformOpenAI, Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-5.4-mini*":     "gpt-5.4-mini*",
				"gpt-5.3-codex":     "gpt-5.3-codex-spark",
				"not-priced-custom": "not-priced-upstream",
			},
		}},
	}}, nil, svc)

	got := mr.RoutableModelInfos(context.Background(), []int64{1})
	require.Equal(t, []string{"gpt-5.3-codex", "gpt-5.4-mini", "gpt-5.4-mini-alt"}, routedIDs(got))
}
