package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func upstreamModelSyncTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
}

func TestBuildV1ModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1/models"))
	require.Equal(t, "https://gateway.example.com/proxy/v1/models", buildV1ModelsURL("https://gateway.example.com/proxy/"))
}

func TestBuildGeminiModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta/models"))
}

func TestExtractUpstreamModelIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "openai and anthropic data array",
			body: `{"data":[{"id":"claude-sonnet-4-5"},{"id":"gpt-5"},{"id":"gpt-5"},{"id":""}]}`,
			want: []string{"claude-sonnet-4-5", "gpt-5"},
		},
		{
			name: "gemini models array strips prefix",
			body: `{"models":[{"name":"models/gemini-2.5-pro"},{"name":"gemini-2.5-flash"}]}`,
			want: []string{"gemini-2.5-flash", "gemini-2.5-pro"},
		},
		{
			name: "top level array",
			body: `[{"id":"z-model"},{"name":"models/a-model"}]`,
			want: []string{"a-model", "z-model"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := extractUpstreamModelIDs([]byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildUpstreamModelsRequestsForAPIKeyAccounts(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	ctx := context.Background()

	anthropicReq, err := svc.buildAnthropicUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "anthropic-key",
			"base_url": "https://anthropic.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://anthropic.example.com/v1/models", anthropicReq.URL.String())
	require.Equal(t, "anthropic-key", anthropicReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", anthropicReq.Header.Get("anthropic-version"))

	anthropicBearerReq, err := svc.buildAnthropicUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "ollama-key",
			"base_url": "https://ollama.com",
		},
		Extra: map[string]any{
			"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://ollama.com/v1/models", anthropicBearerReq.URL.String())
	require.Equal(t, "Bearer ollama-key", anthropicBearerReq.Header.Get("Authorization"))
	require.Empty(t, anthropicBearerReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", anthropicBearerReq.Header.Get("anthropic-version"))

	openAIReq, err := svc.buildOpenAIUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://openai.example.com/v1/models", openAIReq.URL.String())
	require.Equal(t, "Bearer openai-key", openAIReq.Header.Get("Authorization"))

	geminiReq, err := svc.buildGeminiUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com/v1beta",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", geminiReq.URL.String())
	require.Equal(t, "gemini-key", geminiReq.Header.Get("x-goog-api-key"))
}

func TestBuildAnthropicUpstreamModelsRequestRejectsBedrock(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildAnthropicUpstreamModelsRequest(context.Background(), &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeBedrock,
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
}

func TestFetchModelsByConfigParsesNewAPIVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "openai data array with owned_by (newapi/maas)",
			body: `{"data":[{"id":"qwen3.7-max","object":"model","owned_by":"organization"},{"id":"glm-4.6","object":"model"}]}`,
			want: []string{"glm-4.6", "qwen3.7-max"},
		},
		{
			name: "models array",
			body: `{"models":[{"id":"deepseek-v4-pro"},{"name":"kimi-k2.6"}]}`,
			want: []string{"deepseek-v4-pro", "kimi-k2.6"},
		},
		{
			name: "bare array",
			body: `[{"id":"minimax-m2.5"},{"id":"mog-5"}]`,
			want: []string{"minimax-m2.5", "mog-5"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}}
			svc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

			models, err := svc.FetchModelsByConfig(
				context.Background(),
				"https://maas-openapi.wanjiedata.com/api",
				"wanjie-key",
				"",
				"",
				"",
			)
			require.NoError(t, err)
			require.Equal(t, tc.want, models)
			// base_url 末尾为 /api，应拼成 /api/v1/models
			require.Equal(t, "https://maas-openapi.wanjiedata.com/api/v1/models", upstream.lastReq.URL.String())
			// authHeader/authScheme 空时回退为 Authorization: Bearer <key>
			require.Equal(t, "Bearer wanjie-key", upstream.lastReq.Header.Get("Authorization"))
		})
	}
}

func TestFetchModelsByConfigCustomAuthHeaderNoScheme(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"m1"}]}`)),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

	models, err := svc.FetchModelsByConfig(
		context.Background(),
		"https://relay.example.com/v1",
		"raw-key",
		"x-api-key",
		"",
		"",
	)
	require.NoError(t, err)
	require.Equal(t, []string{"m1"}, models)
	require.Equal(t, "https://relay.example.com/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "raw-key", upstream.lastReq.Header.Get("x-api-key"))
	require.Empty(t, upstream.lastReq.Header.Get("Authorization"))
}

func TestFetchModelsByConfigRequiresBaseURLAndKey(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{httpUpstream: &httpUpstreamRecorder{}, cfg: upstreamModelSyncTestConfig()}

	_, err := svc.FetchModelsByConfig(context.Background(), "", "key", "", "", "")
	require.Error(t, err)
	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorConfiguration, syncErr.Kind)

	_, err = svc.FetchModelsByConfig(context.Background(), "https://relay.example.com", "", "", "", "")
	require.Error(t, err)
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorConfiguration, syncErr.Kind)
}

func TestFetchModelsByConfigDoesNotExposeUpstreamBody(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"error":"SECRET_TOKEN leaked"}`)),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

	_, err := svc.FetchModelsByConfig(context.Background(), "https://relay.example.com", "key", "", "", "")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET_TOKEN")

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUpstream, syncErr.Kind)
	require.Contains(t, syncErr.SafeMessage(), "HTTP 401")
}

func TestFetchUpstreamSupportedModelsParsesOpenAIResponse(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5"},{"id":"gpt-5"},{"name":"o3"}]}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       7,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5", "o3"}, models)
	require.Equal(t, "https://openai.example.com/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer openai-key", upstream.lastReq.Header.Get("Authorization"))
}

func TestFetchUpstreamSupportedModelsDoesNotExposeUpstreamBody(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"SECRET_TOKEN should not be exposed"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	_, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       8,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET_TOKEN")

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUpstream, syncErr.Kind)
	require.NotContains(t, syncErr.SafeMessage(), "SECRET_TOKEN")
	require.Contains(t, syncErr.SafeMessage(), "HTTP 502")
}
