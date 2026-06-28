package kirocompat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tidwall/gjson"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// TestNormalizeText 验证纯文本响应：移除 inference_geo，保留已有 stop_reason。
func TestNormalizeText(t *testing.T) {
	out := NormalizeNonStream(readFixture(t, "kiro_nonstream_text.json"))
	if gjson.GetBytes(out, "usage.inference_geo").Exists() {
		t.Errorf("usage.inference_geo 应被移除：%s", out)
	}
	if sr := gjson.GetBytes(out, "stop_reason").String(); sr != "end_turn" {
		t.Errorf("stop_reason got %q want end_turn", sr)
	}
	// 标准字段保留
	if gjson.GetBytes(out, "usage.input_tokens").Int() != 14 {
		t.Errorf("input_tokens 被破坏：%s", out)
	}
	if !gjson.GetBytes(out, "content.0.text").Exists() {
		t.Errorf("content 被破坏：%s", out)
	}
}

// TestNormalizeToolUse 验证 tool_use 响应补齐缺失的 stop_reason=tool_use。
func TestNormalizeToolUse(t *testing.T) {
	raw := readFixture(t, "kiro_nonstream_tooluse.json")
	// 前置：原样本确实缺 stop_reason
	if gjson.GetBytes(raw, "stop_reason").Exists() {
		t.Fatalf("fixture 前提变了：tool_use 样本不应带 stop_reason")
	}
	out := NormalizeNonStream(raw)
	if sr := gjson.GetBytes(out, "stop_reason").String(); sr != "tool_use" {
		t.Errorf("tool_use 响应 stop_reason got %q want tool_use", sr)
	}
	// tool_use 块 + id 保留
	if id := gjson.GetBytes(out, "content.0.id").String(); id == "" {
		t.Errorf("tool_use id 被破坏：%s", out)
	}
}

// TestNormalizeMissingStopReasonText 验证缺 stop_reason 且无 tool_use → end_turn。
func TestNormalizeMissingStopReasonText(t *testing.T) {
	body := []byte(`{"type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"output_tokens":1,"inference_geo":"global"}}`)
	out := NormalizeNonStream(body)
	if sr := gjson.GetBytes(out, "stop_reason").String(); sr != "end_turn" {
		t.Errorf("无 tool_use 缺 stop_reason → got %q want end_turn", sr)
	}
	if gjson.GetBytes(out, "usage.inference_geo").Exists() {
		t.Errorf("inference_geo 应被移除")
	}
}

// TestNormalizeNonMessagePassthrough 验证错误体 / 非 message 原样透传。
func TestNormalizeNonMessagePassthrough(t *testing.T) {
	errBody := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`)
	if out := NormalizeNonStream(errBody); string(out) != string(errBody) {
		t.Errorf("错误体应原样透传，got %s", out)
	}
	if out := NormalizeNonStream(nil); out != nil {
		t.Errorf("nil 应原样返回")
	}
}
