//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newMaskGateTestResp(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// TestNonStreamPassthrough_MaskingDisabled_NoRewrite 验证未开启响应遮蔽的账号，
// 其输出中的 AWS / 亚马逊 不被强制改写为 Anthropic（P0-2 语义纠正）。
func TestNonStreamPassthrough_MaskingDisabled_NoRewrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	body := `{"text":"powered by AWS and 亚马逊"}`
	account := &Account{ID: 1} // Extra 为空 → IsResponseMaskingEnabled()==false

	if _, err := svc.handleNonStreamingResponseAnthropicAPIKeyPassthrough(
		context.Background(), newMaskGateTestResp(body), c, account,
	); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	out := rec.Body.String()
	if !strings.Contains(out, "AWS") || !strings.Contains(out, "亚马逊") {
		t.Fatalf("masking-disabled account had body rewritten: %s", out)
	}
	if strings.Contains(out, "Anthropic") {
		t.Fatalf("masking-disabled account unexpectedly masked: %s", out)
	}
}

// TestNonStreamPassthrough_MaskingEnabled_Rewrites 验证开启响应遮蔽的账号，
// 输出中的 AWS / 亚马逊 正常遮蔽为 Anthropic。
func TestNonStreamPassthrough_MaskingEnabled_Rewrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	body := `{"text":"powered by AWS and 亚马逊"}`
	account := &Account{
		ID:    1,
		Extra: map[string]any{"response_masking": true},
	}

	if _, err := svc.handleNonStreamingResponseAnthropicAPIKeyPassthrough(
		context.Background(), newMaskGateTestResp(body), c, account,
	); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	out := rec.Body.String()
	if strings.Contains(out, "AWS") || strings.Contains(out, "亚马逊") {
		t.Fatalf("masking-enabled account not masked: %s", out)
	}
	if !strings.Contains(out, "Anthropic") {
		t.Fatalf("masking-enabled account missing replacement: %s", out)
	}
}
