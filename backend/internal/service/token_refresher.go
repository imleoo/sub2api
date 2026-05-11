package service

import (
	"context"
	"time"
)

// TokenRefresher 定义平台特定的token刷新策略接口
// 通过此接口可以扩展支持不同平台（Anthropic/OpenAI/Gemini）
type TokenRefresher interface {
	// CanRefresh 检查此刷新器是否能处理指定账号
	CanRefresh(account *Account) bool

	// NeedsRefresh 检查账号的token是否需要刷新
	NeedsRefresh(account *Account, refreshWindow time.Duration) bool

	// Refresh 执行token刷新，返回更新后的credentials
	// 注意：返回的map应该保留原有credentials中的所有字段，只更新token相关字段
	Refresh(ctx context.Context, account *Account) (map[string]any, error)
}

