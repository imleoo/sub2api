//go:build e2e

package integration

// 全功能 E2E —— 多账号选号链（P0 韧性层）
//
// 验证网关的账号选择逻辑：优先级（priority 越小越优先，filterByMinPriority）。
// 观测手段：每个请求落到哪个账号，经 admin usage 日志反查
//   GET /api/v1/admin/usage?api_key_id=<网关key id> → 行的 account_id 即服务账号。
// usage 异步落账，故轮询等待（≤15s，沿用"等待计费落账"模式）。
//
// 失败注入 / DB seed 无关：本用例只用 admin API seed 账号状态，纯 openclaw 真实转发。

import (
	"fmt"
	"testing"
	"time"
)

// createAPIKeyAccountWithFields 同 createAPIKeyAccount，但允许额外字段（如 priority/load_factor）。
func createAPIKeyAccountWithFields(token, name, platform, upstreamKey, baseURL string, groupID int64, extra map[string]any) (int64, error) {
	payload := map[string]any{
		"name":     name,
		"platform": platform,
		"type":     "apikey",
		"credentials": map[string]any{
			"api_key":  upstreamKey,
			"base_url": baseURL,
		},
		"group_ids":   []int64{groupID},
		"concurrency": 10,
	}
	for k, v := range extra {
		payload[k] = v
	}
	env, err := adminAPI(token, "POST", "/api/v1/admin/accounts", payload)
	if err != nil {
		return 0, err
	}
	return int64(dataObj(env)["id"].(float64)), nil
}

// usageAccountIDsForKey 读某网关 key 的 usage 日志（最新在前），返回各行 account_id。
func usageAccountIDsForKey(token string, apiKeyID int64) ([]int64, error) {
	env, err := adminAPI(token, "GET",
		fmt.Sprintf("/api/v1/admin/usage?api_key_id=%d&sort_order=desc&page_size=50", apiKeyID), nil)
	if err != nil {
		return nil, err
	}
	items, _ := dataObj(env)["items"].([]any)
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if f, ok := m["account_id"].(float64); ok {
			ids = append(ids, int64(f))
		}
	}
	return ids, nil
}

