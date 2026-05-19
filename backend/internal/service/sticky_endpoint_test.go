//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// inMemoryEndpointCache 是纯内存 GatewayCache，用于测试 endpoint 维度粘性逻辑。
type inMemoryEndpointCache struct {
	accounts  map[string]int64
	endpoints map[string]string
}

func newInMemoryEndpointCache() *inMemoryEndpointCache {
	return &inMemoryEndpointCache{
		accounts:  make(map[string]int64),
		endpoints: make(map[string]string),
	}
}

func (c *inMemoryEndpointCache) GetSessionAccountID(_ context.Context, groupID int64, sessionHash string) (int64, error) {
	return c.accounts[sessionKey(groupID, sessionHash)], nil
}

func (c *inMemoryEndpointCache) SetSessionAccountID(_ context.Context, groupID int64, sessionHash string, accountID int64, _ time.Duration) error {
	c.accounts[sessionKey(groupID, sessionHash)] = accountID
	return nil
}

func (c *inMemoryEndpointCache) RefreshSessionTTL(_ context.Context, _ int64, _ string, _ time.Duration) error {
	return nil
}

func (c *inMemoryEndpointCache) DeleteSessionAccountID(_ context.Context, groupID int64, sessionHash string) error {
	delete(c.accounts, sessionKey(groupID, sessionHash))
	return nil
}

func (c *inMemoryEndpointCache) GetSessionEndpointStableID(_ context.Context, groupID int64, sessionHash string) (string, error) {
	return c.endpoints[endpointKey(groupID, sessionHash)], nil
}

func (c *inMemoryEndpointCache) SetSessionEndpointStableID(_ context.Context, groupID int64, sessionHash string, stableID string, _ time.Duration) error {
	c.endpoints[endpointKey(groupID, sessionHash)] = stableID
	return nil
}

func (c *inMemoryEndpointCache) DeleteSessionEndpointStableID(_ context.Context, groupID int64, sessionHash string) error {
	delete(c.endpoints, endpointKey(groupID, sessionHash))
	return nil
}

func sessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%d:%s", groupID, sessionHash)
}

func endpointKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("ep:%d:%s", groupID, sessionHash)
}

// TestBindStickySessionWithEndpoint_StoresAndRetrievesBothDimensions 验证 P5-3 核心：
// 同一 session 黏住同一 (account, endpoint) 1h。
func TestBindStickySessionWithEndpoint_StoresAndRetrievesBothDimensions(t *testing.T) {
	cache := newInMemoryEndpointCache()
	svc := &GatewayService{cache: cache}
	ctx := context.Background()
	groupID := stickyTestGroupIDPtr(42)
	const sessionHash = "sess_abc123"
	const accountID = int64(7)
	const endpointStableID = "ep-uuid-stable-001"

	err := svc.BindStickySessionWithEndpoint(ctx, groupID, sessionHash, accountID, endpointStableID)
	require.NoError(t, err)

	gotAccount, gotEndpoint, err := svc.GetCachedSessionBinding(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Equal(t, accountID, gotAccount)
	require.Equal(t, endpointStableID, gotEndpoint)
}

// TestGetCachedSessionBinding_LegacySession 旧会话（无 endpoint key）平滑降级：
// endpointStableID 返回 ""，不报错。
func TestGetCachedSessionBinding_LegacySession(t *testing.T) {
	cache := newInMemoryEndpointCache()
	svc := &GatewayService{cache: cache}
	ctx := context.Background()
	groupID := stickyTestGroupIDPtr(1)
	const sessionHash = "legacy_sess"
	const accountID = int64(5)

	// 只写 account binding，不写 endpoint（模拟旧会话）
	require.NoError(t, svc.BindStickySession(ctx, groupID, sessionHash, accountID))

	gotAccount, gotEndpoint, err := svc.GetCachedSessionBinding(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Equal(t, accountID, gotAccount)
	require.Equal(t, "", gotEndpoint, "旧会话 endpoint 应为空字符串")
}

// TestDeleteSessionBinding_ClearsBothKeys 验证删除时同时清除 account 和 endpoint 两个 key。
func TestDeleteSessionBinding_ClearsBothKeys(t *testing.T) {
	cache := newInMemoryEndpointCache()
	svc := &GatewayService{cache: cache}
	ctx := context.Background()
	groupID := stickyTestGroupIDPtr(10)
	const sessionHash = "sess_del_test"

	require.NoError(t, svc.BindStickySessionWithEndpoint(ctx, groupID, sessionHash, 99, "ep-del-uuid"))

	svc.deleteSessionBinding(ctx, derefGroupID(groupID), sessionHash)

	gotAccount, gotEndpoint, err := svc.GetCachedSessionBinding(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Zero(t, gotAccount, "account key 应已删除")
	require.Equal(t, "", gotEndpoint, "endpoint key 应已删除")
}

// TestBindStickySessionWithEndpoint_EmptyEndpoint 当 endpointStableID 为空时只写 account。
func TestBindStickySessionWithEndpoint_EmptyEndpoint(t *testing.T) {
	cache := newInMemoryEndpointCache()
	svc := &GatewayService{cache: cache}
	ctx := context.Background()
	groupID := stickyTestGroupIDPtr(5)
	const sessionHash = "sess_no_ep"

	require.NoError(t, svc.BindStickySessionWithEndpoint(ctx, groupID, sessionHash, 3, ""))

	gotAccount, gotEndpoint, err := svc.GetCachedSessionBinding(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Equal(t, int64(3), gotAccount)
	require.Equal(t, "", gotEndpoint)
}

func stickyTestGroupIDPtr(v int64) *int64 { return &v }
