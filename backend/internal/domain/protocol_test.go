package domain

import "testing"

// Phase 2 P2-2 protocol 常量与派生函数单测。
//
// 验收：取值与 docs/glossary.md §1.1 一致；空值由派生表兜底。

func TestAllProtocolsMatchesGlossary(t *testing.T) {
	got := AllProtocols()
	want := []string{
		"anthropic_messages",
		"openai_chat",
		"openai_responses",
		"gemini_v1beta",
	}
	if len(got) != len(want) {
		t.Fatalf("AllProtocols length = %d, want %d", len(got), len(want))
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("AllProtocols[%d] = %q, want %q", i, got[i], p)
		}
	}
}

func TestIsValidProtocol(t *testing.T) {
	cases := map[string]bool{
		"anthropic_messages": true,
		"openai_chat":        true,
		"openai_responses":   true,
		"gemini_v1beta":      true,
		// 短名一律视为不合法（消除 §1.1 决议 #7）
		"openai":    false,
		"anthropic": false,
		"gemini":    false,
		// 空值视为未指定，由派生表兜底
		"": false,
		// 别名 / 错误形式
		"openai_completions":     false,
		"anthropic-messages":     false,
		"Gemini_V1Beta":          false,
		"anthropic_messages_v2":  false,
	}
	for in, want := range cases {
		if got := IsValidProtocol(in); got != want {
			t.Errorf("IsValidProtocol(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveInboundProtocol_PreservesValid(t *testing.T) {
	// 已合法 → 不查派生表
	cases := []string{
		"anthropic_messages",
		"openai_chat",
		"openai_responses",
		"gemini_v1beta",
	}
	for _, p := range cases {
		if got := ResolveInboundProtocol(p, "anything"); got != p {
			t.Errorf("ResolveInboundProtocol(%q, ...) = %q, want %q (preserved)", p, got, p)
		}
	}
}

func TestResolveInboundProtocol_FallsBackToPlatform(t *testing.T) {
	cases := map[string]string{
		"anthropic":   "anthropic_messages",
		"openai":      "openai_chat",
		"gemini":      "gemini_v1beta",
		"antigravity": "anthropic_messages", // antigravity 入站走 Anthropic Messages
		"lingjing":    "",                   // lingjing 不走通用 inbound 路由
		"unknown":     "",                   // 未登记 platform 返回空（调用方应回退）
	}
	for platform, want := range cases {
		if got := ResolveInboundProtocol("", platform); got != want {
			t.Errorf("ResolveInboundProtocol(\"\", %q) = %q, want %q", platform, got, want)
		}
	}
}

func TestResolveOutboundProtocol_FallsBackToPlatform(t *testing.T) {
	cases := map[string]string{
		"anthropic":   "anthropic_messages",
		"openai":      "openai_chat", // 保守默认；Phase 3 P3-6 能力探测可升级到 openai_responses
		"gemini":      "gemini_v1beta",
		"antigravity": "anthropic_messages",
		"lingjing":    "",
		"unknown":     "",
	}
	for platform, want := range cases {
		if got := ResolveOutboundProtocol("", platform); got != want {
			t.Errorf("ResolveOutboundProtocol(\"\", %q) = %q, want %q", platform, got, want)
		}
	}
}

// TestResolveProtocol_InvalidInputFallsBackNotPanic 非空但非法的字段会回退到派生表（防御性）。
func TestResolveProtocol_InvalidInputFallsBackNotPanic(t *testing.T) {
	if got := ResolveInboundProtocol("garbage", "openai"); got != "openai_chat" {
		t.Errorf("invalid inbound 'garbage' on openai should fall back to openai_chat, got %q", got)
	}
	if got := ResolveOutboundProtocol("garbage", "anthropic"); got != "anthropic_messages" {
		t.Errorf("invalid outbound 'garbage' on anthropic should fall back to anthropic_messages, got %q", got)
	}
}
