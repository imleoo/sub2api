//go:build e2e

package integration

// 复现 CLAUDE.md「已知陷阱 #1」：跨平台批量修改账号导致 OpenAI 模型映射丢失。
//
// 根因（见 root-cause 调查）：前端 BulkEditAccountModal 在混选不同平台账号 + 勾"模型限制"
// 时，构造一份平台无关的 model_mapping 无差别下发给所有选中账号；后端 JSONB `||` 按顶层键
// 合并 → OpenAI 账号原有的精确映射(gpt-5.3-codex -> 上游真实模型)被整块顶掉 → 网关
// IsModelSupported=false → SelectAccountForModel 无可用账号 → 503 "Service temporarily unavailable"。
//
// 复现策略：固定同一个网关请求 (model=gpt-5.3-codex)，唯一变量是中间那次 bulk-update。
// 基线 200 → 批量改后 503，即为根因铁证（排除"模型名本来就错"的干扰）。
//
// 注意：本测试证明的是「机制成立」。永久回归守护应落在前端 Vitest（BulkEditAccountModal
// 混选多平台时按账号平台分别构造 model_mapping），因为根因在前端 payload 构造、后端按
// payload 忠实执行。

import (
	"fmt"
	"testing"
)

func TestE2EFull_BatchEditModelMappingRepro(t *testing.T) {
	pc := requireProvision(t)
	oa := pc.requirePlatform(t, "openai")

	upstreamModel := oa.model      // E2E_OPENAI_MODEL：用户第三方中转只认它（gpt-5.3）
	clientModel := "gpt-5.3-codex" // 客户端请求名，靠 model_mapping 翻译成 upstreamModel

	// 1) victim：独立分组 + 带精确映射的 OpenAI 账号 + 网关 key
	vGroup, err := createGroup(pc.adminToken, fmt.Sprintf("e2e-repro-oa-%s", runNonce()), "openai")
	if err != nil {
		t.Fatalf("建 victim 分组失败: %v", err)
	}
	victimID, err := createAccountWithMapping(pc.adminToken,
		fmt.Sprintf("e2e-repro-oa-acct-%s", runNonce()), "openai", oa.upstream, oa.baseURL, vGroup,
		map[string]string{clientModel: upstreamModel})
	if err != nil {
		t.Fatalf("建 OpenAI 账号(带映射)失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/admin/accounts/%d", victimID), nil)
	})

	gwKey, keyID, err := createGatewayKey(pc.adminToken, fmt.Sprintf("e2e-repro-key-%s", runNonce()), vGroup, nil, nil)
	if err != nil {
		t.Fatalf("建网关 key 失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/keys/%d", keyID), nil)
	})

	// 2) 基线：映射生效，gpt-5.3-codex 经翻译应 200
	st, body, err := gwOpenAIChat(gwKey, clientModel, "reply with exactly: BASE-OK", 16)
	if err != nil {
		t.Fatalf("基线请求错误: %v", err)
	}
	if st != 200 {
		t.Skipf("基线未达 200（st=%d）：映射或中转模型名不匹配，无法构造复现前提，请核对 E2E_OPENAI_MODEL：%s", st, truncate(body, 300))
	}
	t.Logf("✅ 基线 200：客户端 %q 经映射→上游 %q 转发成功", clientModel, upstreamModel)

	// 3) 第二平台账号(gemini，独立分组，避免触发混合渠道确认门) + 混选 bulk-update
	gGroup, err := createGroup(pc.adminToken, fmt.Sprintf("e2e-repro-gem-%s", runNonce()), "gemini")
	if err != nil {
		t.Fatalf("建 gemini 分组失败: %v", err)
	}
	otherID, err := createAccountWithMapping(pc.adminToken,
		fmt.Sprintf("e2e-repro-gem-acct-%s", runNonce()), "gemini",
		"e2e-dummy-gemini-key", "https://generativelanguage.googleapis.com", gGroup, nil)
	if err != nil {
		t.Fatalf("建 gemini 账号失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/admin/accounts/%d", otherID), nil)
	})

	// 模拟前端混选多平台 + 白名单(只剩 gemini 模型)：下发不含 gpt-5.3-codex 的 model_mapping。
	// 不改 group_ids → 不触发 confirm_mixed_channel_risk 门。
	if err := bulkUpdateModelMapping(pc.adminToken, []int64{victimID, otherID},
		map[string]string{"gemini-2.5-flash": "gemini-2.5-flash"}); err != nil {
		t.Fatalf("bulk-update 失败: %v", err)
	}
	t.Logf("⚙️ 已混选 [openai=%d, gemini=%d] 批量修改，下发 model_mapping={gemini-2.5-flash}", victimID, otherID)

	// 4) 同一请求再打一次：victim 映射被顶掉 → 期望非 200
	st2, body2, err := gwOpenAIChat(gwKey, clientModel, "reply with exactly: AFTER", 16)
	if err != nil {
		t.Fatalf("复现请求错误: %v", err)
	}
	if st2 == 200 {
		t.Fatalf("❌ 未复现：批量改后 %q 仍返回 200（预期映射被覆盖而失败）：%s", clientModel, truncate(body2, 300))
	}
	t.Logf("🔴 复现成功：同一请求 200 → %d —— 映射被跨平台 bulk-update 顶掉：%s", st2, truncate(body2, 300))
}

// createAccountWithMapping 建 apikey 账号，可选在 credentials 内带 model_mapping。
func createAccountWithMapping(token, name, platform, upstreamKey, baseURL string, groupID int64, mapping map[string]string) (int64, error) {
	creds := map[string]any{"api_key": upstreamKey, "base_url": baseURL}
	if mapping != nil {
		creds["model_mapping"] = mapping
	}
	env, err := adminAPI(token, "POST", "/api/v1/admin/accounts", map[string]any{
		"name":        name,
		"platform":    platform,
		"type":        "apikey",
		"credentials": creds,
		"group_ids":   []int64{groupID},
		"concurrency": 10,
	})
	if err != nil {
		return 0, err
	}
	f, ok := dataObj(env)["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("建账号响应无 id")
	}
	return int64(f), nil
}

// bulkUpdateModelMapping 调用 POST /api/v1/admin/accounts/bulk-update，
// 仅下发 credentials.model_mapping（不改 group_ids，规避混合渠道确认门）。
func bulkUpdateModelMapping(token string, ids []int64, mapping map[string]string) error {
	_, err := adminAPI(token, "POST", "/api/v1/admin/accounts/bulk-update", map[string]any{
		"account_ids": ids,
		"credentials": map[string]any{"model_mapping": mapping},
	})
	return err
}
