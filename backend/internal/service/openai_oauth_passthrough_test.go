package service

import (
	"bytes"
	"context"
	"errors"
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
	"github.com/tidwall/gjson"
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

type passthroughCloseTrackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *passthroughCloseTrackingReadCloser) Close() error {
	r.closed = true
	return nil
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

func TestOpenAIGatewayService_APIKeyPassthrough_RebuildsUpstreamErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		statusCode     int
		contentType    string
		responseBody   string
		retryAfter     string
		wantStatus     int
		wantMessage    string
		wantRetryAfter string
	}{
		{
			name:           "upstream forbidden is reported as gateway failure",
			statusCode:     http.StatusForbidden,
			contentType:    "text/html; charset=UTF-8",
			responseBody:   `<!DOCTYPE html><title>secret-upstream.example denied the request</title>`,
			retryAfter:     "17",
			wantStatus:     http.StatusBadGateway,
			wantMessage:    "Upstream access denied",
			wantRetryAfter: "17",
		},
		{
			name:         "upstream unauthorized is reported as gateway failure",
			statusCode:   http.StatusUnauthorized,
			contentType:  "application/json",
			responseBody: `{"error":{"message":"invalid secret-upstream.example token","type":"authentication_error","code":"invalid_api_key","param":"api_key"},"rate_limit":{"remaining":0}}`,
			wantStatus:   http.StatusBadGateway,
			wantMessage:  "Upstream authentication failed",
		},
		// 瞬时 5xx（500/502/503/504/520-524）对 API-key 账号已改走多账号
		// failover（见 APIKeyPassthrough_Transient5xxTriggersFailover），此处
		// 改用非瞬时 5xx 状态码，继续覆盖净化重建路径。
		{
			name:         "html 5xx",
			statusCode:   530,
			contentType:  "text/html; charset=UTF-8",
			responseBody: `<!DOCTYPE html><title>secret-upstream.example | 530: Origin DNS error</title>`,
			wantStatus:   530,
			wantMessage:  "Upstream service temporarily unavailable",
		},
		{
			name:         "structured 5xx",
			statusCode:   http.StatusNotImplemented,
			contentType:  "application/json",
			responseBody: `{"error":{"message":"secret-upstream.example internal failure"}}`,
			wantStatus:   http.StatusNotImplemented,
			wantMessage:  "Upstream service temporarily unavailable",
		},
		{
			name:         "unstructured 4xx",
			statusCode:   http.StatusBadRequest,
			contentType:  "text/plain",
			responseBody: `proxy secret-upstream.example rejected the request`,
			wantStatus:   http.StatusBadRequest,
			wantMessage:  "Upstream request failed",
		},
		{
			name:         "malicious valid json 4xx",
			statusCode:   http.StatusBadRequest,
			contentType:  "application/json",
			responseBody: `{"error":{"message":"secret-upstream.example invalid parameter","type":"invalid_request_error","code":"upstream_secret_code","param":"private_field","internal_token":"sk-upstream-secret"},"rate_limit":{"remaining":0,"reset":"internal-window"},"debug":{"admin":"root"},"redirect":"https://secret-upstream.example/admin"}`,
			retryAfter:   "not-a-valid-delay",
			wantStatus:   http.StatusBadRequest,
			wantMessage:  "Upstream request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: tt.statusCode,
				Header: http.Header{
					"Content-Type":                 []string{tt.contentType},
					"Location":                     []string{"https://secret-upstream.example/admin"},
					"Retry-After":                  []string{tt.retryAfter},
					"Server":                       []string{"secret-upstream-proxy"},
					"Set-Cookie":                   []string{"admin_token=secret"},
					"WWW-Authenticate":             []string{`Bearer realm="secret-upstream.example"`},
					"X-Admin-Debug":                []string{"internal-route=secret-upstream.example"},
					"X-Codex-Primary-Used-Percent": []string{"99"},
					"x-request-id":                 []string{"rid-sensitive-upstream"},
				},
				Body: io.NopCloser(strings.NewReader(tt.responseBody)),
			}}
			svc := &OpenAIGatewayService{
				cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
				httpUpstream: upstream,
			}
			account := &Account{
				ID:          124,
				Name:        "sensitive-upstream",
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":  "sk-test",
					"base_url": "https://secret-upstream.example",
				},
				Extra:       map[string]any{"openai_passthrough": true},
				Status:      StatusActive,
				Schedulable: true,
			}
			requestBody := []byte(`{"model":"gpt-5.2","stream":false,"input":"hello"}`)

			_, err := svc.Forward(context.Background(), c, account, requestBody)

			require.Error(t, err)
			require.Equal(t, tt.wantStatus, rec.Code)
			opsValue, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			opsEvents, ok := opsValue.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.NotEmpty(t, opsEvents)
			require.Equal(t, tt.statusCode, opsEvents[len(opsEvents)-1].UpstreamStatusCode)
			require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
			require.Equal(t, tt.wantRetryAfter, rec.Header().Get("Retry-After"))
			for _, key := range []string{
				"Location",
				"Server",
				"Set-Cookie",
				"WWW-Authenticate",
				"X-Admin-Debug",
				"X-Codex-Primary-Used-Percent",
				"X-Request-Id",
			} {
				require.Empty(t, rec.Header().Values(key), "sensitive upstream header %s must be dropped", key)
			}
			require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
			require.Equal(t, tt.wantMessage, gjson.Get(rec.Body.String(), "error.message").String())
			require.False(t, gjson.Get(rec.Body.String(), "error.code").Exists())
			require.False(t, gjson.Get(rec.Body.String(), "error.param").Exists())
			require.False(t, gjson.Get(rec.Body.String(), "rate_limit").Exists())
			require.NotContains(t, rec.Body.String(), "secret-upstream.example")
			require.NotContains(t, rec.Body.String(), "sk-upstream-secret")
			require.NotContains(t, err.Error(), "secret-upstream.example")
		})
	}
}

