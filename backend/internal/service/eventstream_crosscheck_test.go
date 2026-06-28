package service

import (
	"bytes"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude/conversecompat"
)

// TestConverseEncoderCompatibleWithBedrockDecoder 交叉验证：
// conversecompat 的 EventStream 编码器编出的帧，必须能被仓库现有的
// bedrockEventStreamDecoder（每天在解真实 AWS Bedrock 二进制帧）正确解出。
// 这是「编码器与 AWS 线格式字节一致」的强证据——无需额外 AWS 样本即可把关
// 帧布局 / 双 CRC / TLV header 编码 与真实解码器互逆。
func TestConverseEncoderCompatibleWithBedrockDecoder(t *testing.T) {
	payload := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`)
	// 现有 decoder 只放行 :event-type=="chunk"，故用 chunk 编码做兼容性对拍。
	frame := conversecompat.EncodeEventStreamFrame("chunk", payload)

	dec := newBedrockEventStreamDecoder(bytes.NewReader(frame))
	got, err := dec.Decode()
	if err != nil {
		t.Fatalf("现有 bedrockEventStreamDecoder 解码失败（encoder 与 AWS 线格式不兼容？）：%v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload 不一致：\n got=%s\nwant=%s", got, payload)
	}
}
