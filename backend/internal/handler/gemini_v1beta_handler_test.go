//go:build unit

package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestGeminiV1BetaHandler_BridgeRoutingDecision Phase 4 P4-3：验证 Bridge Registry 路由决策逻辑。
//
// 规则（docs/relay-architecture-design.md §4.3 + sprint-plan P4-3）：
//   - outbound_protocol == gemini_v1beta（或空 → 派生为 gemini_v1beta）→ native 路径
//   - outbound_protocol == openai_chat / anthropic_messages → Bridge 路径（查 Registry）
//   - Registry 无对应桥 → 503
func TestGeminiV1BetaHandler_BridgeRoutingDecision(t *testing.T) {
	r := service.NewProtocolBridgeRegistry()

	tests := []struct {
		name             string
		outboundProtocol string
		platform         string
		wantBridgePath   bool
		wantBridgeFound  bool
		wantBridgeID     string
	}{
		{
			name:             "outbound=gemini_v1beta → native 路径",
			outboundProtocol: "gemini_v1beta",
			platform:         service.PlatformGemini,
			wantBridgePath:   false,
		},
		{
			name:             "outbound=空 + platform=gemini → 派生 gemini_v1beta → native 路径",
			outboundProtocol: "",
			platform:         service.PlatformGemini,
			wantBridgePath:   false,
		},
		{
			name:             "outbound=openai_chat → Bridge 路径（已注册 P4-1 stub）",
			outboundProtocol: "openai_chat",
			platform:         service.PlatformGemini,
			wantBridgePath:   true,
			wantBridgeFound:  true,
			wantBridgeID:     "gemini_v1beta->openai_chat",
		},
		{
			name:             "outbound=anthropic_messages → Bridge 路径（已注册 P4-1 stub）",
			outboundProtocol: "anthropic_messages",
			platform:         service.PlatformGemini,
			wantBridgePath:   true,
			wantBridgeFound:  true,
			wantBridgeID:     "gemini_v1beta->anthropic_messages",
		},
		{
			name:             "outbound=openai_responses → Bridge 路径（未注册 → 503）",
			outboundProtocol: "openai_responses",
			platform:         service.PlatformGemini,
			wantBridgePath:   true,
			wantBridgeFound:  false, // gemini_v1beta->openai_responses 未注册
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟 GeminiV1BetaModels 中的桥路由决策（P4-3 新增段）
			outboundProto := domain.ResolveOutboundProtocol(tt.outboundProtocol, tt.platform)
			isBridgePath := outboundProto != "" && outboundProto != domain.ProtocolGeminiV1Beta
			require.Equal(t, tt.wantBridgePath, isBridgePath, "桥路由路径判断")

			if !isBridgePath {
				return // native 路径，不需要进一步断言
			}

			// 验证 Bridge Registry 查找
			meta, ok := r.Lookup(domain.ProtocolGeminiV1Beta, outboundProto)
			require.Equal(t, tt.wantBridgeFound, ok, "Registry.Lookup 结果")
			if ok {
				require.Equal(t, tt.wantBridgeID, meta.ID, "桥 ID 应匹配注册值")
				require.Contains(t, meta.Implementation, "stub", "P4-3 阶段所有 Gemini 桥应为 stub")
			}
		})
	}
}

func TestShouldFallbackGeminiModel_KnownFallbackOn404(t *testing.T) {
	t.Parallel()

	res := &service.UpstreamHTTPResult{StatusCode: http.StatusNotFound}
	require.True(t, shouldFallbackGeminiModel("gemini-3.1-pro-preview-customtools", res))
}

func TestShouldFallbackGeminiModel_UnknownModelOn404(t *testing.T) {
	t.Parallel()

	res := &service.UpstreamHTTPResult{StatusCode: http.StatusNotFound}
	require.False(t, shouldFallbackGeminiModel("gemini-future-model", res))
}

func TestShouldFallbackGeminiModel_DelegatesScopeFallback(t *testing.T) {
	t.Parallel()

	res := &service.UpstreamHTTPResult{
		StatusCode: http.StatusForbidden,
		Headers:    http.Header{"Www-Authenticate": []string{"Bearer error=\"insufficient_scope\""}},
		Body:       []byte("insufficient authentication scopes"),
	}
	require.True(t, shouldFallbackGeminiModel("gemini-future-model", res))
}
