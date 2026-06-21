package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func f64p(v float64) *float64 { return &v }

type httpUpstreamRecorder struct {
	lastReq  *http.Request
	lastBody []byte
	requests []*http.Request
	bodies   [][]byte

	resp      *http.Response
	responses []*http.Response
	err       error
}

func (u *httpUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		u.bodies = append(u.bodies, append([]byte(nil), b...))
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	u.requests = append(u.requests, req)
	if u.err != nil {
		return nil, u.err
	}
	if len(u.responses) > 0 {
		resp := u.responses[0]
		u.responses = u.responses[1:]
		return resp, nil
	}
	return u.resp, nil
}

func (u *httpUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestOpenAIGatewayService_NativeResponsesBodyModificationPreservesHTMLChars(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payloadText := strings.Repeat(`<tag>&value</tag>`, 128)
	originalBody := []byte(fmt.Sprintf(`{"model":"gpt-5.5","stream":false,"max_output_tokens":100,"previous_response_id":"resp_prev","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}]}`, payloadText))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_native_reencode"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after capture"}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
			Enabled:           false,
			AllowInsecureHTTP: true,
		}}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          456,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode:      string(openai_compat.ResponsesSupportModeAuto),
			openai_compat.ExtraKeyResponsesSupported: true,
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "http://upstream.example/v1/responses", upstream.lastReq.URL.String())
	require.Contains(t, string(upstream.lastBody), payloadText)
	require.NotContains(t, string(upstream.lastBody), `\\u003c`)
	require.NotContains(t, string(upstream.lastBody), `\\u003e`)
	require.NotContains(t, string(upstream.lastBody), `\\u0026`)
}

type openAIPassthroughFailoverRepo struct {
	stubOpenAIAccountRepo
	rateLimitCalls []time.Time
	overloadCalls  []time.Time
}

func (r *openAIPassthroughFailoverRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	r.rateLimitCalls = append(r.rateLimitCalls, resetAt)
	return nil
}

func (r *openAIPassthroughFailoverRepo) SetOverloaded(_ context.Context, _ int64, until time.Time) error {
	r.overloadCalls = append(r.overloadCalls, until)
	return nil
}

var structuredLogCaptureMu sync.Mutex

type inMemoryLogSink struct {
	mu     sync.Mutex
	events []*logger.LogEvent
}

func (s *inMemoryLogSink) WriteLogEvent(event *logger.LogEvent) {
	if event == nil {
		return
	}
	cloned := *event
	if event.Fields != nil {
		cloned.Fields = make(map[string]any, len(event.Fields))
		for k, v := range event.Fields {
			cloned.Fields[k] = v
		}
	}
	s.mu.Lock()
	s.events = append(s.events, &cloned)
	s.mu.Unlock()
}

func (s *inMemoryLogSink) ContainsMessage(substr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.events {
		if ev != nil && strings.Contains(ev.Message, substr) {
			return true
		}
	}
	return false
}

func (s *inMemoryLogSink) ContainsMessageAtLevel(substr, level string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	wantLevel := strings.ToLower(strings.TrimSpace(level))
	for _, ev := range s.events {
		if ev == nil {
			continue
		}
		if strings.Contains(ev.Message, substr) && strings.ToLower(strings.TrimSpace(ev.Level)) == wantLevel {
			return true
		}
	}
	return false
}

func (s *inMemoryLogSink) ContainsFieldValue(field, substr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.events {
		if ev == nil || ev.Fields == nil {
			continue
		}
		if v, ok := ev.Fields[field]; ok && strings.Contains(fmt.Sprint(v), substr) {
			return true
		}
	}
	return false
}

func (s *inMemoryLogSink) ContainsField(field string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.events {
		if ev == nil || ev.Fields == nil {
			continue
		}
		if _, ok := ev.Fields[field]; ok {
			return true
		}
	}
	return false
}

