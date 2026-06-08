//go:build unit

package service

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
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
