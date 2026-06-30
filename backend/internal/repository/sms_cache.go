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

func smsVerifyAttemptsKey(phone string) string {
	return smsVerifyCodeKeyPrefix + phone + ":attempts"
}

// smsVerifyAttemptsIncrScript 原子自增失败尝试次数；首次自增时设置 TTL，避免孤儿键。
// 返回自增后的当前值，供调用方判断是否超过上限。
var smsVerifyAttemptsIncrScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])
	local count = redis.call('INCR', key)
	if count == 1 then
		redis.call('EXPIRE', key, ttl)
	end
	return count
`)

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
	// 同时清除验证码与失败尝试计数，保持两键一致。
	return c.rdb.Del(ctx, smsVerifyCodeKey(phone), smsVerifyAttemptsKey(phone)).Err()
}

// IncrSmsVerifyAttempts 原子自增失败尝试次数并返回自增后的值。
// ttl 用于首次自增时设置计数键过期时间（跟随验证码剩余有效期）。
func (c *smsCache) IncrSmsVerifyAttempts(ctx context.Context, phone string, ttl time.Duration) (int64, error) {
	ttlSeconds := max(int64(ttl.Seconds()), 1)
	key := smsVerifyAttemptsKey(phone)
	return smsVerifyAttemptsIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
}