// waitUsageCount 轮询直到某 key 的 usage 行数 ≥ want（异步落账），返回最终 account_id 列表。
func waitUsageCount(token string, apiKeyID int64, want int, timeout time.Duration) ([]int64, bool) {
	deadline := time.Now().Add(timeout)
	for {
		ids, err := usageAccountIDsForKey(token, apiKeyID)
		if err == nil && len(ids) >= want {
			return ids, true
		}
		if time.Now().After(deadline) {
			return ids, false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// TestE2EFull_AccountSelectionPriority 验证：同分组多账号时，按账号 priority 选号
// （越小越优先）。两账号共用同一有效 openclaw 上游凭证、仅 priority 不同，
// 断言请求实际落到高优先级（priority 小）账号。
func TestE2EFull_AccountSelectionPriority(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	suffix := runNonce()
	gid, err := createGroup(pc.adminToken, fmt.Sprintf("e2e-sel-grp-%s", suffix), "anthropic")
	if err != nil {
		t.Fatalf("建分组: %v", err)
	}

	// 高优先级账号（priority=1，应被选中）
	hiID, err := createAPIKeyAccountWithFields(pc.adminToken,
		fmt.Sprintf("e2e-sel-hi-%s", suffix), "anthropic", pp.upstream, pp.baseURL, gid,
		map[string]any{"priority": 1})
	if err != nil {
		t.Fatalf("建高优先级账号: %v", err)
	}
	// 低优先级账号（priority=100，不应被选中）
	loID, err := createAPIKeyAccountWithFields(pc.adminToken,
		fmt.Sprintf("e2e-sel-lo-%s", suffix), "anthropic", pp.upstream, pp.baseURL, gid,
		map[string]any{"priority": 100})
	if err != nil {
		t.Fatalf("建低优先级账号: %v", err)
	}

	gwKey, gwKeyID, err := createGatewayKey(pc.adminToken, fmt.Sprintf("e2e-sel-key-%s", suffix), gid, nil, nil)
	if err != nil {
		t.Fatalf("建网关 key: %v", err)
	}

	// 发一个真实请求穿过网关（prompt 带 nonce 避免与其它用例的会话哈希撞）。
	st, body, err := gwClaudeMessages(gwKey, pp.model,
		fmt.Sprintf("reply SEL-OK in one word (nonce %s)", suffix), false, 16)
	if err != nil {
		t.Fatalf("网关请求错误: %v", err)
	}
	if st != 200 {
		if st == 502 || st == 503 || bodyContains(body, "Upstream") || bodyContains(body, "temporarily unavailable") {
			t.Skipf("上游暂不可用（网关已转发并回传 st=%d）：%s", st, truncate(body, 200))
		}
		t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
	}

	ids, ok := waitUsageCount(pc.adminToken, gwKeyID, 1, 15*time.Second)
	if !ok {
		t.Fatalf("usage 落账超时（未观测到 key=%d 的用量行）", gwKeyID)
	}
	served := ids[0] // 最新一行
	if served != hiID {
		t.Fatalf("选号错误：期望落到高优先级账号 %d，实际 %d（低优先级=%d）", hiID, served, loID)
	}
	t.Logf("✅ 选号按 priority 生效：请求落到高优先级账号 %d（低优先级 %d 未被选）", hiID, loID)
}

// TestE2EFull_AccountFailover 验证故障转移：坏账号（错误上游 key）设为更高优先级
// （会被先选），好账号低优先级。请求仍应 200 且最终服务账号=好账号——
// 这逻辑上证明：坏账号被先选→上游失败→被排除→重试落到好账号。
// （usage_log 不记失败尝试，故只能观测到最终成功账号；坏账号高优先级是"先试坏号"的保证。）
func TestE2EFull_AccountFailover(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	suffix := runNonce()
	gid, err := createGroup(pc.adminToken, fmt.Sprintf("e2e-fo-grp-%s", suffix), "anthropic")
	if err != nil {
		t.Fatalf("建分组: %v", err)
	}

	// 坏账号：错误 key + 更高优先级（priority=1，先被选）
	badID, err := createAPIKeyAccountWithFields(pc.adminToken,
		fmt.Sprintf("e2e-fo-bad-%s", suffix), "anthropic",
		"sk-ant-invalid-e2e-failover-"+suffix, pp.baseURL, gid,
		map[string]any{"priority": 1})
	if err != nil {
		t.Fatalf("建坏账号: %v", err)
	}
	// 好账号：有效 key + 低优先级（priority=100）
	goodID, err := createAPIKeyAccountWithFields(pc.adminToken,
		fmt.Sprintf("e2e-fo-good-%s", suffix), "anthropic", pp.upstream, pp.baseURL, gid,
		map[string]any{"priority": 100})
	if err != nil {
		t.Fatalf("建好账号: %v", err)
	}

	gwKey, gwKeyID, err := createGatewayKey(pc.adminToken, fmt.Sprintf("e2e-fo-key-%s", suffix), gid, nil, nil)
	if err != nil {
		t.Fatalf("建网关 key: %v", err)
	}

	st, body, err := gwClaudeMessages(gwKey, pp.model,
		fmt.Sprintf("reply FO-OK in one word (nonce %s)", suffix), false, 16)
	if err != nil {
		t.Fatalf("网关请求错误: %v", err)
	}
	if st != 200 {
		t.Fatalf("故障转移未生效：期望 200（应排除坏账号 %d 重试好账号 %d），实际 %d：%s",
			badID, goodID, st, truncate(body, 400))
	}

	ids, ok := waitUsageCount(pc.adminToken, gwKeyID, 1, 15*time.Second)
	if !ok {
		t.Fatalf("usage 落账超时（未观测到 key=%d 的用量行）", gwKeyID)
	}
	served := ids[0]
	if served != goodID {
		t.Fatalf("故障转移账号错误：期望最终落到好账号 %d，实际 %d（坏账号=%d）", goodID, served, badID)
	}
	t.Logf("✅ 故障转移生效：坏账号 %d（高优先级，先试）失败→排除→重试落到好账号 %d", badID, goodID)
}
