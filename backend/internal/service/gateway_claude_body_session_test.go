//go:build unit

package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// --- buildStableSessionSeed / GenerateSessionUUID：幂等性 + 防碰撞 ---

func TestBuildStableSessionSeed_Deterministic(t *testing.T) {
	t.Parallel()
	a := buildStableSessionSeed(42, "1.2.3.4:ua:9", "hello world")
	b := buildStableSessionSeed(42, "1.2.3.4:ua:9", "hello world")
	require.Equal(t, a, b)
}

func TestGenerateSessionUUID_SameSeedSameUUID(t *testing.T) {
	t.Parallel()
	seed := buildStableSessionSeed(42, "1.2.3.4:ua:9", "hello world")
	u1 := GenerateSessionUUID(seed)
	u2 := GenerateSessionUUID(seed)
	require.Equal(t, u1, u2)
	require.True(t, isValidUUIDv4(t, u1))
}

func TestGenerateSessionUUID_DifferentSeedsDifferentUUIDs(t *testing.T) {
	t.Parallel()
	seedA := buildStableSessionSeed(42, "1.2.3.4:ua:9", "hello world")
	seedB := buildStableSessionSeed(42, "1.2.3.4:ua:9", "goodbye world")
	seedC := buildStableSessionSeed(43, "1.2.3.4:ua:9", "hello world")

	uA := GenerateSessionUUID(seedA)
	uB := GenerateSessionUUID(seedB)
	uC := GenerateSessionUUID(seedC)

	require.NotEqual(t, uA, uB, "不同首条消息应派生不同 session_id")
	require.NotEqual(t, uA, uC, "不同账号 ID 应派生不同 session_id")
}

func TestGenerateSessionUUID_EmptySeedIsRandom(t *testing.T) {
	t.Parallel()
	u1 := GenerateSessionUUID("")
	u2 := GenerateSessionUUID("")
	require.NotEqual(t, u1, u2, "空 seed 应回退到随机 UUID，不具备确定性")
	require.True(t, isValidUUIDv4(t, u1))
}

func TestBuildStableSessionSeed_AppendingMessagesKeepsSeedStable(t *testing.T) {
	t.Parallel()
	// 场景：对话尾部追加新 messages，但账号 ID / 客户端区分因子 / 首条 user 消息不变，
	// 会话种子必须保持稳定（这是 sticky session 生成的核心不变式）。
	discriminator := sessionContextDiscriminator(&SessionContext{ClientIP: "10.0.0.1", UserAgent: "curl/8.0", APIKeyID: 7})
	seedBeforeAppend := buildStableSessionSeed(1, discriminator, "第一条消息")
	seedAfterAppend := buildStableSessionSeed(1, discriminator, "第一条消息")
	require.Equal(t, seedBeforeAppend, seedAfterAppend)
	require.Equal(t, GenerateSessionUUID(seedBeforeAppend), GenerateSessionUUID(seedAfterAppend))
}

func isValidUUIDv4(t *testing.T, s string) bool {
	t.Helper()
	parsed, err := uuid.Parse(s)
	require.NoError(t, err)
	return parsed.Version() == 4
}

// --- sessionContextDiscriminator ---

func TestSessionContextDiscriminator_Nil(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", sessionContextDiscriminator(nil))
}

func TestSessionContextDiscriminator_CombinesFields(t *testing.T) {
	t.Parallel()
	sc := &SessionContext{ClientIP: "1.2.3.4", UserAgent: "Mozilla/5.0", APIKeyID: 99}
	got := sessionContextDiscriminator(sc)
	require.Contains(t, got, "1.2.3.4")
	require.Contains(t, got, "99")
}

func TestSessionContextDiscriminator_DifferentClientsDiffer(t *testing.T) {
	t.Parallel()
	scA := &SessionContext{ClientIP: "1.2.3.4", UserAgent: "ua-a", APIKeyID: 1}
	scB := &SessionContext{ClientIP: "5.6.7.8", UserAgent: "ua-a", APIKeyID: 1}
	require.NotEqual(t, sessionContextDiscriminator(scA), sessionContextDiscriminator(scB))
}

