//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ── endpoint health 选择（chaos 场景） ────────────────────────────────────

// TestSelectHealthyEndpoints_FiltersDisabledAndDegraded
// disabled/degraded endpoint 被排除，healthy 按 priority ASC 返回。
func TestSelectHealthyEndpoints_FiltersDisabledAndDegraded(t *testing.T) {
	endpoints := []*DBEndpoint{
		{ID: 1, StableID: "ep-1", Health: EndpointHealthHealthy, Priority: 10},
		{ID: 2, StableID: "ep-2", Health: EndpointHealthDisabled, Priority: 5},
		{ID: 3, StableID: "ep-3", Health: EndpointHealthDegraded, Priority: 1},
		{ID: 4, StableID: "ep-4", Health: EndpointHealthHealthy, Priority: 20},
	}

	got := SelectHealthyEndpoints(endpoints)

	require.Len(t, got, 2, "只应返回 2 个 healthy endpoint")
	require.Equal(t, "ep-1", got[0].StableID, "priority=10 应先于 priority=20")
	require.Equal(t, "ep-4", got[1].StableID)
}

// TestSelectHealthyEndpoints_AllDisabled 全部不可用时返回空切片，不 panic。
func TestSelectHealthyEndpoints_AllDisabled_ReturnsEmpty(t *testing.T) {
	endpoints := []*DBEndpoint{
		{ID: 1, Health: EndpointHealthDisabled},
		{ID: 2, Health: EndpointHealthDegraded},
	}
	require.Empty(t, SelectHealthyEndpoints(endpoints))
}

// TestSelectHealthyEndpoints_NilSafe nil 入参不 panic。
func TestSelectHealthyEndpoints_NilSafe(t *testing.T) {
	require.NotPanics(t, func() {
		require.Empty(t, SelectHealthyEndpoints(nil))
	})
}

// TestSelectHealthyEndpoints_TieBreakByID priority 相同时按 ID 升序。
func TestSelectHealthyEndpoints_TieBreakByID(t *testing.T) {
	endpoints := []*DBEndpoint{
		{ID: 9, StableID: "ep-9", Health: EndpointHealthHealthy, Priority: 100},
		{ID: 3, StableID: "ep-3", Health: EndpointHealthHealthy, Priority: 100},
	}
	got := SelectHealthyEndpoints(endpoints)
	require.Len(t, got, 2)
	require.Equal(t, "ep-3", got[0].StableID, "ID 小的优先")
}

// ── chaos: endpoint 失败 → session 重绑定（无客户端可见错误） ────────────

// TestChaosEndpointFailover_SessionRebound
// 模拟 ep-1 故障后，session 重绑到 ep-2，无返回错误。
func TestChaosEndpointFailover_SessionRebound(t *testing.T) {
	ctx := context.Background()
	cache := newInMemoryEndpointCache()
	svc := &GatewayService{cache: cache}
	gid := chaosGroupIDPtr(1)
	const sess = "chaos_sess"

	// 初始绑定 ep-1
	require.NoError(t, svc.BindStickySessionWithEndpoint(ctx, gid, sess, 100, "ep-1"))
	_, bound, _ := svc.GetCachedSessionBinding(ctx, gid, sess)
	require.Equal(t, "ep-1", bound)

	// chaos: ep-1 变为 disabled，系统删除 session 绑定
	svc.deleteSessionBinding(ctx, 1, sess)
	gotAcc, gotEP, _ := svc.GetCachedSessionBinding(ctx, gid, sess)
	require.Zero(t, gotAcc, "session 已清除")
	require.Empty(t, gotEP)

	// 重新从可用 endpoint 中选择 ep-2（ep-1 已不健康）
	available := []*DBEndpoint{
		{ID: 1, StableID: "ep-1", Health: EndpointHealthDisabled, Priority: 10},
		{ID: 2, StableID: "ep-2", Health: EndpointHealthHealthy, Priority: 20},
	}
	healthy := SelectHealthyEndpoints(available)
	require.NotEmpty(t, healthy, "chaos 后仍有可用 endpoint，不应产生客户端可见错误")

	// 重绑到 ep-2
	require.NoError(t, svc.BindStickySessionWithEndpoint(ctx, gid, sess, 100, healthy[0].StableID))
	_, newBound, _ := svc.GetCachedSessionBinding(ctx, gid, sess)
	require.Equal(t, "ep-2", newBound)
}

// TestChaosEndpointFailover_AllEndpointsFailed 所有 endpoint 失败时
// SelectHealthyEndpoints 返回空，调用方据此判断账号不可调度，不 panic。
func TestChaosEndpointFailover_AllEndpointsFailed(t *testing.T) {
	available := []*DBEndpoint{
		{ID: 1, StableID: "ep-1", Health: EndpointHealthDisabled, Priority: 10},
		{ID: 2, StableID: "ep-2", Health: EndpointHealthDegraded, Priority: 20},
	}
	healthy := SelectHealthyEndpoints(available)
	require.Empty(t, healthy, "全部 endpoint 故障，应为空（调用方按账号不可调度处理）")
}

// ── dual-bucket 计数 ──────────────────────────────────────────────────────

// dualBucketCacheStub 实现 SchedulerCache 中的 dual-bucket 计数方法。
type dualBucketCacheStub struct {
	noopSchedulerCache // 其余方法 no-op
	totals             map[string]int64
	diverged           map[string]int64
}

func newDualBucketCacheStub() *dualBucketCacheStub {
	return &dualBucketCacheStub{
		totals:   make(map[string]int64),
		diverged: make(map[string]int64),
	}
}

