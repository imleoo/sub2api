package conversecompat

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// eventstream.go —— AWS EventStream 二进制帧编码（vnd.amazon.eventstream）。
//
// 帧布局（大端），与仓库现有解码器 bedrockEventStreamDecoder 互逆：
//
//	[total_len: 4][headers_len: 4][prelude_crc: 4]  ← prelude(12B)
//	[headers: headers_len]                          ← TLV，value_type=7(string)
//	[payload: total_len-12-headers_len-4]
//	[message_crc: 4]
//
//	prelude_crc  = CRC32_IEEE(bytes[0:8])
//	message_crc  = CRC32_IEEE(bytes[0:total_len-4])   // 含 prelude+headers+payload
//
// 金标准 = 现有 bedrockEventStreamDecoder（每天在解真实 AWS 二进制帧）的逆过程。
// 用 decodeFrameRaw(EncodeEventStreamFrame(x))==x 往返测试把关帧布局/CRC/header 编解码。

var eventStreamCRCTable = crc32.MakeTable(crc32.IEEE)

const eventStreamHeaderTypeString = 7 // TLV value_type：string（Bedrock header 常用）

type eventStreamHeader struct {
	name  string
	value string
}

// EncodeEventStreamFrame 把一条 Converse 事件（eventType + JSON payload）编为二进制帧。
// headers：:event-type / :content-type=application/json / :message-type=event。
func EncodeEventStreamFrame(eventType string, payload []byte) []byte {
	return assembleEventStreamFrame([]eventStreamHeader{
		{":event-type", eventType},
		{":content-type", "application/json"},
		{":message-type", "event"},
	}, payload)
}

// EncodeExceptionFrame 把中途错误编为 exception 帧（:message-type=exception）。
func EncodeExceptionFrame(exceptionType string, payload []byte) []byte {
	return assembleEventStreamFrame([]eventStreamHeader{
		{":exception-type", exceptionType},
		{":content-type", "application/json"},
		{":message-type", "exception"},
	}, payload)
}

func assembleEventStreamFrame(headers []eventStreamHeader, payload []byte) []byte {
	encHeaders := encodeEventStreamHeaders(headers)
	headersLen := len(encHeaders)
	totalLen := 12 + headersLen + len(payload) + 4

	buf := make([]byte, 0, totalLen)
	var prelude [8]byte
	binary.BigEndian.PutUint32(prelude[0:4], uint32(totalLen))
	binary.BigEndian.PutUint32(prelude[4:8], uint32(headersLen))
	buf = append(buf, prelude[:]...)
	buf = appendUint32(buf, crc32.Checksum(prelude[:], eventStreamCRCTable)) // prelude_crc = CRC32(bytes[0:8])
	buf = append(buf, encHeaders...)
	buf = append(buf, payload...)
	buf = appendUint32(buf, crc32.Checksum(buf, eventStreamCRCTable)) // message_crc = CRC32(bytes[0:total-4])
	return buf
}

// encodeEventStreamHeaders 按 TLV 编码 header 列表（仅 string 类型）。
// 单项：[name_len:1][name][value_type:1=7][value_len:2][value]
func encodeEventStreamHeaders(headers []eventStreamHeader) []byte {
	var buf []byte
	for _, h := range headers {
		buf = append(buf, byte(len(h.name)))
		buf = append(buf, h.name...)
		buf = append(buf, eventStreamHeaderTypeString)
		var l [2]byte
		binary.BigEndian.PutUint16(l[:], uint16(len(h.value)))
		buf = append(buf, l[:]...)
		buf = append(buf, h.value...)
	}
	return buf
}

func appendUint32(b []byte, v uint32) []byte {
	var x [4]byte
	binary.BigEndian.PutUint32(x[:], v)
	return append(b, x[:]...)
}

// decodeFrameRaw 解析单个帧 → (event-type, payload)。
// 纯帧解码：校验布局 + 双 CRC，但**不校验 :event-type 取值白名单**（区别于现有
// bedrockEventStreamDecoder 只认 "chunk"），故可对 Converse 多事件类型做往返验证。
func decodeFrameRaw(frame []byte) (eventType string, payload []byte, err error) {
	if len(frame) < 16 {
		return "", nil, fmt.Errorf("eventstream frame too short: %d", len(frame))
	}
	totalLen := binary.BigEndian.Uint32(frame[0:4])
	headersLen := binary.BigEndian.Uint32(frame[4:8])
	preludeCRC := binary.BigEndian.Uint32(frame[8:12])
	if crc32.Checksum(frame[0:8], eventStreamCRCTable) != preludeCRC {
		return "", nil, fmt.Errorf("eventstream prelude CRC mismatch")
	}
	if int(totalLen) != len(frame) {
		return "", nil, fmt.Errorf("eventstream total_len=%d != frame len=%d", totalLen, len(frame))
	}
	if 12+int(headersLen)+4 > int(totalLen) {
		return "", nil, fmt.Errorf("eventstream headers_len=%d out of range", headersLen)
	}
	msgCRC := binary.BigEndian.Uint32(frame[totalLen-4:])
	if crc32.Checksum(frame[0:totalLen-4], eventStreamCRCTable) != msgCRC {
		return "", nil, fmt.Errorf("eventstream message CRC mismatch")
	}
	headers := frame[12 : 12+headersLen]
	payload = frame[12+headersLen : totalLen-4]
	eventType = extractEventStreamHeader(headers, ":event-type")
	return eventType, payload, nil
}

// extractEventStreamHeader 从 TLV headers 取指定 name 的 string 值（跳过其它类型）。
func extractEventStreamHeader(headers []byte, target string) string {
	pos := 0
	for pos < len(headers) {
		nameLen := int(headers[pos])
		pos++
		if pos+nameLen > len(headers) {
			break
		}
		name := string(headers[pos : pos+nameLen])
		pos += nameLen
		if pos >= len(headers) {
			break
		}
		valueType := headers[pos]
		pos++
		// 仅 string(7) 才有 [len:2][value]；其它类型不在我们的编码范围内，遇到即停止。
		if valueType != eventStreamHeaderTypeString {
			break
		}
		if pos+2 > len(headers) {
			break
		}
		valueLen := int(binary.BigEndian.Uint16(headers[pos : pos+2]))
		pos += 2
		if pos+valueLen > len(headers) {
			break
		}
		value := string(headers[pos : pos+valueLen])
		pos += valueLen
		if name == target {
			return value
		}
	}
	return ""
}
