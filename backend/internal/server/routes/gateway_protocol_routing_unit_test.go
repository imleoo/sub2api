package routes

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Phase 2 P2-5 路由分发矩阵单测（最小可行版）。
//
// 完整 e2e（6 入口路径 × 5 平台 × 流式/非流式 + 真实上游）需要 testcontainer
// + Playwright，超出会话范围；本套测试验证 routes/gateway.go P2-3 协议感知判断
// 在 5 platform × 4 inbound_protocol 矩阵下的行为正确性，作为 e2e 的最小守护层。

func newContextWithGroup(platform, inboundProtocol string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	apiKey := &service.APIKey{
		ID:    1,
		Key:   "sk-test",
		Group: &service.Group{Platform: platform, InboundProtocol: inboundProtocol},
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	return c
}

// TestGetGroupInboundProtocol_DerivesFromPlatformWhenEmpty 验证：
//   - 历史 group（inbound_protocol=""）按 platform 派生
//   - 新 group（inbound_protocol 明确）保留原值
//   - lingjing 历史 group 派生为空（无通用 inbound 概念）
func TestGetGroupInboundProtocol_DerivesFromPlatformWhenEmpty(t *testing.T) {
	cases := []struct {
		platform        string
		storedProtocol  string
		expectedDerived string
	}{
		// 旧 group：inbound_protocol="" → 按 platform 派生
		{"anthropic", "", domain.ProtocolAnthropicMessages},
		{"openai", "", domain.ProtocolOpenAIChat},
		{"gemini", "", domain.ProtocolGeminiV1Beta},
		{"antigravity", "", domain.ProtocolAnthropicMessages}, // antigravity 入站走 Anthropic Messages
		{"lingjing", "", ""},                                  // lingjing 无通用 inbound 概念 → 空
		// 新 group：明确 inbound_protocol → 保留原值
		{"anthropic", domain.ProtocolAnthropicMessages, domain.ProtocolAnthropicMessages},
		{"openai", domain.ProtocolOpenAIChat, domain.ProtocolOpenAIChat},
		{"openai", domain.ProtocolOpenAIResponses, domain.ProtocolOpenAIResponses}, // 双 inbound 共存
		{"gemini", domain.ProtocolGeminiV1Beta, domain.ProtocolGeminiV1Beta},
		// 双字段不一致：以 inbound_protocol 为准（platform 仅作兜底）
		{"openai", domain.ProtocolAnthropicMessages, domain.ProtocolAnthropicMessages},
	}
	for _, tc := range cases {
		t.Run(tc.platform+"_"+tc.storedProtocol, func(t *testing.T) {
			c := newContextWithGroup(tc.platform, tc.storedProtocol)
			got := getGroupInboundProtocol(c)
			require.Equal(t, tc.expectedDerived, got,
				"platform=%q stored=%q → expected %q", tc.platform, tc.storedProtocol, tc.expectedDerived)
		})
	}
}

// TestIsOpenAIInbound_Matrix 验证 isOpenAIInbound 在 5 platform × 4 inbound 矩阵下的判断。
//
// 这是 P2-3 改造的核心契约：
//   - inbound_protocol ∈ {openai_chat, openai_responses} → true（与 platform 无关）
//   - inbound_protocol 为其他合法值 → false
//   - inbound_protocol 为空时回退到 platform == openai
func TestIsOpenAIInbound_Matrix(t *testing.T) {
	cases := []struct {
		platform string
		inbound  string
		expected bool
		reason   string
	}{
		// 新 group 双 inbound 形态都视为 OpenAI 入站
		{"openai", domain.ProtocolOpenAIChat, true, "openai_chat → OpenAI handler"},
		{"openai", domain.ProtocolOpenAIResponses, true, "openai_responses → OpenAI handler"},
		// 跨 platform 的 OpenAI 入站（DeepSeek 等 OpenAI-compatible 在 openai platform 下）
		{"anthropic", domain.ProtocolOpenAIChat, true, "OpenAI inbound 与 platform 解耦"},

		// 非 OpenAI 入站
		{"anthropic", domain.ProtocolAnthropicMessages, false, "Anthropic Messages → Anthropic handler"},
		{"gemini", domain.ProtocolGeminiV1Beta, false, "Gemini → Gemini handler"},

		// 旧 group：inbound 为空 → 按 platform 兜底
		{"openai", "", true, "legacy openai group → OpenAI handler"},
		{"anthropic", "", false, "legacy anthropic group → Anthropic handler"},
		{"gemini", "", false, "legacy gemini group → Gemini handler"},
		{"antigravity", "", false, "legacy antigravity group → Anthropic handler family"},
		{"lingjing", "", false, "legacy lingjing group → 不走通用 OpenAI 路由"},

		// 无 group context（API key 未挂分组）→ 兜底为 false
	}
	for _, tc := range cases {
		t.Run(tc.platform+"_"+tc.inbound, func(t *testing.T) {
			c := newContextWithGroup(tc.platform, tc.inbound)
			got := isOpenAIInbound(c)
			require.Equal(t, tc.expected, got, tc.reason)
		})
	}
}

// TestIsOpenAIInbound_NoGroupContext API key 没有 group → 返回 false（不应 panic）。
func TestIsOpenAIInbound_NoGroupContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	apiKey := &service.APIKey{ID: 1, Key: "sk-orphan", Group: nil}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	require.False(t, isOpenAIInbound(c))
}

// TestGetGroupPlatform_FallbackChain 验证 getGroupPlatform 在 isOpenAIInbound 内部的回退链。
//
// 主要用于 /v1/images/generations 的 lingjing 平台分支保留（旁路 isOpenAIInbound 直接判断 platform）。
func TestGetGroupPlatform_FallbackChain(t *testing.T) {
	c := newContextWithGroup("lingjing", "")
	require.Equal(t, "lingjing", getGroupPlatform(c), "lingjing 平台需保留 platform 直接判断，用于 /v1/images/generations 混合分支")
}
