//go:build e2e

package integration

// 账号级协议适配 —— Kiro 兼容（响应归一）E2E
//
// seed 一个 response_masking 账号（指向真实 openclaw kiro 上游），验证 KiroCompatAdapter
// 把 Kiro 响应归一为标准 Anthropic：
//   - tool_use 响应补缺失的 stop_reason
//   - usage 移除非标准 inference_geo 字段
//
// 需配 E2E_KIRO_UPSTREAM_KEY（script/e2e.env）；未配则 skip。

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func createKiroMaskingAccount(token, name, upstreamKey, baseURL string, groupID int64) (int64, error) {
	env, err := adminAPI(token, "POST", "/api/v1/admin/accounts", map[string]any{
		"name":     name,
		"platform": "anthropic",
		"type":     "apikey",
		"credentials": map[string]any{
			"api_key":  upstreamKey,
			"base_url": baseURL,
		},
		"group_ids":   []int64{groupID},
		"concurrency": 10,
		"extra":       map[string]any{"response_masking": true},
	})
	if err != nil {
		return 0, err
	}
	id, ok := dataObj(env)["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("建账号响应无 id：%v", env)
	}
	return int64(id), nil
}

func TestE2EFull_KiroCompatNormalize(t *testing.T) {
	pc := requireProvision(t)

	kiroKey := strings.TrimSpace(os.Getenv("E2E_KIRO_UPSTREAM_KEY"))
	if kiroKey == "" {
		t.Skip("未配置 E2E_KIRO_UPSTREAM_KEY，跳过")
	}
	kiroBase := getEnv("E2E_KIRO_UPSTREAM_BASE_URL", "https://openclaw.zhiguo.fan")
	kiroModel := getEnv("E2E_KIRO_MODEL", "claude-sonnet-4-6")

	suffix := runNonce()
	gid, err := createGroup(pc.adminToken, "e2e-kiro-"+suffix, "anthropic")
	if err != nil {
		t.Fatalf("建分组失败：%v", err)
	}
	if _, err := createKiroMaskingAccount(pc.adminToken, "e2e-kiro-acct-"+suffix, kiroKey, kiroBase, gid); err != nil {
		t.Fatalf("建 kiro 账号失败：%v", err)
	}
	gwKey, _, err := createGatewayKey(pc.adminToken, "e2e-kiro-key-"+suffix, gid, nil, nil)
	if err != nil {
		t.Fatalf("建网关 key 失败：%v", err)
	}

	t.Run("tool_use 补 stop_reason", func(t *testing.T) {
		payload := map[string]any{
			"model":       kiroModel,
			"max_tokens":  256,
			"tool_choice": map[string]any{"type": "any"},
			"tools": []map[string]any{{
				"name":        "get_weather",
				"description": "Get the weather for a city",
				"input_schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"city": map[string]any{"type": "string"}},
					"required":   []string{"city"},
				},
			}},
			"messages": []map[string]any{{"role": "user", "content": "What is the weather in Paris?"}},
		}
		body, _ := json.Marshal(payload)
		st, resp, err := apiCall("POST", "/v1/messages", map[string]string{
			"x-api-key": gwKey, "anthropic-version": "2023-06-01",
		}, body)
		if err != nil {
			t.Fatalf("请求错误：%v", err)
		}
		if st != 200 {
			t.Fatalf("期望 200，实际 %d：%s", st, truncate(resp, 400))
		}
		// Kiro tool_use 响应原始缺 stop_reason，归一应补齐。
		if !bodyContains(resp, `"stop_reason"`) {
			t.Fatalf("归一后 tool_use 响应应含 stop_reason：%s", truncate(resp, 400))
		}
		if !bodyContains(resp, `"type":"tool_use"`) {
			t.Logf("注意：响应未含 tool_use 块（模型可能直接答复）：%s", truncate(resp, 200))
		}
		t.Logf("✅ Kiro tool_use 归一 OK（已补 stop_reason）")
	})

	t.Run("text 移除 inference_geo", func(t *testing.T) {
		st, resp, err := gwClaudeMessages(gwKey, kiroModel, "Reply with one short sentence.", false, 64)
		if err != nil {
			t.Fatalf("请求错误：%v", err)
		}
		if st != 200 {
			t.Fatalf("期望 200，实际 %d：%s", st, truncate(resp, 400))
		}
		// Kiro 原始 usage 带非标准 inference_geo，归一应移除。
		if bodyContains(resp, "inference_geo") {
			t.Fatalf("归一后应移除 usage.inference_geo：%s", truncate(resp, 400))
		}
		t.Logf("✅ Kiro text 归一 OK（已移除 inference_geo）")
	})
}