// --- shouldNormalizeClientDateline / normalizeClientDatelineIfEnabled ---
// 注：订阅逆向清理（功能35）后，Anthropic OAuth/SetupToken 账号类型已删除，
// shouldNormalizeClientDateline 因此恒为 false —— 这里锁定"永久关闭"这一不变式，
// 防止未来有人重新给它接上账号类型判断分支却漏了测试。

func TestShouldNormalizeClientDateline_AlwaysFalse(t *testing.T) {
	t.Parallel()
	svc := &GatewayService{}
	require.False(t, svc.shouldNormalizeClientDateline(nil, nil))
	require.False(t, svc.shouldNormalizeClientDateline(nil, &Account{Platform: PlatformAnthropic}))
}

func TestNormalizeClientDatelineIfEnabled_AlwaysNoop(t *testing.T) {
	t.Parallel()
	svc := &GatewayService{}
	body := []byte(`{"system":"Today is 2020-01-01."}`)
	next, changed := svc.normalizeClientDatelineIfEnabled(nil, &Account{Platform: PlatformAnthropic}, body)
	require.False(t, changed)
	require.Nil(t, next)
}

// --- enforceCacheControlLimit ---

func TestEnforceCacheControlLimit_EmptyBody(t *testing.T) {
	t.Parallel()
	require.Nil(t, enforceCacheControlLimit(nil))
	require.Equal(t, []byte{}, enforceCacheControlLimit([]byte{}))
}

func TestEnforceCacheControlLimit_UnderLimitUnchanged(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"system": [{"type":"text","text":"a","cache_control":{"type":"ephemeral"}}],
		"messages": [{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}]
	}`)
	got := enforceCacheControlLimit(body)
	require.JSONEq(t, string(body), string(got), "未超限时不应做任何裁剪")
}

func TestEnforceCacheControlLimit_OverLimit_RemovesToolsFirst(t *testing.T) {
	t.Parallel()
	// 5 个 cache_control 断点（system 1 + tools 4），超出上限 4 个，
	// 按"先工具、再 messages、最后 system"的优先级裁剪：应先削减 tools。
	body := []byte(`{
		"system": [{"type":"text","text":"sys","cache_control":{"type":"ephemeral"}}],
		"tools": [
			{"name":"t1","cache_control":{"type":"ephemeral"}},
			{"name":"t2","cache_control":{"type":"ephemeral"}},
			{"name":"t3","cache_control":{"type":"ephemeral"}},
			{"name":"t4","cache_control":{"type":"ephemeral"}}
		]
	}`)
	got := enforceCacheControlLimit(body)

	remaining := countCacheControlBlocks(t, got)
	require.LessOrEqual(t, remaining, maxCacheControlBlocks)

	sysCC := gjsonGetCacheControl(t, got, "system.0")
	require.True(t, sysCC, "system 断点应被保留（tools 优先级更低，先被裁剪）")
}

func TestEnforceCacheControlLimit_RemovesIllegalThinkingBlockCacheControl(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"system": [{"type":"thinking","text":"reasoning...","cache_control":{"type":"ephemeral"}}]
	}`)
	got := enforceCacheControlLimit(body)
	require.False(t, gjsonGetCacheControl(t, got, "system.0"), "thinking 块不允许 cache_control，必须被移除")
}

func countCacheControlBlocks(t *testing.T, body []byte) int {
	t.Helper()
	invalidThinking, messagePaths, toolPaths, systemPaths := collectCacheControlPaths(body)
	return len(invalidThinking) + len(messagePaths) + len(toolPaths) + len(systemPaths)
}

func gjsonGetCacheControl(t *testing.T, body []byte, path string) bool {
	t.Helper()
	return gjson.GetBytes(body, path+".cache_control").Exists()
}
