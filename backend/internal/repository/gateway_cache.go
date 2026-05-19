package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	stickySessionPrefix  = "sticky_session:"
	stickyEndpointPrefix = "sticky_endpoint:"
)

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) service.GatewayCache {
	return &gatewayCache{rdb: rdb}
}

// buildSessionKey 构建 session key，包含 groupID 实现分组隔离
// 格式: sticky_session:{groupID}:{sessionHash}
func buildSessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", stickySessionPrefix, groupID, sessionHash)
}

// buildEndpointKey 构建 endpoint sticky key（P5-3）
// 格式: sticky_endpoint:{groupID}:{sessionHash}
func buildEndpointKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", stickyEndpointPrefix, groupID, sessionHash)
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Get(ctx, key).Int64()
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, accountID, ttl).Err()
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// DeleteSessionAccountID 删除粘性会话与账号的绑定关系。
// 当检测到绑定的账号不可用（如状态错误、禁用、不可调度等）时调用，
// 以便下次请求能够重新选择可用账号。
//
// DeleteSessionAccountID removes the sticky session binding for the given session.
// Called when the bound account becomes unavailable (e.g., error status, disabled,
// or unschedulable), allowing subsequent requests to select a new available account.
func (c *gatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}

// GetSessionEndpointStableID 返回会话绑定的 endpoint stable_id（P5-3）。
// 当 key 不存在（旧会话或非 generic 账号）时返回 ("", nil)。
func (c *gatewayCache) GetSessionEndpointStableID(ctx context.Context, groupID int64, sessionHash string) (string, error) {
	key := buildEndpointKey(groupID, sessionHash)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", nil // 未命中视为正常（旧会话平滑降级）
	}
	return val, nil
}

// SetSessionEndpointStableID 存储会话 → endpoint stable_id 绑定（P5-3）。
func (c *gatewayCache) SetSessionEndpointStableID(ctx context.Context, groupID int64, sessionHash string, stableID string, ttl time.Duration) error {
	if stableID == "" {
		return nil
	}
	key := buildEndpointKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, stableID, ttl).Err()
}

// DeleteSessionEndpointStableID 清除 endpoint 维度绑定（P5-3）。
// 与 DeleteSessionAccountID 配对调用，best-effort，忽略未命中。
func (c *gatewayCache) DeleteSessionEndpointStableID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildEndpointKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}
