package conversecompat

import (
	"encoding/binary"
	"testing"
)

// TestEventStreamRoundtrip 验证 encode→decodeFrameRaw 对各 Converse 事件类型自洽
// （帧布局 + 双 CRC + TLV header 编解码）。
func TestEventStreamRoundtrip(t *testing.T) {
	cases := []struct {
		eventType string
		payload   string
	}{
		{"messageStart", `{"role":"assistant"}`},
		{"contentBlockStart", `{"contentBlockIndex":0,"start":{"toolUse":{"toolUseId":"toolu_bdrk_x","name":"f"}}}`},
		{"contentBlockDelta", `{"contentBlockIndex":0,"delta":{"text":"hello"}}`},
		{"contentBlockStop", `{"contentBlockIndex":0}`},
		{"messageStop", `{"stopReason":"end_turn"}`},
		{"metadata", `{"usage":{"inputTokens":13,"outputTokens":12,"totalTokens":25},"metrics":{"latencyMs":0}}`},
		{"empty", ``},
	}
	for _, tc := range cases {
		frame := EncodeEventStreamFrame(tc.eventType, []byte(tc.payload))
		et, payload, err := decodeFrameRaw(frame)
		if err != nil {
			t.Fatalf("%s: decodeFrameRaw: %v", tc.eventType, err)
		}
		if et != tc.eventType {
			t.Errorf("%s: eventType got %q want %q", tc.eventType, et, tc.eventType)
		}
		if string(payload) != tc.payload {
			t.Errorf("%s: payload got %q want %q", tc.eventType, payload, tc.payload)
		}
	}
}

// TestEventStreamFrameSelfLength 验证 total_len 字段 == 实际帧长度（自描述）。
func TestEventStreamFrameSelfLength(t *testing.T) {
	frame := EncodeEventStreamFrame("metadata", []byte(`{"usage":{"totalTokens":3}}`))
	totalLen := binary.BigEndian.Uint32(frame[0:4])
	if int(totalLen) != len(frame) {
		t.Errorf("total_len=%d != frame len=%d", totalLen, len(frame))
	}
}

// TestEventStreamCorruptedRejected 验证翻转任一字节都会被 CRC 拒绝。
func TestEventStreamCorruptedRejected(t *testing.T) {
	frame := EncodeEventStreamFrame("messageStart", []byte(`{"role":"assistant"}`))
	corrupted := make([]byte, len(frame))
	copy(corrupted, frame)
	corrupted[len(corrupted)/2] ^= 0xFF
	if _, _, err := decodeFrameRaw(corrupted); err == nil {
		t.Error("corrupted frame should fail CRC check")
	}
}

// TestEventStreamExceptionFrame 验证 exception 帧 payload 往返（无 :event-type）。
func TestEventStreamExceptionFrame(t *testing.T) {
	frame := EncodeExceptionFrame("internalServerException", []byte(`{"message":"boom"}`))
	et, payload, err := decodeFrameRaw(frame)
	if err != nil {
		t.Fatalf("decodeFrameRaw: %v", err)
	}
	if et != "" {
		t.Errorf("exception frame should have no :event-type, got %q", et)
	}
	if string(payload) != `{"message":"boom"}` {
		t.Errorf("payload got %q", payload)
	}
	if mt := extractEventStreamHeader(frameHeaders(frame), ":message-type"); mt != "exception" {
		t.Errorf(":message-type got %q want exception", mt)
	}
}

// frameHeaders 取出帧的 headers 段（测试辅助）。
func frameHeaders(frame []byte) []byte {
	headersLen := binary.BigEndian.Uint32(frame[4:8])
	return frame[12 : 12+headersLen]
}
