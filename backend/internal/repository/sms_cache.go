package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const smsVerifyCodeKeyPrefix = "sms_verify_code:"

func smsVerifyCodeKey(phone string) string {
	return smsVerifyCodeKeyPrefix + phone
}

type smsCache struct {
	rdb *redis.Client
}

// NewSmsCache 创建 SMS 验证码缓存实例
func NewSmsCache(rdb *redis.Client) service.SmsCache {
	return &smsCache{rdb: rdb}
}

func (c *smsCache) GetSmsVerifyCode(ctx context.Context, phone string) (*service.VerificationCodeData, error) {
	key := smsVerifyCodeKey(phone)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var data service.VerificationCodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *smsCache) SetSmsVerifyCode(ctx context.Context, phone string, data *service.VerificationCodeData, ttl time.Duration) error {
	key := smsVerifyCodeKey(phone)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *smsCache) DeleteSmsVerifyCode(ctx context.Context, phone string) error {
	key := smsVerifyCodeKey(phone)
	return c.rdb.Del(ctx, key).Err()
}