func TestWriteOpenAIPassthroughErrorHeaders_StrictRetryAfter(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "positive delay seconds", raw: "17", want: true},
		{name: "fractional delay", raw: "1.5"},
		{name: "scientific notation", raw: "1e3"},
		{name: "explicit plus sign", raw: "+17"},
		{name: "zero", raw: "0"},
		{name: "negative delay", raw: "-1"},
		{name: "uint64 overflow", raw: "18446744073709551616"},
		{name: "future http date", raw: now.Add(time.Hour).Format(http.TimeFormat), want: true},
		{name: "past http date", raw: now.Add(-time.Hour).Format(http.TimeFormat)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := http.Header{"Retry-After": []string{"stale"}}
			writeOpenAIPassthroughErrorHeaders(dst, http.Header{"Retry-After": []string{tt.raw}})
			if tt.want {
				require.Equal(t, tt.raw, dst.Get("Retry-After"))
			} else {
				require.Empty(t, dst.Get("Retry-After"))
			}
		})
	}
}

func TestOpenAIGatewayService_APIKeyPassthrough_CompactErrorBeforeKeepaliveIsSingleJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(nil))
	MarkOpenAICompactClientStream(c)
	stop := StartOpenAICompactSSEKeepalive(c, time.Hour)
	defer stop()

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"secret-upstream.example invalid request"}}`)),
		}},
	}
	account := &Account{
		ID: 125, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://secret-upstream.example"},
		Extra:       map[string]any{"openai_passthrough": true}, Status: StatusActive, Schedulable: true,
	}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.2","input":"hello"}`))

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.True(t, gjson.Valid(rec.Body.String()))
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.NotContains(t, rec.Body.String(), "event:")
	require.NotContains(t, rec.Body.String(), ": keepalive")
	require.NotContains(t, rec.Body.String(), "secret-upstream.example")
}

func TestOpenAIGatewayService_APIKeyPassthrough_CompactErrorAfterKeepaliveIsFailedSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(nil))
	MarkOpenAICompactClientStream(c)
	stop := StartOpenAICompactSSEKeepalive(c, keepaliveTestInterval)
	defer stop()
	waitForKeepaliveBeats()

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"secret-upstream.example invalid request"}}`)),
		}},
	}
	account := &Account{
		ID: 126, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://secret-upstream.example"},
		Extra:       map[string]any{"openai_passthrough": true}, Status: StatusActive, Schedulable: true,
	}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.2","input":"hello"}`))

	require.Error(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Result().Header.Get("Content-Type"), "text/event-stream")
	events := parseCompactBridgeSSE(t, stripKeepaliveComments(rec.Body.String()))
	require.Len(t, events, 1)
	require.Equal(t, "response.failed", events[0][0])
	require.Equal(t, "failed", gjson.Get(events[0][1], "response.status").String())
	require.Equal(t, "upstream_error", gjson.Get(events[0][1], "response.error.code").String())
	require.Equal(t, "Upstream request failed", gjson.Get(events[0][1], "response.error.message").String())
	require.NotContains(t, rec.Body.String(), "secret-upstream.example")
}

