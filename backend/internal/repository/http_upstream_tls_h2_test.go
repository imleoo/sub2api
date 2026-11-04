package repository

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"golang.org/x/net/http2"
)

func newTLSProtocolModeTestService(t *testing.T) *httpUpstreamService {
	t.Helper()
	svc, ok := NewHTTPUpstream(&config.Config{}).(*httpUpstreamService)
	if !ok {
		t.Fatal("expected *httpUpstreamService")
	}
	svc.cfg.Gateway.OpenAIHTTP2 = config.GatewayOpenAIHTTP2Config{
		Enabled:                   true,
		AllowProxyFallbackToHTTP1: true,
	}
	return svc
}

func TestResolveTLSProtocolMode(t *testing.T) {
	svc := newTLSProtocolModeTestService(t)
	// 默认 OpenSSL 指纹无 ALPN → 不走 H2
	defaultProfile := tlsfingerprint.OpenSSLNativeProfile()
	// 带 h2 ALPN 的模板 profile（DB 模板场景）→ 可走 TLS+H2
	h2Profile := &tlsfingerprint.Profile{
		Name:          "h2-template",
		ALPNProtocols: []string{"h2", "http/1.1"},
	}
	h1Profile := tlsfingerprint.ProfileForHTTP1Only(h2Profile)

	// OpenAI 上游 + h2 ALPN + H2 启用 → TLS+H2
	if got := svc.resolveTLSProtocolMode(h2Profile, service.HTTPUpstreamProfileOpenAI, "direct", nil); got != upstreamProtocolModeOpenAIH2TLS {
		t.Fatalf("expected openai_h2_tls, got %s", got)
	}
	// 默认 OpenSSL 指纹（无 ALPN）→ default（TLS+H1，与真实 codex 一致）
	if got := svc.resolveTLSProtocolMode(defaultProfile, service.HTTPUpstreamProfileOpenAI, "direct", nil); got != upstreamProtocolModeDefault {
		t.Fatalf("expected default for no-alpn profile, got %s", got)
	}
	// H1-only profile → default（TLS+H1）
	if got := svc.resolveTLSProtocolMode(h1Profile, service.HTTPUpstreamProfileOpenAI, "direct", nil); got != upstreamProtocolModeDefault {
		t.Fatalf("expected default for h1-only profile, got %s", got)
	}
	// 非 OpenAI 上游 → default
	if got := svc.resolveTLSProtocolMode(h2Profile, service.HTTPUpstreamProfileDefault, "direct", nil); got != upstreamProtocolModeDefault {
		t.Fatalf("expected default for non-openai profile, got %s", got)
	}
	// H2 全局关闭 → default
	svc.cfg.Gateway.OpenAIHTTP2.Enabled = false
	if got := svc.resolveTLSProtocolMode(h2Profile, service.HTTPUpstreamProfileOpenAI, "direct", nil); got != upstreamProtocolModeDefault {
		t.Fatalf("expected default when h2 disabled, got %s", got)
	}
	svc.cfg.Gateway.OpenAIHTTP2.Enabled = true

	// HTTP 代理 H2 兼容熔断激活 → default（TLS+H1）
	proxyURL, _ := url.Parse("http://proxy-a:8080")
	_ = proxyURL
	proxyKey := "http://proxy-a:8080"
	state := svc.getOrCreateOpenAIHTTP2FallbackState(proxyKey)
	state.recordFailure(time.Now(), 1, time.Minute, time.Hour)
	if !svc.isOpenAIHTTP2FallbackActive(proxyKey) {
		t.Fatal("failed to prime fallback state")
	}
	if got := svc.resolveTLSProtocolMode(h2Profile, service.HTTPUpstreamProfileOpenAI, proxyKey, proxyURL); got != upstreamProtocolModeDefault {
		t.Fatalf("expected default when fallback active, got %s", got)
	}
}

func TestBuildUpstreamTransportWithTLSFingerprintH2(t *testing.T) {
	// h2 分支仅对配置了 h2 ALPN 的模板 profile 生效
	h2Profile := &tlsfingerprint.Profile{
		Name:          "h2-template",
		ALPNProtocols: []string{"h2", "http/1.1"},
	}
	settings := poolSettings{
		maxIdleConns:        10,
		maxIdleConnsPerHost: 10,
		maxConnsPerHost:     10,
	}

	// openai_h2_tls → *http2.Transport（独立 DialTLSContext，绕开 net/http 的
	// *tls.Conn 硬断言），keepalive 对齐 openai_h2
	rt, err := buildUpstreamTransportWithTLSFingerprint(settings, nil, h2Profile, upstreamProtocolModeOpenAIH2TLS)
	if err != nil {
		t.Fatal(err)
	}
	h2t, ok := rt.(*http2.Transport)
	if !ok {
		t.Fatalf("expected *http2.Transport, got %T", rt)
	}
	if h2t.ReadIdleTimeout != openAIHTTP2ReadIdleTimeout || h2t.PingTimeout != openAIHTTP2PingTimeout {
		t.Fatal("http2 keepalive settings mismatch")
	}
	if h2t.DialTLSContext == nil {
		t.Fatal("DialTLSContext must be set")
	}

	// default 模式 → *http.Transport 且 ForceAttemptHTTP2=false（ALPN 覆写为 h1-only，
	// 避免协商出 h2 却跑 HTTP/1.1）
	rt, err = buildUpstreamTransportWithTLSFingerprint(settings, nil, h2Profile, upstreamProtocolModeDefault)
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", rt)
	}
	if transport.ForceAttemptHTTP2 {
		t.Fatal("TLS-H1 transport must not attempt HTTP/2")
	}
	if transport.DialTLSContext == nil {
		t.Fatal("DialTLSContext must be set")
	}
}

func TestTLSClientCacheKeySeparatesProtocolModes(t *testing.T) {
	svc := newTLSProtocolModeTestService(t)
	h2Profile := &tlsfingerprint.Profile{
		Name:          "h2-template",
		ALPNProtocols: []string{"h2", "http/1.1"},
	}
	h1Profile := tlsfingerprint.ProfileForHTTP1Only(h2Profile)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://chatgpt.com/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(service.WithHTTPUpstreamProfile(req.Context(), service.HTTPUpstreamProfileOpenAI))

	entryH2, err := svc.getClientEntryWithTLS("", 7, 4, h2Profile, service.HTTPUpstreamProfileOpenAI, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if entryH2.protocolMode != upstreamProtocolModeOpenAIH2TLS {
		t.Fatalf("expected openai_h2_tls entry, got %s", entryH2.protocolMode)
	}
	entryH1, err := svc.getClientEntryWithTLS("", 7, 4, h1Profile, service.HTTPUpstreamProfileOpenAI, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if entryH1.protocolMode != upstreamProtocolModeDefault {
		t.Fatalf("expected default entry, got %s", entryH1.protocolMode)
	}
	if entryH2 == entryH1 {
		t.Fatal("TLS-H2 and TLS-H1 clients must not share a cache entry")
	}
	svc.mu.RLock()
	keys := make([]string, 0, len(svc.clients))
	for k := range svc.clients {
		keys = append(keys, k)
	}
	svc.mu.RUnlock()
	if len(keys) != 2 {
		t.Fatalf("expected 2 distinct cache entries, got %v", keys)
	}
}
