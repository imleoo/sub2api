//go:build e2e

package integration

// 功能 45（月度对账 Vendor Report）e2e：
//   1. 建带初始余额的用户 → 当月对账单 closed=false、期初=期末=初始余额、gap≈0；
//   2. months 列表含当月；
//   3. 用户端/管理端导出均返回 200 + xlsx MIME + attachment；
//   4. 走网关产生一笔真实消费后，utilisation_after > 0 且期末余额 = 初始余额 − 消费。
//
// zhiguofan fork-only: 月度对账（功能 45）。

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"testing"
	"time"
)

const statementMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

func getStatementEnv(token string, userQuery string) (map[string]any, error) {
	st, body, err := apiCall("GET", "/api/v1/usage/statement?"+userQuery,
		map[string]string{"Authorization": "Bearer " + token}, nil)
	if err != nil {
		return nil, err
	}
	if st != 200 {
		return nil, fmt.Errorf("statement 返回 %d: %s", st, truncate(body, 300))
	}
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	return env, nil
}

func TestE2EFull_MonthlyStatement(t *testing.T) {
	pc := requireProvision(t)

	const initialBalance = 50.0
	email := fmt.Sprintf("e2e-stmt-%s@test.local", runNonce())
	password := "e2e-stmt-passw0rd"
	userID, err := createUser(pc.adminToken, email, password, initialBalance)
	if err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/admin/users/%d", userID), nil)
	})
	userToken, err := adminLogin(email, password)
	if err != nil {
		t.Fatalf("用户登录失败: %v", err)
	}

	month := time.Now().Format("2006-01")
	query := "month=" + url.QueryEscape(month)

	// 1) 新用户当月对账单：期初=期末=初始余额、未封账、gap≈0。
	env, err := getStatementEnv(userToken, query)
	if err != nil {
		t.Fatalf("拉取对账单失败: %v", err)
	}
	d := dataObj(env)
	if closed, _ := d["closed"].(bool); closed {
		t.Fatalf("当月对账单不应为已封账")
	}
	opening, _ := d["opening_balance"].(float64)
	closing, _ := d["closing_balance"].(float64)
	if math.Abs(closing-initialBalance) > 1e-6 {
		t.Fatalf("期末余额应为初始余额 %.2f，实际 %.6f", initialBalance, closing)
	}
	if math.Abs(opening-closing) > 1e-6 {
		t.Fatalf("无流水时期初应等于期末，opening=%.6f closing=%.6f", opening, closing)
	}
	t.Logf("✅ 新用户当月对账单：opening=closing=%.2f closed=false", closing)

	// 2) months 列表含当月。
	st, body, err := apiCall("GET", "/api/v1/usage/statement/months",
		map[string]string{"Authorization": "Bearer " + userToken}, nil)
	if err != nil || st != 200 {
		t.Fatalf("months 失败: st=%d err=%v", st, err)
	}
	if !bodyContains(body, month) {
		t.Fatalf("months 应含当月 %s: %s", month, truncate(body, 200))
	}

	// 3) 用户端导出：200 + xlsx MIME + attachment（当月带 -partial）。
	stH, hdr, xbody, err := apiCallH("GET", "/api/v1/usage/statement/export?"+query,
		map[string]string{"Authorization": "Bearer " + userToken}, nil)
	if err != nil || stH != 200 {
		t.Fatalf("用户导出失败: st=%d err=%v %s", stH, err, truncate(xbody, 200))
	}
	if ct := hdr.Get("Content-Type"); !strings.Contains(ct, statementMIME) {
		t.Fatalf("用户导出 Content-Type 异常: %s", ct)
	}
	if cd := hdr.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "-partial") {
		t.Fatalf("用户导出 Content-Disposition 异常: %s", cd)
	}
	if len(xbody) == 0 {
		t.Fatalf("用户导出内容为空")
	}
	t.Logf("✅ 用户端导出 %d bytes（-partial）", len(xbody))

	// 4) 管理端导出同用户。
	stA, hdrA, abody, err := apiCallH("GET",
		fmt.Sprintf("/api/v1/admin/users/%d/statement/export?%s", userID, query),
		map[string]string{"Authorization": "Bearer " + pc.adminToken}, nil)
	if err != nil || stA != 200 {
		t.Fatalf("管理端导出失败: st=%d err=%v %s", stA, err, truncate(abody, 200))
	}
	if ct := hdrA.Get("Content-Type"); !strings.Contains(ct, statementMIME) {
		t.Fatalf("管理端导出 Content-Type 异常: %s", ct)
	}
	t.Logf("✅ 管理端导出 %d bytes", len(abody))

	// 5) 真实消费 → utilisation 与期末余额联动（依赖 openai 上游可用，不可用则跳过本段）。
	oa := pc.requirePlatform(t, "openai")
	group, err := createGroup(pc.adminToken, fmt.Sprintf("e2e-stmt-%s", runNonce()), "openai")
	if err != nil {
		t.Fatalf("建分组失败: %v", err)
	}
	acctID, err := createAccountWithMapping(pc.adminToken,
		fmt.Sprintf("e2e-stmt-acct-%s", runNonce()), "openai", oa.upstream, oa.baseURL, group, nil)
	if err != nil {
		t.Fatalf("建账号失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/admin/accounts/%d", acctID), nil)
	})
	// 授权用户使用该分组后，以用户身份建 key（消费记入该用户名下）。
	if _, err := adminAPI(pc.adminToken, "PUT", fmt.Sprintf("/api/v1/admin/users/%d", userID),
		map[string]any{"allowed_groups": []int64{group}}); err != nil {
		t.Fatalf("授权分组失败: %v", err)
	}
	gwKey, keyID, err := createGatewayKey(userToken, fmt.Sprintf("e2e-stmt-key-%s", runNonce()), group, nil, nil)
	if err != nil {
		t.Fatalf("用户建网关 key 失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/keys/%d", keyID), nil)
	})

	stGW, gwBody, err := gwOpenAIChat(gwKey, oa.model, "reply with exactly: STMT-OK", 16)
	if err != nil || stGW != 200 {
		t.Skipf("网关消费未达 200（st=%d err=%v）：跳过消费联动断言：%s", stGW, err, truncate(gwBody, 200))
	}
	// 计费落库为异步 worker，轮询等待 utilisation 出现。
	deadline := time.Now().Add(30 * time.Second)
	var utilisation, closing2 float64
	for {
		env2, err := getStatementEnv(userToken, query)
		if err != nil {
			t.Fatalf("消费后拉取对账单失败: %v", err)
		}
		d2 := dataObj(env2)
		totals, _ := d2["totals"].(map[string]any)
		utilisation, _ = totals["utilisation_after"].(float64)
		closing2, _ = d2["closing_balance"].(float64)
		if utilisation > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if utilisation <= 0 {
		t.Fatalf("消费后 utilisation_after 应 > 0")
	}
	if math.Abs(closing2-(initialBalance-utilisation)) > 1e-6 {
		t.Fatalf("期末余额应 = 初始余额 − 消费：%.8f vs %.2f − %.8f", closing2, initialBalance, utilisation)
	}
	t.Logf("✅ 消费联动：utilisation=%.8f closing=%.8f", utilisation, closing2)
}
