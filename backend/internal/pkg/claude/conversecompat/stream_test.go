package conversecompat

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// sseEvent 是从 .sse fixture 解析出的一条 native 事件。
type sseEvent struct {
	event string
	data  string
}

// parseSSEFixture 解析 .sse 文件为事件序列（按空行分隔，取 event:/data: 行）。
func parseSSEFixture(t *testing.T, name string) []sseEvent {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer func() { _ = f.Close() }()

	var events []sseEvent
	var cur sseEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			cur.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			cur.data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case strings.TrimSpace(line) == "":
			if cur.event != "" || cur.data != "" {
				events = append(events, cur)
				cur = sseEvent{}
			}
		}
	}
	if cur.event != "" || cur.data != "" {
		events = append(events, cur)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", name, err)
	}
	return events
}

// convertFixture 把 fixture 全流转换为 Converse 帧序列，再解码还原为 (eventType, payload)。
func convertFixture(t *testing.T, name string) []struct{ et, payload string } {
	t.Helper()
	sc := NewStreamConverter()
	var out []struct{ et, payload string }
	for _, ev := range parseSSEFixture(t, name) {
		frame := sc.Convert(ev.event, []byte(ev.data))
		if frame == nil {
			continue
		}
		et, payload, err := decodeFrameRaw(frame)
		if err != nil {
			t.Fatalf("%s: decode frame for %q: %v", name, ev.event, err)
		}
		out = append(out, struct{ et, payload string }{et, string(payload)})
	}
	// 末尾 metadata 帧
	mf := sc.Finish()
	et, payload, err := decodeFrameRaw(mf)
	if err != nil {
		t.Fatalf("%s: decode metadata frame: %v", name, err)
	}
	out = append(out, struct{ et, payload string }{et, string(payload)})
	return out
}

// TestStreamToolUseConverse 用真实 tool_use 流式 fixture 对拍 Converse 事件序列。
func TestStreamToolUseConverse(t *testing.T) {
	got := convertFixture(t, "native_stream_tooluse.sse")

	wantSeq := []string{
		"messageStart",
		"contentBlockStart",
		"contentBlockDelta", "contentBlockDelta", "contentBlockDelta", "contentBlockDelta",
		"contentBlockStop",
		"messageStop",
		"metadata",
	}
	if len(got) != len(wantSeq) {
		t.Fatalf("事件数不符：got %d want %d\n%v", len(got), len(wantSeq), got)
	}
	for i, et := range wantSeq {
		if got[i].et != et {
			t.Errorf("事件[%d] got %q want %q", i, got[i].et, et)
		}
	}

	// contentBlockStart 保留 toolUseId（含 toolu_bdrk_ 前缀）+ name
	start := got[1].payload
	if id := gjson.Get(start, "start.toolUse.toolUseId").String(); id != "toolu_bdrk_01Cri1LR2wE1UHevAmguSFni" {
		t.Errorf("toolUseId got %q", id)
	}
	if name := gjson.Get(start, "start.toolUse.name").String(); name != "get_weather" {
		t.Errorf("tool name got %q", name)
	}
	// contentBlockDelta 携带 toolUse.input 分片（拼接后为 {"city": "Paris"}）
	joined := ""
	for i := 2; i <= 5; i++ {
		joined += gjson.Get(got[i].payload, "delta.toolUse.input").String()
	}
	if joined != `{"city": "Paris"}` {
		t.Errorf("拼接 input got %q", joined)
	}
	// messageStop.stopReason
	if sr := gjson.Get(got[7].payload, "stopReason").String(); sr != "tool_use" {
		t.Errorf("stopReason got %q", sr)
	}
	// metadata.usage：input=657 output=38 total=695
	meta := got[8].payload
	if in := gjson.Get(meta, "usage.inputTokens").Int(); in != 657 {
		t.Errorf("inputTokens got %d", in)
	}
	if out := gjson.Get(meta, "usage.outputTokens").Int(); out != 38 {
		t.Errorf("outputTokens got %d", out)
	}
	if tot := gjson.Get(meta, "usage.totalTokens").Int(); tot != 695 {
		t.Errorf("totalTokens got %d", tot)
	}
}

// TestStreamTextConverse 用真实纯文本流式 fixture 验证 text_delta→delta.text。
func TestStreamTextConverse(t *testing.T) {
	got := convertFixture(t, "native_stream_text.sse")
	if len(got) < 3 {
		t.Fatalf("事件过少：%v", got)
	}
	if got[0].et != "messageStart" {
		t.Errorf("首事件 got %q want messageStart", got[0].et)
	}
	if got[len(got)-1].et != "metadata" {
		t.Errorf("末事件 got %q want metadata", got[len(got)-1].et)
	}
	// 至少有一条 contentBlockDelta 带 text
	foundText := false
	for _, e := range got {
		if e.et == "contentBlockDelta" && gjson.Get(e.payload, "delta.text").Exists() {
			foundText = true
			break
		}
	}
	if !foundText {
		t.Errorf("未找到带 delta.text 的 contentBlockDelta：%v", got)
	}
}
