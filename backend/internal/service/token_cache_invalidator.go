package service

import (
	"context"
	"log/slog"
	"strconv"
)

type TokenCacheInvalidator interface {
	InvalidateToken(ctx context.Context, account *Account) error
}

type CompositeTokenCacheInvalidator struct {
	cache GeminiTokenCache // 统一使用一个缓存接口，通过缓存键前缀区分平台
}

func NewCompositeTokenCacheInvalidator(cache GeminiTokenCache) *CompositeTokenCacheInvalidator {
	return &CompositeTokenCacheInvalidator{
		cache: cache,
	}
}

func (c *CompositeTokenCacheInvalidator) InvalidateToken(ctx context.Context, account *Account) error {
	if c == nil || c.cache == nil || account == nil {
		return nil
	}

	var keysToDelete []string
	accountIDKey := "account:" + strconv.FormatInt(account.ID, 10)

	switch account.Platform {
	case PlatformGemini:
		// Gemini service_account 可能有两种缓存键：project_id 或 account_id
		// 首次获取 token 时可能没有 project_id，之后自动检测到 project_id 后会使用新 key
		// 刷新时需要同时删除两种可能的 key，确保不会遗留旧缓存
		keysToDelete = append(keysToDelete, GeminiTokenCacheKey(account))
		keysToDelete = append(keysToDelete, "gemini:"+accountIDKey)
	default:
		return nil
	}

	// 删除所有可能的缓存键（去重后）
	seen := make(map[string]bool)
	for _, key := range keysToDelete {
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := c.cache.DeleteAccessToken(ctx, key); err != nil {
			slog.Warn("token_cache_delete_failed", "key", key, "account_id", account.ID, "error", err)
		}
	}

	return nil
}
