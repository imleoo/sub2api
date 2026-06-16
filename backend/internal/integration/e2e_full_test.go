//go:build e2e

package integration

// 全功能 E2E —— 核心运行时 + 计费 + 限额
//
// 自包含：依赖 e2e_full_provision_test.go 的 provisioning（admin seed 账号/分组/key）。
// 运行：scripts/e2e-test.sh（自动起服务 + 注入 env + 跑测试 + 拆环境）。
// 环境不全（无 admin / 无上游 key）时各测试 t.Skip。

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// 1) 网关多平台转发 + 流式 / count_tokens
// ============================================================================

func TestE2EFull_ClaudeForwarding(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	t.Run("非流式", func(t *testing.T) {
		st, body, err := gwClaudeMessages(pp.gatewayKey, pp.model, "reply with exactly: GW-OK", false, 32)
		if err != nil {
			t.Fatalf("请求错误: %v", err)
		}
		if st != 200 {
			t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
		}
		// Anthropic 响应结构：content[].text
		var r struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		_ = json.Unmarshal(body, &r)
		if len(r.Content) == 0 || strings.TrimSpace(r.Content[0].Text) == "" {
			t.Fatalf("响应无 content 文本：%s", truncate(body, 400))
		}
		t.Logf("✅ Claude 非流式转发 OK：%q", r.Content[0].Text)
	})

	t.Run("流式", func(t *testing.T) {
		st, body, err := gwClaudeMessages(pp.gatewayKey, pp.model, "say STREAM-OK", true, 24)
		if err != nil {
			t.Fatalf("请求错误: %v", err)
		}
		if st != 200 {
			t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
		}
		if !bodyContains(body, "event: message_stop") && !bodyContains(body, "message_stop") {
			t.Fatalf("流式响应缺少 message_stop：%s", truncate(body, 400))
		}
		if !bodyContains(body, "content_block_delta") {
			t.Fatalf("流式响应缺少 content_block_delta：%s", truncate(body, 400))
		}
		t.Logf("✅ Claude 流式转发 OK（SSE 含 message_stop）")
	})

	t.Run("count_tokens", func(t *testing.T) {
		st, body, err := gwClaudeCountTokens(pp.gatewayKey, pp.model, "count these tokens please")
		if err != nil {
			t.Fatalf("请求错误: %v", err)
		}
		// 我们的网关已正确转发 count_tokens；上游（第三方中转）若不支持会回 upstream_error，
		// 这属上游能力限制而非网关缺陷 → skip。
		if st != 200 {
			if bodyContains(body, "upstream") {
				t.Skipf("上游不支持 count_tokens（网关已转发并回传上游响应 st=%d）：%s", st, truncate(body, 200))
			}
			t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
		}
		if !bodyContains(body, "input_tokens") {
			t.Fatalf("count_tokens 响应缺少 input_tokens：%s", truncate(body, 400))
		}
		t.Logf("✅ Claude count_tokens OK：%s", truncate(body, 120))
	})
}

func TestE2EFull_OpenAIForwarding(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "openai")

	st, body, err := gwOpenAIChat(pp.gatewayKey, pp.model, "reply with exactly: OAI-OK", 32)
	if err != nil {
		t.Fatalf("请求错误: %v", err)
	}
	if st != 200 {
		t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
	}
	var r struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	_ = json.Unmarshal(body, &r)
	if len(r.Choices) == 0 || strings.TrimSpace(r.Choices[0].Message.Content) == "" {
		t.Fatalf("响应无 choices 文本：%s", truncate(body, 400))
	}
	t.Logf("✅ OpenAI 转发 OK：%q", r.Choices[0].Message.Content)
}

func TestE2EFull_GeminiForwarding(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "gemini")

	st, body, err := gwGemini(pp.gatewayKey, pp.model, "say GEM-OK in one word")
	if err != nil {
		t.Fatalf("请求错误: %v", err)
	}
	// 上游（第三方中转）gemini 后端不可用时（502/INTERNAL/upstream）→ skip，
	// 我们的网关已正确转发并回传上游响应，非网关缺陷。
	if st != 200 {
		if st == 502 || st == 503 || bodyContains(body, "Upstream") || bodyContains(body, "INTERNAL") || bodyContains(body, "temporarily unavailable") {
			t.Skipf("上游 gemini 暂不可用（网关已转发并回传上游响应 st=%d）：%s", st, truncate(body, 200))
		}
		t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
	}
	var r struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	_ = json.Unmarshal(body, &r)
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 || strings.TrimSpace(r.Candidates[0].Content.Parts[0].Text) == "" {
		t.Fatalf("响应无 candidates 文本：%s", truncate(body, 400))
	}
	t.Logf("✅ Gemini 转发 OK：%q", r.Candidates[0].Content.Parts[0].Text)
}

