package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPlatformToProtocol(t *testing.T) {
	cases := []struct {
		platform string
		want     string
	}{
		{PlatformAnthropic, domain.ProtocolAnthropicMessages},
		{PlatformOpenAI, domain.ProtocolOpenAIChat},
		{PlatformGemini, domain.ProtocolGeminiV1Beta},
		{PlatformLingjing, ""},  // lingjing 不参与双桶
		{PlatformGeneric, ""},   // generic 通过 Endpoint 实体路由，不参与双桶
		{"unknown", ""},         // 未知 platform 不参与
	}
	for _, c := range cases {
		got := platformToProtocol(c.platform)
		require.Equal(t, c.want, got, "platform=%s", c.platform)
	}
}

func TestEqualInt64Slices(t *testing.T) {
	require.True(t, equalInt64Slices(nil, nil))
	require.True(t, equalInt64Slices([]int64{}, []int64{}))
	require.True(t, equalInt64Slices([]int64{1, 2, 3}, []int64{1, 2, 3}))
	require.False(t, equalInt64Slices([]int64{1, 2}, []int64{1, 3}))
	require.False(t, equalInt64Slices([]int64{1}, []int64{1, 2}))
}

func TestDiffIDSlices(t *testing.T) {
	added, removed := diffIDSlices([]int64{1, 2, 3}, []int64{2, 3, 4})
	require.Equal(t, []int64{4}, added)
	require.Equal(t, []int64{1}, removed)

	added, removed = diffIDSlices([]int64{1, 2}, []int64{1, 2})
	require.Empty(t, added)
	require.Empty(t, removed)
}

func TestExtractSortedIDs(t *testing.T) {
	accounts := []Account{{ID: 3}, {ID: 1}, {ID: 2}}
	ids := extractSortedIDs(accounts)
	require.Equal(t, []int64{1, 2, 3}, ids)
}
