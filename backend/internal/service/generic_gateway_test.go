package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeEndpointRepo 仅实现 ListByAccountID（其它方法返回零值），供 generic 单测使用。
type fakeEndpointRepo struct {
	byAccount map[int64][]*DBEndpoint
}

func (f *fakeEndpointRepo) Create(context.Context, *DBEndpoint) error { return nil }
func (f *fakeEndpointRepo) Update(context.Context, *DBEndpoint) error { return nil }
func (f *fakeEndpointRepo) Delete(context.Context, int64) error       { return nil }
func (f *fakeEndpointRepo) GetByID(context.Context, int64) (*DBEndpoint, error) {
	return nil, nil
}
func (f *fakeEndpointRepo) FindByStableID(context.Context, int64, string) (*DBEndpoint, error) {
	return nil, nil
}
func (f *fakeEndpointRepo) UpdateSupportedModels(context.Context, int64, []string) error {
	return nil
}
func (f *fakeEndpointRepo) ListByAccountID(_ context.Context, id int64) ([]*DBEndpoint, error) {
	return f.byAccount[id], nil
}

func TestSelectGenericEndpointForProtocols(t *testing.T) {
	t.Parallel()

	eps := []*DBEndpoint{
		{ID: 1, OutboundProtocol: "anthropic_messages", BaseURL: "https://x/api/anthropic", Health: "healthy", Priority: 100},
		{ID: 2, OutboundProtocol: "openai_chat", BaseURL: "https://x/api", Health: "healthy", Priority: 50},
		{ID: 3, OutboundProtocol: "openai_chat", BaseURL: "https://x/api-degraded", Health: "degraded", Priority: 10},
		{ID: 4, OutboundProtocol: "gemini_v1beta", BaseURL: "https://x/api", Health: "healthy", Priority: 100},
	}

	// openai 协议族：跳过 degraded（ID3，优先级更高但不健康），命中 healthy 的 ID2
	got := selectGenericEndpointForProtocols(eps, genericOpenAIChatProtocols)
	require.NotNil(t, got)
	require.Equal(t, int64(2), got.ID)

	// anthropic 协议
	got = selectGenericEndpointForProtocols(eps, []string{"anthropic_messages"})
	require.NotNil(t, got)
	require.Equal(t, int64(1), got.ID)

	// 无匹配协议
	require.Nil(t, selectGenericEndpointForProtocols(eps, []string{"unknown_proto"}))

	// 全部 disabled → nil
	disabled := []*DBEndpoint{{ID: 9, OutboundProtocol: "openai_chat", Health: "disabled", Priority: 1}}
	require.Nil(t, selectGenericEndpointForProtocols(disabled, genericOpenAIChatProtocols))
}

func TestSelectGenericEndpointPicksLowestPriority(t *testing.T) {
	t.Parallel()

	eps := []*DBEndpoint{
		{ID: 1, OutboundProtocol: "openai_chat", Health: "healthy", Priority: 200},
		{ID: 2, OutboundProtocol: "openai_chat", Health: "healthy", Priority: 100},
	}
	got := selectGenericEndpointForProtocols(eps, genericOpenAIChatProtocols)
	require.NotNil(t, got)
	require.Equal(t, int64(2), got.ID, "priority 越小越优先")
}

func TestGenericProtocolsForPlatform(t *testing.T) {
	t.Parallel()

	require.Equal(t, genericAnthropicProtocols, genericProtocolsForPlatform(PlatformAnthropic))
	require.Equal(t, genericGeminiProtocols, genericProtocolsForPlatform(PlatformGemini))
	// openai 由 OpenAIGatewayService 独立处理（不走 genericProtocolsForPlatform）
	require.Nil(t, genericProtocolsForPlatform(PlatformOpenAI))
}

// wanjieEndpoints 模拟万界方舟的三协议端点配置。
func wanjieEndpoints() []*DBEndpoint {
	return []*DBEndpoint{
		{ID: 1, AccountID: 42, OutboundProtocol: "openai_chat", BaseURL: "https://maas-openapi.wanjiedata.com/api", AuthHeader: "Authorization", AuthScheme: "Bearer", Health: "healthy", Priority: 100, StableID: "wanjie-openai"},
		{ID: 2, AccountID: 42, OutboundProtocol: "anthropic_messages", BaseURL: "https://maas-openapi.wanjiedata.com/api/anthropic", AuthHeader: "x-api-key", AuthScheme: "", Health: "healthy", Priority: 100, StableID: "wanjie-anthropic"},
		{ID: 3, AccountID: 42, OutboundProtocol: "gemini_v1beta", BaseURL: "https://maas-openapi.wanjiedata.com/api", AuthHeader: "x-goog-api-key", AuthScheme: "", Health: "healthy", Priority: 100, StableID: "wanjie-gemini"},
	}
}

func TestOpenAIGatewayService_GenericOpenAIBaseURL(t *testing.T) {
	t.Parallel()

	s := &OpenAIGatewayService{
		endpointRepo: &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{42: wanjieEndpoints()}},
	}
	generic := &Account{ID: 42, Platform: PlatformGeneric, Type: AccountTypeAPIKey}
	require.Equal(t, "https://maas-openapi.wanjiedata.com/api", s.genericOpenAIBaseURL(context.Background(), generic))

	// 非 generic 账号 → ""
	non := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	require.Equal(t, "", s.genericOpenAIBaseURL(context.Background(), non))

	// 无匹配端点 → ""
	empty := &OpenAIGatewayService{endpointRepo: &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{}}}
	require.Equal(t, "", empty.genericOpenAIBaseURL(context.Background(), generic))
}

func TestGatewayService_GenericAnthropicBaseURL(t *testing.T) {
	t.Parallel()

	s := &GatewayService{
		endpointRepo: &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{42: wanjieEndpoints()}},
	}
	generic := &Account{ID: 42, Platform: PlatformGeneric, Type: AccountTypeAPIKey}
	require.Equal(t, "https://maas-openapi.wanjiedata.com/api/anthropic", s.genericAnthropicBaseURL(context.Background(), generic))
}

func TestGeminiMessagesCompatService_GenericGeminiBaseURL(t *testing.T) {
	t.Parallel()

	s := &GeminiMessagesCompatService{
		endpointRepo: &fakeEndpointRepo{byAccount: map[int64][]*DBEndpoint{42: wanjieEndpoints()}},
	}
	generic := &Account{ID: 42, Platform: PlatformGeneric, Type: AccountTypeAPIKey}
	require.Equal(t, "https://maas-openapi.wanjiedata.com/api", s.genericGeminiBaseURL(context.Background(), generic))
}

func TestAccountIsGenericAndAPIKey(t *testing.T) {
	t.Parallel()

	generic := &Account{
		Platform:    PlatformGeneric,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "wanjie-key"},
	}
	require.True(t, generic.IsGeneric())
	require.Equal(t, "wanjie-key", generic.GetGenericAPIKey())

	openai := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-x"}}
	require.False(t, openai.IsGeneric())
	require.Equal(t, "", openai.GetGenericAPIKey(), "非 generic 账号返回空")
}