// ============================================================================
// 2) 计费：可计费请求扣余额，count_tokens 不扣
// ============================================================================

func TestE2EFull_BillingDeduction(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	t.Run("可计费请求扣减余额", func(t *testing.T) {
		before, err := profileBalance(pc.adminToken)
		if err != nil {
			t.Fatalf("读余额失败: %v", err)
		}
		st, body, err := gwClaudeMessages(pp.gatewayKey, pp.model, "hi", false, 16)
		if err != nil || st != 200 {
			t.Fatalf("计费请求未成功 st=%d err=%v：%s", st, err, truncate(body, 300))
		}
		after, dropped := waitBalanceDrop(pc.adminToken, before, 15*time.Second)
		if !dropped {
			t.Fatalf("可计费请求后余额未下降：before=%.8f after=%.8f", before, after)
		}
		t.Logf("✅ 余额扣减 OK：%.8f → %.8f (Δ=%.8f)", before, after, before-after)
	})

	t.Run("count_tokens 不计费", func(t *testing.T) {
		before, err := profileBalance(pc.adminToken)
		if err != nil {
			t.Fatalf("读余额失败: %v", err)
		}
		st, ctBody, err := gwClaudeCountTokens(pp.gatewayKey, pp.model, "count me")
		if err != nil {
			t.Fatalf("count_tokens 请求错误: %v", err)
		}
		if st != 200 {
			t.Skipf("上游不支持 count_tokens，跳过其计费断言（st=%d）：%s", st, truncate(ctBody, 200))
		}
		// 给异步计费一点时间，确认余额没动
		time.Sleep(3 * time.Second)
		after, err := profileBalance(pc.adminToken)
		if err != nil {
			t.Fatalf("读余额失败: %v", err)
		}
		if after != before {
			t.Fatalf("count_tokens 不应计费，但余额变化：before=%.8f after=%.8f", before, after)
		}
		t.Logf("✅ count_tokens 未计费（余额不变 %.8f）", after)
	})
}

// ============================================================================
// 3) Key 配额（quota，USD 总额上限）拦截
// ============================================================================

