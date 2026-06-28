package service

import (
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

// TestKiroCompatAdapter_ZeroRegression 验证 Kiro 接壳零回归：请求放行、响应原样返回。
func TestKiroCompatAdapter_ZeroRegression(t *testing.T) {
	k := &KiroCompatAdapter{}
	if act := k.InspectRequest(nil); act.Kind != ActionPass {
		t.Errorf("KiroCompatAdapter should Pass, got %v", act.Kind)
	}
	body := []byte(`{"type":"message","content":[{"type":"text","text":"hi"}]}`)
	if got := k.CorrectNonStreamResponse(body); string(got) != string(body) {
		t.Errorf("KiroCompatAdapter must return body unchanged (zero regression), got %s", got)
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
