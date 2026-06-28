//go:build e2e

package integration

// 账号级协议适配 —— Kiro vision/document 跨组兜底 E2E
//
// 验证：Kiro 组配置 FallbackGroupIDOnInvalidRequest 后，请求用到 Kiro 静默忽略的能力
// （image/document）时被 reroute 到兜底组处理。强证明用对比：
//   - 文本请求 → 由 Kiro 处理（200，不 reroute）
//   - vision 请求 + 兜底→官方组 → reroute 成功处理（200）
//   - vision 请求 + 兜底→空组（无账号）→ 非 200（强证明请求确实离开 Kiro，而非被静默处理）
//
// 需配 E2E_KIRO_UPSTREAM_KEY + anthropic 上游（官方兜底）。

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

const e2eVisionPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// createGroupWithInvalidFallback 建一个配置了 fallback_group_id_on_invalid_request 的分组。
func createGroupWithInvalidFallback(token, name, platform string, fallbackGID int64) (int64, error) {
	env, err := adminAPI(token, "POST", "/api/v1/admin/groups", map[string]any{
		"name":                                 name,
		"platform":                             platform,
		"subscription_type":                    "standard",
		"rate_multiplier":                      1,
		"fallback_group_id_on_invalid_request": fallbackGID,
	})
	if err != nil {
		return 0, err
	}
	id, ok := dataObj(env)["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("建分组响应无 id：%v", env)
	}
	return int64(id), nil
}

// gwVisionMessage 发一个含 image 块的非流式请求。
func gwVisionMessage(gwKey, model string) (int, []byte, error) {
	payload := map[string]any{
		"model":      model,
		"max_tokens": 64,
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "image", "source": map[string]any{"type": "base64", "media_type": "image/png", "data": e2eVisionPNG}},
				{"type": "text", "text": "What is in this image?"},
			},
		}},
	}
	body, _ := json.Marshal(payload)
	return apiCall("POST", "/v1/messages", map[string]string{
		"x-api-key": gwKey, "anthropic-version": "2023-06-01",
	}, body)
}

func TestE2EFull_KiroVisionReroute(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	kiroKey := strings.TrimSpace(os.Getenv("E2E_KIRO_UPSTREAM_KEY"))
	if kiroKey == "" {
		t.Skip("未配置 E2E_KIRO_UPSTREAM_KEY，跳过")
	}
	kiroBase := getEnv("E2E_KIRO_UPSTREAM_BASE_URL", "https://openclaw.zhiguo.fan")
	kiroModel := getEnv("E2E_KIRO_MODEL", "claude-sonnet-4-6")
	suffix := runNonce()

	// 官方兜底组 + 官方账号（能真正处理 vision）
	officialGID, err := createGroup(pc.adminToken, "e2e-official-"+suffix, "anthropic")
	if err != nil {
		t.Fatalf("建官方组失败：%v", err)
	}
	if _, err := createAPIKeyAccount(pc.adminToken, "e2e-official-acct-"+suffix, "anthropic", pp.upstream, pp.baseURL, officialGID); err != nil {
		t.Fatalf("建官方账号失败：%v", err)
	}
	// 空兜底组（无账号）—— 负向强证明用
	emptyGID, err := createGroup(pc.adminToken, "e2e-empty-"+suffix, "anthropic")
	if err != nil {
		t.Fatalf("建空组失败：%v", err)
	}

	// Kiro 组 A：fallback → 官方组
	kiroAGID, err := createGroupWithInvalidFallback(pc.adminToken, "e2e-kiroA-"+suffix, "anthropic", officialGID)
	if err != nil {
		t.Fatalf("建 kiroA 组失败：%v", err)
	}
	if _, err := createKiroMaskingAccount(pc.adminToken, "e2e-kiroA-acct-"+suffix, kiroKey, kiroBase, kiroAGID); err != nil {
		t.Fatalf("建 kiroA 账号失败：%v", err)
	}
	gwKeyA, _, err := createGatewayKey(pc.adminToken, "e2e-kiroA-key-"+suffix, kiroAGID, nil, nil)
	if err != nil {
		t.Fatalf("建 kiroA key 失败：%v", err)
	}

	// Kiro 组 B：fallback → 空组
	kiroBGID, err := createGroupWithInvalidFallback(pc.adminToken, "e2e-kiroB-"+suffix, "anthropic", emptyGID)
	if err != nil {
		t.Fatalf("建 kiroB 组失败：%v", err)
	}
	if _, err := createKiroMaskingAccount(pc.adminToken, "e2e-kiroB-acct-"+suffix, kiroKey, kiroBase, kiroBGID); err != nil {
		t.Fatalf("建 kiroB 账号失败：%v", err)
	}
	gwKeyB, _, err := createGatewayKey(pc.adminToken, "e2e-kiroB-key-"+suffix, kiroBGID, nil, nil)
	if err != nil {
		t.Fatalf("建 kiroB key 失败：%v", err)
	}

	t.Run("文本请求不 reroute（Kiro 处理）", func(t *testing.T) {
		st, resp, err := gwClaudeMessages(gwKeyA, kiroModel, "Reply with one short sentence.", false, 64)
		if err != nil {
			t.Fatalf("请求错误：%v", err)
		}
		if st != 200 {
			t.Fatalf("文本请求应由 Kiro 处理并 200，实际 %d：%s", st, truncate(resp, 200))
		}
		t.Logf("✅ 文本请求由 Kiro 处理（不 reroute），200")
	})

	t.Run("vision reroute 到官方组（官方真处理 image）", func(t *testing.T) {
		st, resp, err := gwVisionMessage(gwKeyA, kiroModel)
		if err != nil {
			t.Fatalf("请求错误：%v", err)
		}
		// reroute 到官方组后由官方真正处理 image：200 成功，或对极小测试图返回 image 相关错误
		// （如 "Could not process image"）。两者都证明请求到了能处理 image 的官方账号——与 Kiro
		// 静默忽略 image（200 但答"没看到图"、绝不报 image 错误）形成对比。
		if st != 200 && !bodyContains(resp, "image") {
			t.Fatalf("vision 未被官方处理（非 200 且无 image 错误），st=%d：%s", st, truncate(resp, 200))
		}
		t.Logf("✅ vision reroute 到官方组，官方真处理 image（st=%d）", st)
	})

	t.Run("vision reroute 强证明（兜底空组→失败）", func(t *testing.T) {
		st, resp, err := gwVisionMessage(gwKeyB, kiroModel)
		if err != nil {
			t.Fatalf("请求错误：%v", err)
		}
		// reroute 到空组（无账号）→ 非 200；若得到 200 说明未 reroute（被 Kiro 静默处理）。
		if st == 200 {
			t.Fatalf("vision 应被 reroute 到空兜底组而失败，却得到 200（未 reroute）：%s", truncate(resp, 200))
		}
		t.Logf("✅ vision reroute 强证明：兜底空组无账号 → st=%d（请求确实离开 Kiro 组）", st)
	})
}
