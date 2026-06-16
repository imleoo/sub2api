//go:build e2e

package integration

// 全功能 E2E —— provisioning 助手
//
// 与历史 e2e（e2e_gateway_test.go / e2e_user_flow_test.go，黑盒打已运行服务）不同，
// 本套件是「自包含」的：给定 admin 凭证 + 上游 apikey（均走 env 注入，绝不入仓库），
// 通过 admin API 自动 seed 账号/分组/网关 key，再驱动请求穿过网关做端到端断言。
//
// 必需 env：
//   E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD            (默认 admin@sub2api.local / admin123)
//   E2E_ANTHROPIC_UPSTREAM_KEY                       至少配 anthropic，否则整套 skip
//   E2E_ANTHROPIC_UPSTREAM_BASE_URL                  (默认 https://api.anthropic.com)
//   E2E_ANTHROPIC_MODEL                              (默认 claude-sonnet-4-6)
// 可选 env（配了才跑对应平台）：
//   E2E_OPENAI_UPSTREAM_KEY / E2E_OPENAI_UPSTREAM_BASE_URL / E2E_OPENAI_MODEL
//   E2E_GEMINI_UPSTREAM_KEY / E2E_GEMINI_UPSTREAM_BASE_URL / E2E_GEMINI_MODEL
//
// BASE_URL（被测网关地址）复用 e2e_gateway_test.go 的包级变量 baseURL（默认 :8080）。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	e2eAdminEmailEnv    = "E2E_ADMIN_EMAIL"
	e2eAdminPasswordEnv = "E2E_ADMIN_PASSWORD"
)

// platformProvision 是某个上游平台 seed 后的句柄。
type platformProvision struct {
	platform   string
	model      string
	baseURL    string
	upstream   string // 上游 apikey
	groupID    int64
	accountID  int64
	gatewayKey string // 网关侧 API key（客户端用它打我们的网关）
}

// provisionCtx 是整套自包含 e2e 的共享上下文。
type provisionCtx struct {
	adminToken string
	platforms  map[string]*platformProvision // "anthropic" / "openai" / "gemini"
}

var (
	provOnce sync.Once
	provCtx  *provisionCtx
	provErr  error
	nonceSeq int64
)

// runNonce 为每次进程内创建的资源生成稳定且唯一的后缀（避免 dev DB 重复跑累积冲突）。
func runNonce() string {
	n := atomic.AddInt64(&nonceSeq, 1)
	return fmt.Sprintf("%d-%d", os.Getpid(), n)
}

// requireProvision 懒初始化并返回共享上下文；环境不全则 skip 整个调用方测试。
func requireProvision(t *testing.T) *provisionCtx {
	t.Helper()
	provOnce.Do(func() { provCtx, provErr = buildProvision() })
	if provErr != nil {
		t.Skipf("E2E 自包含套件未启用：%v", provErr)
	}
	return provCtx
}

// requirePlatform 返回指定平台的 provision，未配置则 skip。
func (p *provisionCtx) requirePlatform(t *testing.T, platform string) *platformProvision {
	t.Helper()
	pp := p.platforms[platform]
	if pp == nil {
		t.Skipf("未配置 %s 上游凭证，跳过", platform)
	}
	return pp
}

func buildProvision() (*provisionCtx, error) {
	adminEmail := getEnv(e2eAdminEmailEnv, "admin@sub2api.local")
	adminPassword := getEnv(e2eAdminPasswordEnv, "admin123")

	// 上游配置（anthropic 必需，其余可选）
	type upCfg struct{ platform, keyEnv, urlEnv, urlDefault, modelEnv, modelDefault string }
	candidates := []upCfg{
		{"anthropic", "E2E_ANTHROPIC_UPSTREAM_KEY", "E2E_ANTHROPIC_UPSTREAM_BASE_URL", "https://api.anthropic.com", "E2E_ANTHROPIC_MODEL", "claude-sonnet-4-6"},
		{"openai", "E2E_OPENAI_UPSTREAM_KEY", "E2E_OPENAI_UPSTREAM_BASE_URL", "https://api.openai.com/v1", "E2E_OPENAI_MODEL", "gpt-4o-mini"},
		{"gemini", "E2E_GEMINI_UPSTREAM_KEY", "E2E_GEMINI_UPSTREAM_BASE_URL", "https://generativelanguage.googleapis.com", "E2E_GEMINI_MODEL", "gemini-2.5-flash"},
	}

	if strings.TrimSpace(os.Getenv("E2E_ANTHROPIC_UPSTREAM_KEY")) == "" {
		return nil, fmt.Errorf("未设置 E2E_ANTHROPIC_UPSTREAM_KEY")
	}

	token, err := adminLogin(adminEmail, adminPassword)
	if err != nil {
		return nil, fmt.Errorf("admin 登录失败（email=%s）：%w", adminEmail, err)
	}

	ctx := &provisionCtx{adminToken: token, platforms: map[string]*platformProvision{}}
	for _, c := range candidates {
		key := strings.TrimSpace(os.Getenv(c.keyEnv))
		if key == "" {
			continue
		}
		pp := &platformProvision{
			platform: c.platform,
			model:    getEnv(c.modelEnv, c.modelDefault),
			baseURL:  getEnv(c.urlEnv, c.urlDefault),
			upstream: key,
		}
		if err := provisionPlatform(token, pp); err != nil {
			return nil, fmt.Errorf("seed %s 失败：%w", c.platform, err)
		}
		ctx.platforms[c.platform] = pp
	}
	return ctx, nil
}

