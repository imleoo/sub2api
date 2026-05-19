package apicompat

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase 3 P3-2 SSE replay framework 单测。

const anthropicFixture = `event: message_start
data: {"type":"message_start","message":{"id":"msg_1"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}

event: message_stop
data: {"type":"message_stop"}

`

const multilineDataFixture = `event: chunk
data: line1
data: line2

`

const commentFixture = `: keep-alive

event: ping
data: ok

`

func TestParseSSEStream_Anthropic(t *testing.T) {
	events, err := ParseSSEStream(strings.NewReader(anthropicFixture))
	require.NoError(t, err)
	require.Len(t, events, 4)

	require.Equal(t, "message_start", events[0].Event)
	require.Contains(t, events[0].Data, `"type":"message_start"`)

	require.Equal(t, "content_block_delta", events[1].Event)
	require.Contains(t, events[1].Data, "Hello")

	require.Equal(t, "content_block_delta", events[2].Event)
	require.Contains(t, events[2].Data, "world")

	require.Equal(t, "message_stop", events[3].Event)
}

func TestParseSSEStream_MultilineData(t *testing.T) {
	events, err := ParseSSEStream(strings.NewReader(multilineDataFixture))
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "chunk", events[0].Event)
	require.Equal(t, "line1\nline2", events[0].Data, "多行 data 按 \\n 拼接（SSE spec）")
}

func TestParseSSEStream_Comments(t *testing.T) {
	events, err := ParseSSEStream(strings.NewReader(commentFixture))
	require.NoError(t, err)

	// 第一个事件只是注释，dispatch 后 Event/Data 为空
	require.Len(t, events, 2)
	require.Empty(t, events[0].Event)
	require.Empty(t, events[0].Data)
	require.Equal(t, []string{" keep-alive"}, events[0].Comments)

	// 第二个是正常事件
	require.Equal(t, "ping", events[1].Event)
	require.Equal(t, "ok", events[1].Data)
}

func TestToSequence_StripsCommentsAndRaw(t *testing.T) {
	events, _ := ParseSSEStream(strings.NewReader(anthropicFixture))
	seq := ToSequence(events)
	require.Len(t, seq, 4)
	require.Equal(t, "message_start", seq[0].Event)
	require.Equal(t, "message_stop", seq[3].Event)
}

func TestReplaySSEInChunks_PreservesByteOrder(t *testing.T) {
	var dst bytes.Buffer
	n, err := ReplaySSEInChunks(&dst, []byte(anthropicFixture), 7)
	require.NoError(t, err)
	require.Equal(t, len(anthropicFixture), n)
	require.Equal(t, anthropicFixture, dst.String(), "chunk 切分不应改变字节序")
}

func TestReplaySSEInChunks_ZeroChunkSizeFallsBackToWholeWrite(t *testing.T) {
	var dst bytes.Buffer
	n, err := ReplaySSEInChunks(&dst, []byte(anthropicFixture), 0)
	require.NoError(t, err)
	require.Equal(t, len(anthropicFixture), n)
}

func TestReplaySSE_ParseRoundtrip(t *testing.T) {
	var dst bytes.Buffer
	_, err := ReplaySSEInChunks(&dst, []byte(anthropicFixture), 13)
	require.NoError(t, err)

	// 重放后的字节流仍可被相同 parser 解析
	events, err := ParseSSEStream(&dst)
	require.NoError(t, err)
	require.Len(t, events, 4, "chunk 切分不影响事件边界识别")
}
