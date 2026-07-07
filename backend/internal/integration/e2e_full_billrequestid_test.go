//go:build e2e

package integration

// Bill-Request-ID 下游对账标识（fork 功能 27）E2E 断言。
//
// 网关 client_request_id 中间件对每个网关请求：
//   - 恒回写 X-Client-Request-ID（本次请求的 UUID）；
//   - 读取下游上传的 Bill-Request-ID：合法（非空且 ≤64 字符）则原样回写，
//     否则回退为 X-Client-Request-ID，保证下游总能用一个稳定 ID 反查账单。
//
// 与 e2e_full_provision_test.go 的 provisioning 共享上下文。

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// apiCallH 同 apiCall，但额外返回响应头，用于断言网关注入的响应头。
func apiCallH(method, path string, headers map[string]string, body []byte) (int, http.Header, []byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, baseURL+path, r)
	if err != nil {
		return 0, nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, rb, nil
}

func TestE2EFull_BillRequestIDWriteback(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	// claudeReq 打一次 /v1/messages（带/不带 Bill-Request-ID），返回响应头。
	// 中间件在鉴权前运行，响应头随任何结果一并写回，故不强依赖上游 200。
	claudeReq := func(billID string) (http.Header, int, []byte) {
		payload := map[string]any{
			"model":      pp.model,
			"max_tokens": 16,
			"messages":   []map[string]any{{"role": "user", "content": "hi"}},
		}
		body, _ := json.Marshal(payload)
		h := map[string]string{
			"x-api-key":         pp.gatewayKey,
			"anthropic-version": "2023-06-01",
		}
		if billID != "" {
			h[billRequestIDHeaderName] = billID
		}
		st, respH, rb, err := apiCallH("POST", "/v1/messages", h, body)
		if err != nil {
			t.Fatalf("请求错误: %v", err)
		}
		return respH, st, rb
	}

	t.Run("下游 Bill-Request-ID 原样回写", func(t *testing.T) {
		billID := "e2e-bill-" + runNonce()
		respH, st, rb := claudeReq(billID)
		gotBill := respH.Get(billRequestIDHeaderName)
		cli := respH.Get(clientRequestIDHeaderName)
		if gotBill != billID {
			t.Fatalf("Bill-Request-ID 未原样回写：期望 %q 实际 %q (st=%d, body=%s)", billID, gotBill, st, truncate(rb, 200))
		}
		if strings.TrimSpace(cli) == "" {
			t.Fatalf("缺少 X-Client-Request-ID (st=%d)", st)
		}
		if cli == billID {
			t.Fatalf("X-Client-Request-ID 应为独立 UUID，不应等于下游 Bill-Request-ID")
		}
		t.Logf("✅ 原样回写：Bill-Request-ID=%s，独立 X-Client-Request-ID=%s (st=%d)", gotBill, cli, st)
	})

	t.Run("缺失 Bill-Request-ID 回退为 X-Client-Request-ID", func(t *testing.T) {
		respH, st, _ := claudeReq("")
		gotBill := respH.Get(billRequestIDHeaderName)
		cli := respH.Get(clientRequestIDHeaderName)
		if strings.TrimSpace(cli) == "" {
			t.Fatalf("缺少 X-Client-Request-ID (st=%d)", st)
		}
		if gotBill != cli {
			t.Fatalf("缺失时 Bill-Request-ID 应回退为 X-Client-Request-ID：bill=%q client=%q (st=%d)", gotBill, cli, st)
		}
		t.Logf("✅ 缺失回退：Bill-Request-ID == X-Client-Request-ID = %s (st=%d)", gotBill, st)
	})

	t.Run("超 64 字符 Bill-Request-ID 回退", func(t *testing.T) {
		oversized := strings.Repeat("x", 65)
		respH, st, _ := claudeReq(oversized)
		gotBill := respH.Get(billRequestIDHeaderName)
		cli := respH.Get(clientRequestIDHeaderName)
		if strings.TrimSpace(cli) == "" {
			t.Fatalf("缺少 X-Client-Request-ID (st=%d)", st)
		}
		if gotBill == oversized {
			t.Fatalf("超 64 字符 Bill-Request-ID 不应原样回写 (st=%d)", st)
		}
		if gotBill != cli {
			t.Fatalf("超长时应回退为 X-Client-Request-ID：bill=%q client=%q (st=%d)", gotBill, cli, st)
		}
		t.Logf("✅ 超长回退：Bill-Request-ID == X-Client-Request-ID = %s (st=%d)", gotBill, st)
	})
}

const (
	clientRequestIDHeaderName = "X-Client-Request-ID"
	billRequestIDHeaderName   = "Bill-Request-ID"
)