// provisionPlatform 为某平台建分组 + apikey 账号 + 网关 key。
func provisionPlatform(token string, pp *platformProvision) error {
	suffix := runNonce()
	groupName := fmt.Sprintf("e2e-%s-%s", pp.platform, suffix)

	gid, err := createGroup(token, groupName, pp.platform)
	if err != nil {
		return fmt.Errorf("建分组：%w", err)
	}
	pp.groupID = gid

	aid, err := createAPIKeyAccount(token, fmt.Sprintf("e2e-%s-acct-%s", pp.platform, suffix), pp.platform, pp.upstream, pp.baseURL, gid)
	if err != nil {
		return fmt.Errorf("建账号：%w", err)
	}
	pp.accountID = aid

	gwKey, _, err := createGatewayKey(token, fmt.Sprintf("e2e-%s-key-%s", pp.platform, suffix), gid, nil, nil)
	if err != nil {
		return fmt.Errorf("建网关 key：%w", err)
	}
	pp.gatewayKey = gwKey
	return nil
}

// =============================================================================
// 低层 HTTP / API 助手
// =============================================================================

// apiCall 发起一个带可选 Bearer/x-api-key 的请求，返回 status + 原始 body。
func apiCall(method, path string, headers map[string]string, body []byte) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, baseURL+path, r)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, rb, nil
}

// adminAPI 发起管理端 JSON 请求并解包 {code,message,data}，非 0 code 视为错误。
func adminAPI(token, method, path string, payload any) (map[string]any, error) {
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	}
	st, rb, err := apiCall(method, path, map[string]string{"Authorization": "Bearer " + token}, body)
	if err != nil {
		return nil, err
	}
	var env map[string]any
	if e := json.Unmarshal(rb, &env); e != nil {
		return nil, fmt.Errorf("HTTP %d 非 JSON 响应：%s", st, truncate(rb, 300))
	}
	if code, ok := env["code"].(float64); ok && code != 0 {
		return env, fmt.Errorf("HTTP %d code=%v msg=%v", st, env["code"], env["message"])
	}
	if st >= 400 {
		return env, fmt.Errorf("HTTP %d：%s", st, truncate(rb, 300))
	}
	return env, nil
}

