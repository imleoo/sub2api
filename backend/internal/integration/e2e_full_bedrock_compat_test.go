//go:build e2e

package integration

// 账号级协议适配 —— Bedrock Converse 兼容 E2E
//
// seed 一个独立的 anthropic apikey 账号（extra.bedrock_compat=true，复用 anthropic 上游），
// 验证：
//   - 非流式请求 → 客户端收到 Bedrock Converse 形态 JSON（output.message / stopReason / usage.*Tokens）
//   - 流式请求   → 收 400（阶段一未实现流式 Converse，由 InspectRequest 兜底拒绝）
//
// 依赖 e2e_full_provision_test.go 的 provisioning helper。未配 anthropic 上游则 skip。

import (
	"encoding/json"
	"fmt"
	"testing"
)

// createBedrockCompatAccount 建一个 extra.bedrock_compat=true 的 anthropic apikey 账号。
func createBedrockCompatAccount(token, name, upstreamKey, baseURL string, groupID int64) (int64, error) {
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
		"extra":       map[string]any{"bedrock_compat": true},
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

func TestE2EFull_BedrockCompatConverse(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	// 独立分组 + bedrock-compat 账号 + 网关 key（复用 anthropic 真实上游）。
	suffix := runNonce()
	gid, err := createGroup(pc.adminToken, "e2e-bedrockcompat-"+suffix, "anthropic")
	if err != nil {
		t.Fatalf("建分组失败：%v", err)
	}
	if _, err := createBedrockCompatAccount(pc.adminToken, "e2e-bedrockcompat-acct-"+suffix, pp.upstream, pp.baseURL, gid); err != nil {
		t.Fatalf("建 bedrock-compat 账号失败：%v", err)
	}
	gwKey, _, err := createGatewayKey(pc.adminToken, "e2e-bedrockcompat-key-"+suffix, gid, nil, nil)
	if err != nil {
		t.Fatalf("建网关 key 失败：%v", err)
	}

	t.Run("非流式→Converse 形态", func(t *testing.T) {
		st, body, err := gwClaudeMessages(gwKey, pp.model, "reply with exactly: BR-OK", false, 32)
		if err != nil {
			t.Fatalf("请求错误：%v", err)
		}
		if st != 200 {
			t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
		}
		// Converse 响应结构：output.message.{role,content}、stopReason、usage.{inputTokens,...}
		var r struct {
			Output struct {
				Message struct {
					Role    string           `json:"role"`
					Content []map[string]any `json:"content"`
				} `json:"message"`
			} `json:"output"`
			StopReason string `json:"stopReason"`
			Usage      struct {
				InputTokens  int `json:"inputTokens"`
				OutputTokens int `json:"outputTokens"`
				TotalTokens  int `json:"totalTokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			t.Fatalf("响应非 JSON：%v\n%s", err, truncate(body, 400))
		}
		if r.Output.Message.Role != "assistant" {
			t.Fatalf("Converse 缺 output.message.role=assistant：%s", truncate(body, 400))
		}
		if len(r.Output.Message.Content) == 0 {
			t.Fatalf("Converse 缺 output.message.content：%s", truncate(body, 400))
		}
		if r.StopReason == "" {
			t.Fatalf("Converse 缺 stopReason：%s", truncate(body, 400))
		}
		if r.Usage.TotalTokens == 0 {
			t.Fatalf("Converse 缺 usage.totalTokens：%s", truncate(body, 400))
		}
		// 确认已脱离 native 形态：native 顶层有 stop_reason，Converse 不应再出现。
		if bodyContains(body, "\"stop_reason\"") {
			t.Fatalf("响应仍含 native stop_reason，改写未生效：%s", truncate(body, 400))
		}
		t.Logf("✅ Bedrock Converse 非流式 OK（stopReason=%q, totalTokens=%d）", r.StopReason, r.Usage.TotalTokens)
	})

	t.Run("流式→Converse 二进制 EventStream", func(t *testing.T) {
		st, body, err := gwClaudeMessages(gwKey, pp.model, "say STREAM in one word", true, 24)
		if err != nil {
			t.Fatalf("请求错误：%v", err)
		}
		if st != 200 {
			t.Fatalf("期望 200，实际 %d：%s", st, truncate(body, 400))
		}
		// Converse 二进制 EventStream：:event-type header 与 payload 字段为明文 ASCII，可字符串断言。
		if !bodyContains(body, "messageStart") {
			t.Fatalf("流式响应缺 messageStart 帧：%s", truncate(body, 200))
		}
		if !bodyContains(body, "metadata") || !bodyContains(body, "inputTokens") {
			t.Fatalf("流式响应缺末尾 metadata{usage}：%s", truncate(body, 200))
		}
		// 不应再是 native SSE（native 为 "event: message_start" 文本帧）。
		if bodyContains(body, "event: message_start") {
			t.Fatalf("流式响应仍是 native SSE，未转 Converse：%s", truncate(body, 200))
		}
		t.Logf("✅ Bedrock Converse 流式 OK（含 messageStart + metadata 帧）")
	})
}
