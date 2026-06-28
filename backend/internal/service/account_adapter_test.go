package service

import (
	"bytes"
	"testing"
)

func bedrockAccount() *Account { return &Account{Extra: map[string]any{"bedrock_compat": true}} }
func maskingAccount() *Account { return &Account{Extra: map[string]any{"response_masking": true}} }
func plainAccount() *Account   { return &Account{Extra: map[string]any{}} }
func bothFlagsAccount() *Account {
	return &Account{Extra: map[string]any{"bedrock_compat": true, "response_masking": true}}
}

// TestPickAdapter 验证 adapter 选择：bedrock 优先、masking 次之、都关 → nil。
func TestPickAdapter(t *testing.T) {
	if _, ok := pickAdapter(bedrockAccount()).(*BedrockFixAdapter); !ok {
		t.Errorf("bedrock_compat account should pick *BedrockFixAdapter")
	}
	if _, ok := pickAdapter(maskingAccount()).(*KiroCompatAdapter); !ok {
		t.Errorf("response_masking account should pick *KiroCompatAdapter")
	}
	if a := pickAdapter(plainAccount()); a != nil {
		t.Errorf("plain account should pick nil adapter, got %T", a)
	}
	if a := pickAdapter(nil); a != nil {
		t.Errorf("nil account should pick nil adapter, got %T", a)
	}
	// 两开关同开属配置异常：按 switch 取先匹配项（bedrock 优先）。
	if _, ok := pickAdapter(bothFlagsAccount()).(*BedrockFixAdapter); !ok {
		t.Errorf("both flags should prefer *BedrockFixAdapter")
	}
}

// TestBedrockFixAdapter_PassStream 验证流式请求放行（P3 已实现流式 Converse）。
func TestBedrockFixAdapter_PassStream(t *testing.T) {
	parsed, err := ParseGatewayRequest(
		NewRequestBodyRef([]byte(`{"stream":true,"messages":[{"role":"user","content":"hi"}]}`)), "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !parsed.Stream {
		t.Fatalf("expected parsed.Stream==true")
	}
	act := (&BedrockFixAdapter{}).InspectRequest(parsed)
	if act.Kind != ActionPass {
		t.Errorf("streaming request should now Pass (P3 implemented), got %v", act.Kind)
	}
}

// TestBedrockFixAdapter_StreamInterface 验证流式接口：接管 + Content-Type。
func TestBedrockFixAdapter_StreamInterface(t *testing.T) {
	b := &BedrockFixAdapter{}
	if !b.StreamTakesOver() {
		t.Error("BedrockFixAdapter should take over streaming")
	}
	if ct := b.StreamContentType(); ct != "application/vnd.amazon.eventstream" {
		t.Errorf("StreamContentType got %q", ct)
	}
	if k := (&KiroCompatAdapter{}); k.StreamTakesOver() {
		t.Error("KiroCompatAdapter must not take over streaming (zero regression)")
	}
}

// TestBedrockFixAdapter_RejectDocument 验证含 document 块的请求被 Reject。
func TestBedrockFixAdapter_RejectDocument(t *testing.T) {
	body := `{"messages":[{"role":"user","content":[
		{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"JVBERi0="}},
		{"type":"text","text":"summarize"}
	]}]}`
	parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(body)), "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	act := (&BedrockFixAdapter{}).InspectRequest(parsed)
	if act.Kind != ActionReject {
		t.Errorf("document block request should be Reject, got %v", act.Kind)
	}
}

// TestBedrockFixAdapter_PassPlain 验证普通非流式请求放行。
func TestBedrockFixAdapter_PassPlain(t *testing.T) {
	parsed, err := ParseGatewayRequest(
		NewRequestBodyRef([]byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)), "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	act := (&BedrockFixAdapter{}).InspectRequest(parsed)
	if act.Kind != ActionPass {
		t.Errorf("plain request should Pass, got %v (err=%v)", act.Kind, act.Err)
	}
}

// TestKiroCompatAdapter_Normalize 验证 Kiro 非流式归一：补 stop_reason、移除 inference_geo；
// 请求侧放行；流式不接管（走 needMask）。
func TestKiroCompatAdapter_Normalize(t *testing.T) {
	k := &KiroCompatAdapter{}
	if act := k.InspectRequest(nil); act.Kind != ActionPass {
		t.Errorf("KiroCompatAdapter should Pass, got %v", act.Kind)
	}
	// tool_use 响应缺 stop_reason + 带 inference_geo → 归一。
	body := []byte(`{"type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_x","name":"f","input":{}}],"usage":{"input_tokens":1,"output_tokens":1,"inference_geo":"global"}}`)
	got := k.CorrectNonStreamResponse(body)
	if !bytes.Contains(got, []byte(`"stop_reason":"tool_use"`)) {
		t.Errorf("应补 stop_reason=tool_use：%s", got)
	}
	if bytes.Contains(got, []byte("inference_geo")) {
		t.Errorf("应移除 inference_geo：%s", got)
	}
	if k.StreamTakesOver() {
		t.Errorf("KiroCompat 不应接管流式（保持 needMask 现状）")
	}
}

// TestKiroCompatAdapter_RerouteOnVisionDocument 验证 image/document 请求 → ActionReroute。
func TestKiroCompatAdapter_RerouteOnVisionDocument(t *testing.T) {
	k := &KiroCompatAdapter{}
	cases := []struct {
		name string
		body string
		want ActionKind
	}{
		{"image", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAAB"}}]}]}`, ActionReroute},
		{"document", `{"messages":[{"role":"user","content":[{"type":"document","source":{}}]}]}`, ActionReroute},
		{"text only", `{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`, ActionPass},
		{"string content", `{"messages":[{"role":"user","content":"hi"}]}`, ActionPass},
	}
	for _, tc := range cases {
		parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(tc.body)), "")
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		if act := k.InspectRequest(parsed); act.Kind != tc.want {
			t.Errorf("%s: InspectRequest kind=%v want %v", tc.name, act.Kind, tc.want)
		}
	}
}

// TestHasDocumentBlock 覆盖 document 检测的各情况。
func TestHasDocumentBlock(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"empty", ``, false},
		{"string content", `[{"role":"user","content":"hello"}]`, false},
		{"text only", `[{"role":"user","content":[{"type":"text","text":"hi"}]}]`, false},
		{"has document", `[{"role":"user","content":[{"type":"document"}]}]`, true},
		{"document in 2nd msg", `[{"role":"user","content":[{"type":"text","text":"a"}]},{"role":"user","content":[{"type":"document"}]}]`, true},
	}
	for _, tc := range cases {
		if got := hasDocumentBlock([]byte(tc.raw)); got != tc.want {
			t.Errorf("%s: hasDocumentBlock=%v, want %v", tc.name, got, tc.want)
		}
	}
}
