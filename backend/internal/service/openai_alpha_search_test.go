package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type alphaSearchAccountStateRepo struct {
	AccountRepository
	setErrorCalls int
	lastError     string
}

func (r *alphaSearchAccountStateRepo) SetError(_ context.Context, _ int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastError = errorMsg
	return nil
}

func TestForwardAlphaSearchAPIKeyMapsModelAndPassesThroughError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"id":"search-session","model":"gpt-5.6-sol","commands":{"search_query":[{"q":"news"}]}}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/alpha/search", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := `{"error":{"type":"invalid_request_error","message":"bad search"}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	service := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:       7,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat.example/v4",
			"model_mapping": map[string]any{
				"gpt-5.6-sol": "upstream-5.6",
			},
		},
	}

	result, err := service.ForwardAlphaSearch(context.Background(), c, account, body)

	require.NoError(t, err)
	// 上游错误透传不是一次成功的搜索：不返回 result、不产生按次计费。
	require.Nil(t, result)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, upstreamBody, recorder.Body.String())
	require.Equal(t, "https://compat.example/v4/alpha/search", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "upstream-5.6", gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "commands.search_query").IsArray())
}

func TestForwardAlphaSearchReturnsFailoverBeforeWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"id":"search-session","model":"gpt-5.6-sol","commands":{}}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
	}}
	service := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:       8,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := service.ForwardAlphaSearch(context.Background(), c, account, body)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Equal(t, openAIPlatformAlphaSearchURL, upstream.lastReq.URL.String())
	require.False(t, c.Writer.Written())
	require.Empty(t, recorder.Body.String())
}

// 上游 0.1.158 回补：apikey 上游不支持 /v1/alpha/search（404/405）时应换号
// failover 且不写账号错误状态，而非把 404 透传给客户端。
func TestForwardAlphaSearchAPIKeyEndpointNotFoundFailsOver(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"id":"search-session","model":"gpt-5.6-sol","commands":{"search_query":[{"q":"news"}]}}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Not Found"}}`)),
	}}
	repo := &alphaSearchAccountStateRepo{}
	cfg := &config.Config{}
	service := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		accountRepo:      repo,
		rateLimitService: NewRateLimitService(repo, nil, cfg, nil, nil),
	}
	account := &Account{
		ID:       9,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://relay.example",
		},
	}

	result, err := service.ForwardAlphaSearch(context.Background(), c, account, body)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
	require.Zero(t, repo.setErrorCalls)
	require.Empty(t, repo.lastError)
	require.False(t, c.Writer.Written())
	require.Empty(t, recorder.Body.String())
}

// 上游 0.1.158 回补：alpha/search 的 401 允许换号但不得把健康账号标坏。
func TestForwardAlphaSearchUnauthorizedDoesNotMarkAccountError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"id":"search-session","model":"gpt-5.6-sol","commands":{"search_query":[{"q":"news"}]}}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unauthorized"}}`)),
	}}
	repo := &alphaSearchAccountStateRepo{}
	cfg := &config.Config{}
	service := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		accountRepo:      repo,
		rateLimitService: NewRateLimitService(repo, nil, cfg, nil, nil),
	}
	account := &Account{
		ID:       11,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := service.ForwardAlphaSearch(context.Background(), c, account, body)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusUnauthorized, failoverErr.StatusCode)
	require.Zero(t, repo.setErrorCalls)
	require.False(t, c.Writer.Written())
}

// 上游 0.1.158 回补：发送前剥离 Responses 公共字段（prompt_cache_key 等），
// 避免第三方代理误带字段被上游拒绝为 Unknown parameter。
func TestForwardAlphaSearchSanitizesResponsesFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"id":"search-session","model":"gpt-5.6-sol","prompt_cache_key":"cache-key","prompt_cache_retention":"24h","commands":{"search_query":[{"q":"news"}]}}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"results":[]}`)),
	}}
	service := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:       12,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := service.ForwardAlphaSearch(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_retention").Exists())
	require.True(t, gjson.GetBytes(upstream.lastBody, "commands.search_query").IsArray())
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestShouldApplyOpenAIAlphaSearchAccountErrorSideEffects(t *testing.T) {
	require.False(t, shouldApplyOpenAIAlphaSearchAccountErrorSideEffects(http.StatusUnauthorized))
	require.False(t, shouldApplyOpenAIAlphaSearchAccountErrorSideEffects(http.StatusNotFound))
	require.False(t, shouldApplyOpenAIAlphaSearchAccountErrorSideEffects(http.StatusMethodNotAllowed))
	require.True(t, shouldApplyOpenAIAlphaSearchAccountErrorSideEffects(http.StatusForbidden))
	require.True(t, shouldApplyOpenAIAlphaSearchAccountErrorSideEffects(http.StatusTooManyRequests))
}

func TestIsOpenAIAlphaSearchEndpointUnsupported(t *testing.T) {
	apiKey := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	bedrock := &Account{Platform: PlatformOpenAI, Type: AccountTypeBedrock}

	require.True(t, isOpenAIAlphaSearchEndpointUnsupported(apiKey, http.StatusNotFound))
	require.True(t, isOpenAIAlphaSearchEndpointUnsupported(apiKey, http.StatusMethodNotAllowed))
	require.False(t, isOpenAIAlphaSearchEndpointUnsupported(apiKey, http.StatusBadRequest))
	require.False(t, isOpenAIAlphaSearchEndpointUnsupported(bedrock, http.StatusNotFound))
	require.False(t, isOpenAIAlphaSearchEndpointUnsupported(nil, http.StatusNotFound))
}