func (c *dualBucketCacheStub) IncrDualBucketTotal(_ context.Context, platform string) (int64, error) {
	c.totals[platform]++
	return c.totals[platform], nil
}

func (c *dualBucketCacheStub) IncrDualBucketDiverged(_ context.Context, platform string) (int64, error) {
	c.diverged[platform]++
	return c.diverged[platform], nil
}

func (c *dualBucketCacheStub) GetDualBucketStats(_ context.Context, platform string, _ int) ([]DualBucketDayStats, error) {
	date := time.Now().UTC().Format("2006-01-02")
	return []DualBucketDayStats{{
		Date:     date,
		Total:    c.totals[platform],
		Diverged: c.diverged[platform],
	}}, nil
}

// noopSchedulerCache 满足 SchedulerCache 接口，所有方法 no-op。
type noopSchedulerCache struct{}

func (noopSchedulerCache) GetSnapshot(_ context.Context, _ SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}
func (noopSchedulerCache) SetSnapshot(_ context.Context, _ SchedulerBucket, _ []Account) error {
	return nil
}
func (noopSchedulerCache) GetAccount(_ context.Context, _ int64) (*Account, error)       { return nil, nil }
func (noopSchedulerCache) SetAccount(_ context.Context, _ *Account) error                { return nil }
func (noopSchedulerCache) DeleteAccount(_ context.Context, _ int64) error                { return nil }
func (noopSchedulerCache) UpdateLastUsed(_ context.Context, _ map[int64]time.Time) error { return nil }
func (noopSchedulerCache) TryLockBucket(_ context.Context, _ SchedulerBucket, _ time.Duration) (bool, error) {
	return true, nil
}
func (noopSchedulerCache) UnlockBucket(_ context.Context, _ SchedulerBucket) error { return nil }
func (noopSchedulerCache) ListBuckets(_ context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}
func (noopSchedulerCache) GetOutboxWatermark(_ context.Context) (int64, error) { return 0, nil }
func (noopSchedulerCache) SetOutboxWatermark(_ context.Context, _ int64) error { return nil }
func (noopSchedulerCache) IncrDualBucketTotal(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (noopSchedulerCache) IncrDualBucketDiverged(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (noopSchedulerCache) GetDualBucketStats(_ context.Context, _ string, _ int) ([]DualBucketDayStats, error) {
	return nil, nil
}

// dualBucketAccountRepoStub 实现 dual-bucket shadow 需要的三个查询方法。
type dualBucketAccountRepoStub struct {
	accountRepoStub // 继承基础 stub（不实现会 panic）
	protoAccounts   []Account
}

func (s *dualBucketAccountRepoStub) ListSchedulableByGroupIDAndOutboundProtocol(
	_ context.Context, _ int64, _ string,
) ([]Account, error) {
	return s.protoAccounts, nil
}

func (s *dualBucketAccountRepoStub) ListSchedulableByOutboundProtocol(
	_ context.Context, _ string,
) ([]Account, error) {
	return s.protoAccounts, nil
}

func (s *dualBucketAccountRepoStub) ListSchedulableUngroupedByOutboundProtocol(
	_ context.Context, _ string,
) ([]Account, error) {
	return s.protoAccounts, nil
}

// TestDualBucketShadow_IncrementsTotal 一致时 total+1，diverged 不变。
func TestDualBucketShadow_IncrementsTotal_WhenConsistent(t *testing.T) {
	cache := newDualBucketCacheStub()
	platformAccounts := []Account{{ID: 1}, {ID: 2}}
	// 协议桶返回相同账号集
	stub := &dualBucketAccountRepoStub{protoAccounts: platformAccounts}
	svc := &SchedulerSnapshotService{cache: cache, accountRepo: stub}

	svc.runDualBucketShadow(PlatformAnthropic, 0, platformAccounts)
	time.Sleep(15 * time.Millisecond)

	require.Equal(t, int64(1), cache.totals[PlatformAnthropic])
	require.Equal(t, int64(0), cache.diverged[PlatformAnthropic])
}

// TestDualBucketShadow_IncrementsDiverged 差异时 diverged+1。
func TestDualBucketShadow_IncrementsDiverged_WhenDifferent(t *testing.T) {
	cache := newDualBucketCacheStub()
	stub := &dualBucketAccountRepoStub{protoAccounts: []Account{{ID: 99}}}
	svc := &SchedulerSnapshotService{cache: cache, accountRepo: stub}

	// platform 桶 {1,2}，协议桶 {99}
	svc.runDualBucketShadow(PlatformAnthropic, 0, []Account{{ID: 1}, {ID: 2}})
	time.Sleep(15 * time.Millisecond)

	require.Equal(t, int64(1), cache.totals[PlatformAnthropic])
	require.Equal(t, int64(1), cache.diverged[PlatformAnthropic])
}

// TestDualBucketShadow_SkipsLingjingAndGeneric 不参与双桶的平台不计数。
func TestDualBucketShadow_SkipsLingjingAndGeneric(t *testing.T) {
	cache := newDualBucketCacheStub()
	stub := &dualBucketAccountRepoStub{}
	svc := &SchedulerSnapshotService{cache: cache, accountRepo: stub}

	svc.runDualBucketShadow(PlatformLingjing, 0, []Account{{ID: 1}})
	svc.runDualBucketShadow(PlatformGeneric, 0, []Account{{ID: 1}})
	time.Sleep(15 * time.Millisecond)

	require.Zero(t, cache.totals[PlatformLingjing])
	require.Zero(t, cache.totals[PlatformGeneric])
}

func chaosGroupIDPtr(v int64) *int64 { return &v }
