package service

import (
	"context"
	"time"
)

// IsSchedulableForModelWithContext 结合模型级限流判断账号是否可调度。
func (a *Account) IsSchedulableForModelWithContext(ctx context.Context, requestedModel string) bool {
	if a == nil {
		return false
	}
	if !a.IsSchedulable() {
		return false
	}
	if a.isModelRateLimitedWithContext(ctx, requestedModel) {
		return false
	}
	return true
}

// GetRateLimitRemainingTimeWithContext 获取限流剩余时间（模型级限流）。
// 返回 0 表示未限流或已过期。
func (a *Account) GetRateLimitRemainingTimeWithContext(ctx context.Context, requestedModel string) time.Duration {
	if a == nil {
		return 0
	}
	return a.GetModelRateLimitRemainingTimeWithContext(ctx, requestedModel)
}