func TestE2EFull_KeyQuotaEnforcement(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	quota := 0.00001 // 极小总额：消费任意一次后即超额
	keyName := fmt.Sprintf("e2e-quota-%s", runNonce())
	gwKey, keyID, err := createGatewayKey(pc.adminToken, keyName, pp.groupID, &quota, nil)
	if err != nil {
		t.Fatalf("建配额 key 失败: %v", err)
	}
	t.Cleanup(func() { _, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/keys/%d", keyID), nil) })

	blocked := sendUntilBlocked(t, func() (int, []byte) {
		st, body, _ := gwClaudeMessages(gwKey, pp.model, "hi", false, 16)
		return st, body
	}, 4)
	if !blocked {
		t.Fatalf("配额 quota=%.5f 未在 4 次请求内拦截", quota)
	}
	t.Logf("✅ 配额拦截生效")
}

// ============================================================================
// 4) 余额不足拒绝（独立低余额用户）
// ============================================================================

func TestE2EFull_BalanceInsufficientRejection(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	// 建一个余额=0 的用户（网关预检阈值为 Balance <= 0），用其身份建 key 打网关 → 应被拒绝
	email := fmt.Sprintf("e2e-poor-%s@example.com", runNonce())
	password := "E2ePass!234"
	uid, err := createUser(pc.adminToken, email, password, 0)
	if err != nil {
		t.Skipf("建低余额用户失败（可能注册策略限制）：%v", err)
	}
	t.Cleanup(func() { _, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/admin/users/%d", uid), nil) })

	userTok, err := adminLogin(email, password)
	if err != nil {
		t.Fatalf("低余额用户登录失败: %v", err)
	}
	// 确认余额确实 <= 0（环境若发放注册赠额则无法构造该场景 → skip）
	if bal, err := profileBalance(userTok); err == nil && bal > 0 {
		t.Skipf("用户余额 %.8f > 0（环境发放了注册赠额），无法构造余额不足场景", bal)
	}
	gwKey, _, err := createGatewayKey(userTok, "e2e-poor-key", pp.groupID, nil, nil)
	if err != nil {
		t.Skipf("低余额用户建 key 失败（可能无分组权限）：%v", err)
	}

	st, body, err := gwClaudeMessages(gwKey, pp.model, "hi", false, 16)
	if err != nil {
		t.Fatalf("请求错误: %v", err)
	}
	if st == 200 {
		t.Fatalf("余额不足却返回 200（应被拒绝）：%s", truncate(body, 300))
	}
	t.Logf("✅ 余额不足拒绝生效（st=%d）：%s", st, truncate(body, 200))
}

// ============================================================================
// 5) 限流（rate_limit_5h，USD/5h 窗口）
// ============================================================================

func TestE2EFull_RateLimit(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	rl := 0.00001 // 极小窗口额度
	keyName := fmt.Sprintf("e2e-rl-%s", runNonce())
	gwKey, keyID, err := createGatewayKey(pc.adminToken, keyName, pp.groupID, nil, &rl)
	if err != nil {
		t.Fatalf("建限流 key 失败: %v", err)
	}
	t.Cleanup(func() { _, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/keys/%d", keyID), nil) })

	blocked := sendUntilBlocked(t, func() (int, []byte) {
		st, body, _ := gwClaudeMessages(gwKey, pp.model, "hi", false, 16)
		return st, body
	}, 4)
	if !blocked {
		t.Fatalf("限流 rate_limit_5h=%.5f 未在 4 次请求内拦截", rl)
	}
	t.Logf("✅ 限流拦截生效")
}

// ============================================================================
// 6) API Key 生命周期：建 → 列 → 查 → 禁用 → 删
// ============================================================================

func TestE2EFull_APIKeyLifecycle(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	name := fmt.Sprintf("e2e-life-%s", runNonce())
	_, keyID, err := createGatewayKey(pc.adminToken, name, pp.groupID, nil, nil)
	if err != nil {
		t.Fatalf("建 key 失败: %v", err)
	}

	// 列表能找到
	env, err := adminAPI(pc.adminToken, "GET", "/api/v1/keys?page=1&page_size=100", nil)
	if err != nil {
		t.Fatalf("列 key 失败: %v", err)
	}
	if !bodyContains([]byte(fmt.Sprint(env)), name) {
		t.Fatalf("新建 key %q 未出现在列表", name)
	}

	// 查单个
	if _, err := adminAPI(pc.adminToken, "GET", fmt.Sprintf("/api/v1/keys/%d", keyID), nil); err != nil {
		t.Fatalf("查 key 失败: %v", err)
	}

	// 禁用（status=inactive，合法值 active|inactive）
	if _, err := adminAPI(pc.adminToken, "PUT", fmt.Sprintf("/api/v1/keys/%d", keyID), map[string]any{"status": "inactive"}); err != nil {
		t.Fatalf("禁用 key 失败: %v", err)
	}

	// 删除
	if _, err := adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/keys/%d", keyID), nil); err != nil {
		t.Fatalf("删 key 失败: %v", err)
	}
	t.Logf("✅ API Key 生命周期 OK")
}

// ============================================================================
// 7) admin 账号 / 分组 CRUD
// ============================================================================

func TestE2EFull_AdminAccountGroupCRUD(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	// 建分组
	gname := fmt.Sprintf("e2e-crud-%s", runNonce())
	gid, err := createGroup(pc.adminToken, gname, "anthropic")
	if err != nil {
		t.Fatalf("建分组失败: %v", err)
	}

	// 建账号（指向上游）→ 绑该分组
	aid, err := createAPIKeyAccount(pc.adminToken, fmt.Sprintf("e2e-crud-acct-%s", runNonce()), "anthropic", pp.upstream, pp.baseURL, gid)
	if err != nil {
		t.Fatalf("建账号失败: %v", err)
	}

	// 改账号（concurrency）
	if _, err := adminAPI(pc.adminToken, "PUT", fmt.Sprintf("/api/v1/admin/accounts/%d", aid), map[string]any{"concurrency": 7}); err != nil {
		t.Fatalf("改账号失败: %v", err)
	}

	// 列分组能找到
	env, err := adminAPI(pc.adminToken, "GET", "/api/v1/admin/groups?page=1&page_size=100", nil)
	if err != nil {
		t.Fatalf("列分组失败: %v", err)
	}
	if !bodyContains([]byte(fmt.Sprint(env)), gname) {
		t.Fatalf("新建分组 %q 未出现在列表", gname)
	}

	// 删账号 + 删分组
	if _, err := adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/admin/accounts/%d", aid), nil); err != nil {
		t.Fatalf("删账号失败: %v", err)
	}
	if _, err := adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/admin/groups/%d", gid), nil); err != nil {
		t.Fatalf("删分组失败: %v", err)
	}
	t.Logf("✅ admin 账号/分组 CRUD OK")
}

// ============================================================================
// 共享断言助手
// ============================================================================

// sendUntilBlocked 重复发请求，直到出现非 200（被拦截）或超过 maxTries。
// 每次成功后等待计费落账，确保配额/限流计数推进。
func sendUntilBlocked(t *testing.T, send func() (int, []byte), maxTries int) bool {
	t.Helper()
	for i := 0; i < maxTries; i++ {
		st, body := send()
		if st != 200 {
			t.Logf("第 %d 次被拦截 st=%d：%s", i+1, st, truncate(body, 200))
			return true
		}
		t.Logf("第 %d 次通过（等待计费落账）", i+1)
		time.Sleep(2 * time.Second)
	}
	return false
}
