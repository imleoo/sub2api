package conversecompat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// jsonEqual 比较两段 JSON 是否语义相等（忽略字段顺序）。
func jsonEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not valid JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("want is not valid JSON: %v\n%s", err, want)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("JSON mismatch\n got: %s\nwant: %s", got, want)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// TestAnthropicToConverseJSON_Text 对拍真实非流式纯文本样本（§8 fixture）。
func TestAnthropicToConverseJSON_Text(t *testing.T) {
	native := readFixture(t, "native_nonstream_text.json")
	got := AnthropicToConverseJSON(native)
	want := []byte(`{
		"output":{"message":{"role":"assistant","content":[
			{"text":"What would you like me to respond to?"}
		]}},
		"stopReason":"end_turn",
		"usage":{"inputTokens":13,"outputTokens":12,"totalTokens":25,
			"cacheReadInputTokens":0,"cacheWriteInputTokens":0}
	}`)
	jsonEqual(t, got, want)
}

// TestAnthropicToConverseJSON_ToolUse 对拍真实非流式工具调用样本（§8 fixture）。
// 重点：tool_use.id → toolUse.toolUseId 原样保留 toolu_bdrk_ 前缀。
func TestAnthropicToConverseJSON_ToolUse(t *testing.T) {
	native := readFixture(t, "native_nonstream_tooluse.json")
	got := AnthropicToConverseJSON(native)
	want := []byte(`{
		"output":{"message":{"role":"assistant","content":[
			{"toolUse":{"toolUseId":"toolu_bdrk_01NGigZFiUp65AtMat2WLdFm","name":"get_weather","input":{"city":"Paris"}}}
		]}},
		"stopReason":"tool_use",
		"usage":{"inputTokens":657,"outputTokens":38,"totalTokens":695,
			"cacheReadInputTokens":0,"cacheWriteInputTokens":0}
	}`)
	jsonEqual(t, got, want)
}

// TestCacheCreationRenamedToWrite 验证 cache_creation_input_tokens → cacheWriteInputTokens（§11.3）。
func TestCacheCreationRenamedToWrite(t *testing.T) {
	native := []byte(`{"type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],
		"stop_reason":"end_turn",
		"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":3,
			"cache_creation_input_tokens":7,
			"cache_creation":{"ephemeral_5m_input_tokens":4,"ephemeral_1h_input_tokens":3}}}`)
	got := AnthropicToConverseJSON(native)
	want := []byte(`{"output":{"message":{"role":"assistant","content":[{"text":"hi"}]}},
		"stopReason":"end_turn",
		"usage":{"inputTokens":10,"outputTokens":5,"totalTokens":15,
			"cacheReadInputTokens":3,"cacheWriteInputTokens":7}}`)
	jsonEqual(t, got, want)
}

// TestThinkingAndRedacted 验证 thinking / redacted_thinking 映射。
func TestThinkingAndRedacted(t *testing.T) {
	native := []byte(`{"type":"message","role":"assistant","content":[
		{"type":"thinking","thinking":"let me think","signature":"sig123"},
		{"type":"redacted_thinking","data":"ENCRYPTED"},
		{"type":"text","text":"answer"}
	],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	got := AnthropicToConverseJSON(native)
	want := []byte(`{"output":{"message":{"role":"assistant","content":[
		{"reasoningContent":{"reasoningText":{"text":"let me think","signature":"sig123"}}},
		{"reasoningContent":{"redactedContent":"ENCRYPTED"}},
		{"text":"answer"}
	]}},"stopReason":"end_turn",
	"usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2,"cacheReadInputTokens":0,"cacheWriteInputTokens":0}}`)
	jsonEqual(t, got, want)
}

// TestStopReasonMapping 验证 stop_reason 各分支（含无 Converse 对应的显式映射）。
func TestStopReasonMapping(t *testing.T) {
	cases := map[string]string{
		"end_turn":                      "end_turn",
		"max_tokens":                    "max_tokens",
		"stop_sequence":                 "stop_sequence",
		"tool_use":                      "tool_use",
		"model_context_window_exceeded": "model_context_window_exceeded",
		"pause_turn":                    "end_turn",         // 无对应 → end_turn
		"refusal":                       "content_filtered", // 无对应 → content_filtered
		"something_new":                 "end_turn",         // 未知 → 兜底 end_turn
	}
	for in, want := range cases {
		if got := mapStopReason(in); got != want {
			t.Errorf("mapStopReason(%q)=%q, want %q", in, got, want)
		}
	}
}

// TestUnknownBlockDowngrade 验证未知 content 块降级为可观测 text 占位。
func TestUnknownBlockDowngrade(t *testing.T) {
	native := []byte(`{"type":"message","role":"assistant","content":[
		{"type":"some_future_block","foo":"bar"}
	],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	got := AnthropicToConverseJSON(native)
	want := []byte(`{"output":{"message":{"role":"assistant","content":[
		{"text":"[unsupported block: some_future_block]"}
	]}},"stopReason":"end_turn",
	"usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2,"cacheReadInputTokens":0,"cacheWriteInputTokens":0}}`)
	jsonEqual(t, got, want)
}

// TestImageBlock 验证 image 块 media_type → format、data → bytes。
func TestImageBlock(t *testing.T) {
	native := []byte(`{"type":"message","role":"assistant","content":[
		{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAAB"}}
	],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	got := AnthropicToConverseJSON(native)
	want := []byte(`{"output":{"message":{"role":"assistant","content":[
		{"image":{"format":"png","source":{"bytes":"AAAB"}}}
	]}},"stopReason":"end_turn",
	"usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2,"cacheReadInputTokens":0,"cacheWriteInputTokens":0}}`)
	jsonEqual(t, got, want)
}

// TestNonMessagePassthrough 验证错误体 / 非 message 响应原样透传，不被改写。
func TestNonMessagePassthrough(t *testing.T) {
	errBody := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`)
	if got := AnthropicToConverseJSON(errBody); !reflect.DeepEqual(got, errBody) {
		t.Errorf("error body should pass through unchanged, got %s", got)
	}
	invalid := []byte(`not json`)
	if got := AnthropicToConverseJSON(invalid); !reflect.DeepEqual(got, invalid) {
		t.Errorf("invalid JSON should pass through unchanged, got %s", got)
	}
}