func dataObj(env map[string]any) map[string]any {
	if d, ok := env["data"].(map[string]any); ok {
		return d
	}
	return map[string]any{}
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func adminLogin(email, password string) (string, error) {
	env, err := adminAPI("", "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password})
	if err != nil {
		return "", err
	}
	tok, _ := dataObj(env)["access_token"].(string)
	if tok == "" {
		return "", fmt.Errorf("登录响应无 access_token")
	}
	return tok, nil
}

func createGroup(token, name, platform string) (int64, error) {
	env, err := adminAPI(token, "POST", "/api/v1/admin/groups", map[string]any{
		"name": name, "platform": platform, "subscription_type": "standard", "rate_multiplier": 1,
	})
	if err != nil {
		return 0, err
	}
	return int64(dataObj(env)["id"].(float64)), nil
}

func createAPIKeyAccount(token, name, platform, upstreamKey, baseURL string, groupID int64) (int64, error) {
	env, err := adminAPI(token, "POST", "/api/v1/admin/accounts", map[string]any{
		"name":     name,
		"platform": platform,
		"type":     "apikey",
		"credentials": map[string]any{
			"api_key":  upstreamKey,
			"base_url": baseURL,
		},
		"group_ids":   []int64{groupID},
		"concurrency": 10,
	})
	if err != nil {
		return 0, err
	}
	return int64(dataObj(env)["id"].(float64)), nil
}

// createGatewayKey 建一个客户端用的网关 API key。quota（USD）/ rateLimit5h 可选。
func createGatewayKey(token, name string, groupID int64, quota, rateLimit5h *float64) (string, int64, error) {
	payload := map[string]any{"name": name, "group_id": groupID}
	if quota != nil {
		payload["quota"] = *quota
	}
	if rateLimit5h != nil {
		payload["rate_limit_5h"] = *rateLimit5h
	}
	env, err := adminAPI(token, "POST", "/api/v1/keys", payload)
	if err != nil {
		return "", 0, err
	}
	d := dataObj(env)
	key, _ := d["key"].(string)
	if key == "" {
		key, _ = d["api_key"].(string)
	}
	var id int64
	if f, ok := d["id"].(float64); ok {
		id = int64(f)
	}
	if key == "" {
		return "", 0, fmt.Errorf("建 key 响应无明文 key")
	}
	return key, id, nil
}

// =============================================================================
// 余额 / 用量查询
// =============================================================================

// profileBalance 以某用户 token 读取自身余额。
func profileBalance(token string) (float64, error) {
	env, err := adminAPI(token, "GET", "/api/v1/user/profile", nil)
	if err != nil {
		return 0, err
	}
	bal, ok := dataObj(env)["balance"].(float64)
	if !ok {
		return 0, fmt.Errorf("profile 响应无 balance")
	}
	return bal, nil
}

// waitBalanceDrop 轮询直到余额低于 before（计费可能在响应后异步落账），返回最终余额。
func waitBalanceDrop(token string, before float64, timeout time.Duration) (float64, bool) {
	deadline := time.Now().Add(timeout)
	for {
		bal, err := profileBalance(token)
		if err == nil && bal < before {
			return bal, true
		}
		if time.Now().After(deadline) {
			return bal, false
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// =============================================================================
// 网关数据面调用（穿过被测网关）
// =============================================================================

// gwClaudeMessages 打 POST /v1/messages（Anthropic 格式），用网关 key 走 x-api-key。
func gwClaudeMessages(gwKey, model, prompt string, stream bool, maxTokens int) (int, []byte, error) {
	payload := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"stream":     stream,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	body, _ := json.Marshal(payload)
	return apiCall("POST", "/v1/messages", map[string]string{
		"x-api-key":         gwKey,
		"anthropic-version": "2023-06-01",
	}, body)
}

// gwClaudeCountTokens 打 POST /v1/messages/count_tokens。
func gwClaudeCountTokens(gwKey, model, prompt string) (int, []byte, error) {
	payload := map[string]any{
		"model":    model,
		"messages": []map[string]any{{"role": "user", "content": prompt}},
	}
	body, _ := json.Marshal(payload)
	return apiCall("POST", "/v1/messages/count_tokens", map[string]string{
		"x-api-key":         gwKey,
		"anthropic-version": "2023-06-01",
	}, body)
}

// gwOpenAIChat 打 POST /v1/chat/completions（OpenAI 格式），用网关 key 走 Bearer。
func gwOpenAIChat(gwKey, model, prompt string, maxTokens int) (int, []byte, error) {
	payload := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	body, _ := json.Marshal(payload)
	return apiCall("POST", "/v1/chat/completions", map[string]string{
		"Authorization": "Bearer " + gwKey,
	}, body)
}

// gwGemini 打 POST /v1beta/models/{model}:generateContent（Gemini 格式），
// 用网关 key 走 Bearer。
func gwGemini(gwKey, model, prompt string) (int, []byte, error) {
	payload := map[string]any{
		"contents": []map[string]any{
			{"role": "user", "parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{"maxOutputTokens": 32},
	}
	body, _ := json.Marshal(payload)
	path := fmt.Sprintf("/v1beta/models/%s:generateContent", model)
	return apiCall("POST", path, map[string]string{"Authorization": "Bearer " + gwKey}, body)
}

// jsonField 从原始 body 取一个顶层/嵌套字符串字段（用于轻量断言）。
func bodyContains(b []byte, sub string) bool {
	return strings.Contains(string(b), sub)
}

// createUser 以 admin 身份建用户（带初始余额），返回 user id。
func createUser(token, email, password string, balance float64) (int64, error) {
	env, err := adminAPI(token, "POST", "/api/v1/admin/users", map[string]any{
		"email":    email,
		"password": password,
		"username": strings.SplitN(email, "@", 2)[0],
		"balance":  balance,
	})
	if err != nil {
		return 0, err
	}
	f, ok := dataObj(env)["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("建用户响应无 id")
	}
	return int64(f), nil
}
