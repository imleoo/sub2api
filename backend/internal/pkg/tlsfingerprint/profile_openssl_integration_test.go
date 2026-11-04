//go:build integration

package tlsfingerprint

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestOpenSSLNativeProfileLiveHandshake 用校准后的 codex 指纹对真实上游
// （chatgpt.com，Cloudflare 终结 TLS）完成完整握手并请求 /cdn-cgi/trace，
// 验证 X25519MLKEM768 keyshare、ML-DSA 签名算法列表等在真实服务器上可用。
// 运行方式：go test -tags=integration -run TestOpenSSLNativeProfileLiveHandshake ./internal/pkg/tlsfingerprint/
func TestOpenSSLNativeProfileLiveHandshake(t *testing.T) {
	dialer := NewDialer(OpenSSLNativeProfile(), nil)
	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: dialer.DialTLSContext,
		},
		Timeout: 15 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://chatgpt.com/cdn-cgi/trace", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("live handshake with chatgpt.com failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var bodyHead string
	if len(body) > 0 {
		bodyHead = strings.TrimSpace(string(body[:min(len(body), 80)]))
	}
	t.Logf("status=%d tls_handshake_ok body_head=%q", resp.StatusCode, bodyHead)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if cs := resp.TLS; cs != nil {
		t.Logf("negotiated: version=0x%04x cipher=0x%04x alpn=%q", cs.Version, cs.CipherSuite, cs.NegotiatedProtocol)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