func captureStructuredLog(t *testing.T) (*inMemoryLogSink, func()) {
	t.Helper()
	structuredLogCaptureMu.Lock()

	err := logger.Init(logger.InitOptions{
		Level:       "debug",
		Format:      "json",
		ServiceName: "tokenpanel",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	sink := &inMemoryLogSink{}
	logger.SetSink(sink)
	return sink, func() {
		logger.SetSink(nil)
		structuredLogCaptureMu.Unlock()
	}
}

func TestOpenAIGatewayService_OpenAIPassthrough_429And529TriggerFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalBody := []byte(`{"model":"gpt-5.2","stream":false,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	newAccount := func(accountType string) *Account {
		account := &Account{
			ID:             123,
			Name:           "acc",
			Platform:       PlatformOpenAI,
			Type:           accountType,
			Concurrency:    1,
			Extra:          map[string]any{"openai_passthrough": true},
			Status:         StatusActive,
			Schedulable:    true,
			RateMultiplier: f64p(1),
		}
		switch accountType {
		case AccountTypeAPIKey:
			account.Credentials = map[string]any{"api_key": "sk-test"}
		}
		return account
	}

	testCases := []struct {
		name        string
		accountType string
		statusCode  int
		body        string
		assertRepo  func(t *testing.T, repo *openAIPassthroughFailoverRepo, start time.Time)
	}{
		{
			name:        "apikey_429_rate_limit",
			accountType: AccountTypeAPIKey,
			statusCode:  http.StatusTooManyRequests,
			body: func() string {
				resetAt := time.Now().Add(7 * 24 * time.Hour).Unix()
				return fmt.Sprintf(`{"error":{"message":"The usage limit has been reached","type":"usage_limit_reached","resets_at":%d}}`, resetAt)
			}(),
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, _ time.Time) {
				require.Len(t, repo.rateLimitCalls, 1)
				require.Empty(t, repo.overloadCalls)
				require.True(t, time.Until(repo.rateLimitCalls[0]) > 24*time.Hour)
			},
		},
		{
			name:        "apikey_529_overload",
			accountType: AccountTypeAPIKey,
			statusCode:  529,
			body:        `{"error":{"message":"server overloaded","type":"server_error"}}`,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, start time.Time) {
				require.Empty(t, repo.rateLimitCalls)
				require.Len(t, repo.overloadCalls, 1)
				require.WithinDuration(t, start.Add(10*time.Minute), repo.overloadCalls[0], 5*time.Second)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

			resp := &http.Response{
				StatusCode: tc.statusCode,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"x-request-id": []string{"rid-failover"},
				},
				Body: io.NopCloser(strings.NewReader(tc.body)),
			}
			upstream := &httpUpstreamRecorder{resp: resp}
			repo := &openAIPassthroughFailoverRepo{}
			rateSvc := &RateLimitService{
				accountRepo: repo,
				cfg: &config.Config{
					RateLimit: config.RateLimitConfig{OverloadCooldownMinutes: 10},
				},
			}

			svc := &OpenAIGatewayService{
				cfg:              &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
				httpUpstream:     upstream,
				rateLimitService: rateSvc,
			}

			account := newAccount(tc.accountType)
			start := time.Now()
			_, err := svc.Forward(context.Background(), c, account, originalBody)
			require.Error(t, err)

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, tc.statusCode, failoverErr.StatusCode)
			require.False(t, c.Writer.Written(), "429/529 passthrough 应返回 failover 错误给上层换号，而不是直接向客户端写响应")

			v, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			arr, ok := v.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.NotEmpty(t, arr)
			require.True(t, arr[len(arr)-1].Passthrough)
			require.Equal(t, "failover", arr[len(arr)-1].Kind)
			require.Equal(t, tc.statusCode, arr[len(arr)-1].UpstreamStatusCode)

			tc.assertRepo(t, repo, start)
		})
	}
}

func TestOpenAIGatewayService_APIKeyPassthrough_PreservesBodyAndUsesResponsesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	c.Request.Header.Set("X-Test", "keep")

	originalBody := []byte(`{"model":"gpt-5.2","stream":false,"service_tier":"flex","max_output_tokens":128,"input":[{"type":"text","text":"hi"}]}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader(`{"output":[],"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             456,
		Name:           "apikey-acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Concurrency:    1,
		Credentials:    map[string]any{"api_key": "sk-api-key", "base_url": "https://api.openai.com"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ServiceTier)
	require.Equal(t, "flex", *result.ServiceTier)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, originalBody, upstream.lastBody)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "curl/8.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Test"))
}
