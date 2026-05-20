package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

const baseURL = "https://openclaw.zhiguo.fan"

var token string

func doRequest(t *testing.T, method, path string, body map[string]interface{}) map[string]interface{} {
	var bodyReader io.Reader
	if body != nil {
		bs, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bs)
	}

	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("Failed to create request %s: %v", path, err)
	}

	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request %s: %v", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("Failed to unmarshal response for %s: %v\nBody: %s", path, err, string(respBody))
	}

	return result
}

func reportResult(t *testing.T, apiName, path string, result map[string]interface{}) {
	code, ok := result["code"].(float64)
	if !ok || code != 0 {
		t.Errorf("❌ [%s] %s failed. Code: %v, Message: %v", apiName, path, result["code"], result["message"])
		fmt.Printf("❌ [%s] %s - Failed (Code: %v, Msg: %v)\n", apiName, path, result["code"], result["message"])
	} else {
		fmt.Printf("✅ [%s] %s - Success\n", apiName, path)
	}
	prettyJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("📦 Response Details:\n%s\n\n", string(prettyJSON))
}

func TestReconciliationAPIs(t *testing.T) {
	fmt.Println("=== 开始执行对账 API 测试及报告 ===")

	// 1. Login
	loginBody := map[string]interface{}{
		"email":    "jp-test1@b517.com",
		"password": "jp20260408",
	}
	loginRes := doRequest(t, "POST", "/api/v1/auth/login", loginBody)
	reportResult(t, "认证：登录获取 Token", "/api/v1/auth/login", loginRes)
	
	if data, ok := loginRes["data"].(map[string]interface{}); ok {
		if tToken, ok := data["access_token"].(string); ok {
			token = tToken
		}
	}
	if token == "" {
		t.Fatalf("Failed to get token, aborting tests")
	}

	// 2. Refresh Token
	var refreshToken string
	if data, ok := loginRes["data"].(map[string]interface{}); ok {
		if rToken, ok := data["refresh_token"].(string); ok {
			refreshToken = rToken
		}
	}
	if refreshToken != "" {
		refreshBody := map[string]interface{}{
			"refresh_token": refreshToken,
		}
		refreshRes := doRequest(t, "POST", "/api/v1/auth/refresh", refreshBody)
		reportResult(t, "认证：刷新 Token", "/api/v1/auth/refresh", refreshRes)
	}

	// 3. Balance
	meRes := doRequest(t, "GET", "/api/v1/auth/me", nil)
	reportResult(t, "账户信息：余额查询", "/api/v1/auth/me", meRes)

	// 4. Orders
	ordersRes := doRequest(t, "GET", "/api/v1/payment/orders/my?page=1&page_size=10", nil)
	reportResult(t, "支付订单：查询我的订单", "/api/v1/payment/orders/my", ordersRes)
	
	// 5. Order Verify
	verifyBody := map[string]interface{}{
		"out_trade_no": "TOKENPANEL_20260420_xxxxxxxxxxxxxxxx",
	}
	verifyRes := doRequest(t, "POST", "/api/v1/payment/orders/verify", verifyBody)
	if code, _ := verifyRes["code"].(float64); code != 0 {
		fmt.Printf("⚠️  [支付订单：主动核验（补单）] /api/v1/payment/orders/verify - Expected failure with dummy trade no (Code: %v, Msg: %v)\n", verifyRes["code"], verifyRes["message"])
		prettyJSON, _ := json.MarshalIndent(verifyRes, "", "  ")
		fmt.Printf("📦 Response Details:\n%s\n\n", string(prettyJSON))
	} else {
		reportResult(t, "支付订单：主动核验（补单）", "/api/v1/payment/orders/verify", verifyRes)
	}

	// 6. Usage details
	usageRes := doRequest(t, "GET", "/api/v1/usage?page=1&page_size=3&timezone=Asia/Shanghai", nil)
	reportResult(t, "用量明细：查询用量记录", "/api/v1/usage", usageRes)

	// 7. Usage stats
	statsRes := doRequest(t, "GET", "/api/v1/usage/stats?period=month&timezone=Asia/Shanghai", nil)
	reportResult(t, "用量统计：汇总数据", "/api/v1/usage/stats", statsRes)

	// 8. Usage dashboard stats
	dashStatsRes := doRequest(t, "GET", "/api/v1/usage/dashboard/stats", nil)
	reportResult(t, "用量统计：仪表盘总览", "/api/v1/usage/dashboard/stats", dashStatsRes)

	// 9. Usage dashboard trend
	trendRes := doRequest(t, "GET", "/api/v1/usage/dashboard/trend?granularity=day&timezone=Asia/Shanghai", nil)
	reportResult(t, "用量统计：按天趋势", "/api/v1/usage/dashboard/trend", trendRes)

	// 10. Usage dashboard models
	modelsRes := doRequest(t, "GET", "/api/v1/usage/dashboard/models?timezone=Asia/Shanghai", nil)
	reportResult(t, "用量统计：按模型分组", "/api/v1/usage/dashboard/models", modelsRes)

	fmt.Println("=== 对账 API 测试报告结束 ===")
}