func TestOpenAIGatewayService_OpenAIPassthrough_RetryableStatusesTriggerFailover(t *testing.T) {
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
		name           string
		accountType    string
		statusCode     int
		body           string
		expectFailover bool
		assertRepo     func(t *testing.T, repo *openAIPassthroughFailoverRepo, start time.Time)
	}{
		{
			name:        "apikey_429_rate_limit",
			accountType: AccountTypeAPIKey,
			statusCode:  http.StatusTooManyRequests,
			body: func() string {
				resetAt := time.Now().Add(7 * 24 * time.Hour).Unix()
				return fmt.Sprintf(`{"error":{"message":"The usage limit has been reached","type":"usage_limit_reached","resets_at":%d}}`, resetAt)
			}(),
			expectFailover: true,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, _ time.Time) {
				require.Len(t, repo.rateLimitCalls, 1)
				require.Empty(t, repo.overloadCalls)
				require.True(t, time.Until(repo.rateLimitCalls[0]) > 24*time.Hour)
			},
		},
		{
			name:           "apikey_529_overload",
			accountType:    AccountTypeAPIKey,
			statusCode:     529,
			body:           `{"error":{"message":"server overloaded","type":"server_error"}}`,
			expectFailover: true,
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
			if tc.expectFailover {
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, tc.statusCode, failoverErr.StatusCode)
				require.False(t, c.Writer.Written(), "retryable passthrough 错误应返回 failover 错误给上层换号，而不是直接向客户端写响应")
			} else {
				require.False(t, errors.As(err, &failoverErr))
				require.True(t, c.Writer.Written(), "非 failover 的 passthrough http 错误应直接写回客户端")
				require.Equal(t, tc.statusCode, rec.Code)
			}

			v, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			arr, ok := v.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.NotEmpty(t, arr)
			require.True(t, arr[len(arr)-1].Passthrough)
			if tc.expectFailover {
				require.Equal(t, "failover", arr[len(arr)-1].Kind)
			} else {
				require.Equal(t, "http_error", arr[len(arr)-1].Kind)
			}
			require.Equal(t, tc.statusCode, arr[len(arr)-1].UpstreamStatusCode)

			tc.assertRepo(t, repo, start)
		})
	}
}

func TestOpenAIGatewayService_APIKeyPassthrough_Transient5xxTriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestBody := []byte(`{"model":"gpt-5.2","stream":false,"input":"hello"}`)

	for _, statusCode := range []int{
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		520, 521, 522, 523, 524,
	} {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

			upstreamBody := fmt.Sprintf(`{"error":{"message":"temporary upstream failure","status":%d}}`, statusCode)
			body := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader(upstreamBody)}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: statusCode,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"rid-api-key-5xx"},
				},
				Body: body,
			}}
			svc := &OpenAIGatewayService{
				cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
				httpUpstream: upstream,
			}
			account := &Account{
				ID:          124,
				Name:        "api-key-transient-5xx",
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":  "sk-test",
					"base_url": "https://api.example.test",
				},
				Extra:       map[string]any{"openai_passthrough": true},
				Status:      StatusActive,
				Schedulable: true,
			}

			result, err := svc.Forward(context.Background(), c, account, requestBody)

			require.Nil(t, result, "failed attempts must not report usage or success metadata")
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, statusCode, failoverErr.StatusCode)
			require.JSONEq(t, upstreamBody, string(failoverErr.ResponseBody))
			require.Equal(t, "rid-api-key-5xx", failoverErr.ResponseHeaders.Get("x-request-id"))
			require.False(t, c.Writer.Written(), "failover must happen before downstream output is committed")
			require.True(t, body.closed, "the failed upstream response body must be closed")
			require.Equal(t, requestBody, upstream.lastBody, "the request body remains available for the outer account retry")

			value, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			events, ok := value.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.NotEmpty(t, events)
			require.Equal(t, "failover", events[len(events)-1].Kind)
			require.Equal(t, account.ID, events[len(events)-1].AccountID)
		})
	}
}

func TestOpenAIGatewayService_APIKeyPassthrough_ContextWindow502DoesNotFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))

	const upstreamBody = `{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"upstream_error"}}`
	body := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader(upstreamBody)}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       body,
		}},
	}
	account := &Account{
		ID: 127, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.example.test"},
		Extra:       map[string]any{"openai_passthrough": true}, Status: StatusActive, Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.2","input":"hello"}`))

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "context-window errors are deterministic request failures")
	require.True(t, c.Writer.Written())
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "exceeds the context window")
	require.True(t, body.closed)
}

func TestOpenAIGatewayService_APIKeyPassthrough_PoolModeConfigured5xxRetriesSameAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"temporary upstream failure"}}`)),
		}},
	}
	account := &Account{
		ID: 128, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                      "sk-test",
			"base_url":                     "https://api.example.test",
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusBadGateway)},
		},
		Extra: map[string]any{"openai_passthrough": true}, Status: StatusActive, Schedulable: true,
	}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.2","input":"hello"}`))

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written())
}

func TestOpenAIGatewayService_APIKeyPassthrough_PreservesBodyAndUsesResponsesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	c.Request.Header.Set("X-Test", "keep")
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")
	c.Request.Header.Set("X-Codex-Window-ID", "window-passthrough")
	c.Request.Header.Set("X-Codex-Installation-ID", "installation-passthrough")

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
	require.Equal(t, "remote_compaction_v2", upstream.lastReq.Header.Get("x-codex-beta-features"))
	require.Equal(t, "window-passthrough", upstream.lastReq.Header.Get("X-Codex-Window-ID"))
	require.Equal(t, "installation-passthrough", upstream.lastReq.Header.Get("X-Codex-Installation-ID"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Test"))
}
