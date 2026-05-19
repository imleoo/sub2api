// Package apicompat: SSE replay framework (Phase 3 P3-2).
//
// 用于桥层的流式回归测试：从录制的 fixture（backend/testdata/sse/<protocol>/<name>.sse）
// 重放真实上游 SSE 字节流，断言桥转换后写到 client 的事件序列正确。
//
// 设计目标（docs/relay-architecture-design.md §11 流式 SSE 回归测试集）：
//   - 测试不走真实网络（fixture 来自 script/record_upstream_sse.sh 离线录制）
//   - 验证 reader/writer 流式不整体 buffer
//   - 断言事件序列与同协议 native 录制完全等价
//   - 支持 chunk 边界扰动（验证 SSE 解析器的鲁棒性）
package apicompat

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// SSEEvent 表示一条解析后的 SSE 事件（Phase 3 P3-2）。
//
// 字段语义遵循 https://html.spec.whatwg.org/multipage/server-sent-events.html
// 但只关注桥层需要断言的核心字段：
//   - Event: "event:" 行的值，缺省时为 "message"
//   - Data: "data:" 行的值（多行 data 已按 \n 拼接）
//   - Comments: ":" 开头的注释行（部分上游用 ":keep-alive" 维持连接，需保留以验证不丢失）
//   - Raw: 原始字节（含 trailing \n\n），便于断言写入字节数
type SSEEvent struct {
	Event    string
	Data     string
	Comments []string
	Raw      []byte
}

// ParseSSEStream 按行解析一个 SSE 字节流，返回有序的事件列表。
//
// 解析规则：
//   - 空行触发 dispatch（一个事件结束）
//   - "data:" / "event:" 字段名后允许零或一个空格
//   - 多行 data 按 "\n" 拼接（spec 标准）
//   - ":" 开头作为 comment 收集
//   - 不支持 retry / id 字段（桥层不需要）
//
// 用于断言：rebuilt := ParseSSEStream(client_output); golden := ParseSSEStream(fixture)
func ParseSSEStream(r io.Reader) ([]SSEEvent, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	var (
		events  []SSEEvent
		current SSEEvent
		raw     bytes.Buffer
		dataBuf bytes.Buffer
		started bool
	)

	dispatch := func() {
		if !started {
			return
		}
		current.Data = dataBuf.String()
		current.Raw = append([]byte(nil), raw.Bytes()...)
		events = append(events, current)
		current = SSEEvent{}
		dataBuf.Reset()
		raw.Reset()
		started = false
	}

	for scanner.Scan() {
		line := scanner.Text()
		raw.WriteString(line)
		raw.WriteByte('\n')

		// 空行 → dispatch
		if line == "" {
			dispatch()
			continue
		}

		started = true
		// 注释行
		if strings.HasPrefix(line, ":") {
			current.Comments = append(current.Comments, strings.TrimPrefix(line, ":"))
			continue
		}

		// field:value 形式
		var field, value string
		if idx := strings.IndexByte(line, ':'); idx >= 0 {
			field = line[:idx]
			value = strings.TrimPrefix(line[idx+1:], " ")
		} else {
			field = line
		}
		switch field {
		case "event":
			current.Event = value
		case "data":
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(value)
		default:
			// 忽略未知字段（id / retry / 其他）
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan SSE: %w", err)
	}
	dispatch()
	return events, nil
}

// SSEEventSequence 是用于断言的事件序列简化形态：
// 只保留 Event + Data 两个字段，忽略 Comments / Raw（适合 require.Equal）。
type SSEEventSequence []struct {
	Event string
	Data  string
}

// ToSequence 把 []SSEEvent 转成可读断言形式。
func ToSequence(events []SSEEvent) SSEEventSequence {
	out := make(SSEEventSequence, 0, len(events))
	for _, e := range events {
		out = append(out, struct {
			Event string
			Data  string
		}{Event: e.Event, Data: e.Data})
	}
	return out
}

// ReplaySSEInChunks 按 chunkSize 字节切片把 fixture bytes 推到 dst（模拟网络 chunk 抖动）。
//
// 用于验证桥层的 reader/writer 流式实现：即便上游 chunk 落在事件中间也不丢字节。
// chunkSize=0 时全部一次性写入（等价于 io.Copy）。
func ReplaySSEInChunks(dst io.Writer, fixture []byte, chunkSize int) (int, error) {
	if chunkSize <= 0 {
		return dst.Write(fixture)
	}
	total := 0
	for offset := 0; offset < len(fixture); offset += chunkSize {
		end := offset + chunkSize
		if end > len(fixture) {
			end = len(fixture)
		}
		n, err := dst.Write(fixture[offset:end])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
