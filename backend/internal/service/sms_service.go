package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sms"
)

var (
	ErrSmsNotConfigured    = infraerrors.ServiceUnavailable("SMS_NOT_CONFIGURED", "SMS service not configured")
	ErrInvalidSmsCode      = infraerrors.BadRequest("INVALID_SMS_CODE", "invalid or expired SMS verification code")
	ErrSmsCodeTooFrequent  = infraerrors.TooManyRequests("SMS_CODE_TOO_FREQUENT", "please wait before requesting a new code")
	ErrSmsCodeMaxAttempts  = infraerrors.TooManyRequests("SMS_CODE_MAX_ATTEMPTS", "too many failed attempts, please request a new code")
	ErrInvalidPhoneNumber  = infraerrors.BadRequest("INVALID_PHONE_NUMBER", "invalid phone number format")
	ErrPhoneAlreadyExists  = infraerrors.Conflict("PHONE_ALREADY_EXISTS", "phone number already registered")
)

const (
	smsVerifyCodeTTL         = 15 * time.Minute
	smsVerifyCodeCooldown    = 1 * time.Minute
	smsMaxVerifyCodeAttempts = 5
)

// SmsCache 定义 SMS 验证码缓存操作
type SmsCache interface {
	GetSmsVerifyCode(ctx context.Context, phone string) (*VerificationCodeData, error)
	SetSmsVerifyCode(ctx context.Context, phone string, data *VerificationCodeData, ttl time.Duration) error
	DeleteSmsVerifyCode(ctx context.Context, phone string) error
}

// SendSmsCodeResult 发送短信验证码返回结果
type SendSmsCodeResult struct {
	Countdown int // 倒计时秒数
}

// SmsService 短信服务
type SmsService struct {
	cache       SmsCache
	settingRepo SettingRepository
}

// NewSmsService 创建短信服务实例
func NewSmsService(settingRepo SettingRepository, cache SmsCache) *SmsService {
	return &SmsService{
		cache:       cache,
		settingRepo: settingRepo,
	}
}

// generateSmsCode 生成6位数字验证码
func generateSmsCode() (string, error) {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[num.Int64()]
	}
	return string(code), nil
}

// loadVolcengineClient 从设置中动态加载火山引擎 SMS 客户端
func (s *SmsService) loadVolcengineClient(ctx context.Context) (*sms.VolcengineClient, error) {
	keys := []string{
		SettingKeyVolcengineAccessKeyID,
		SettingKeyVolcengineAccessKeySecret,
		SettingKeyVolcengineSmsAccountID,
		SettingKeyVolcengineSmsSign,
		SettingKeyVolcengineSmsTemplateID,
	}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("load sms settings: %w", err)
	}

	cfg := sms.VolcengineConfig{
		AccessKeyID:     settings[SettingKeyVolcengineAccessKeyID],
		AccessKeySecret: settings[SettingKeyVolcengineAccessKeySecret],
		AccountID:       settings[SettingKeyVolcengineSmsAccountID],
		Sign:            settings[SettingKeyVolcengineSmsSign],
		TemplateID:      settings[SettingKeyVolcengineSmsTemplateID],
	}

	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, ErrSmsNotConfigured
	}

	return sms.NewVolcengineClient(cfg), nil
}

// SendVerifyCode 生成并发送短信验证码
func (s *SmsService) SendVerifyCode(ctx context.Context, phone string) (*SendSmsCodeResult, error) {
	// 检查是否在冷却期内
	existing, err := s.cache.GetSmsVerifyCode(ctx, phone)
	if err == nil && existing != nil {
		if time.Since(existing.CreatedAt) < smsVerifyCodeCooldown {
			remaining := smsVerifyCodeCooldown - time.Since(existing.CreatedAt)
			return &SendSmsCodeResult{Countdown: int(remaining.Seconds()) + 1}, ErrSmsCodeTooFrequent
		}
	}

	// 生成验证码
	code, err := generateSmsCode()
	if err != nil {
		return nil, fmt.Errorf("generate sms code: %w", err)
	}

	// 保存到 Redis
	data := &VerificationCodeData{
		Code:      code,
		Attempts:  0,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(smsVerifyCodeTTL),
	}
	if err := s.cache.SetSmsVerifyCode(ctx, phone, data, smsVerifyCodeTTL); err != nil {
		return nil, fmt.Errorf("save sms code: %w", err)
	}

	// 发送短信
	client, err := s.loadVolcengineClient(ctx)
	if err != nil {
		slog.Error("failed to load volcengine client", "error", err)
		return nil, err
	}

	if err := client.SendCode(ctx, phone, code); err != nil {
		slog.Error("failed to send sms code", "phone_hash", fmt.Sprintf("%x", phone[:min(4, len(phone))]), "error", err)
		return nil, fmt.Errorf("send sms: %w", err)
	}

	return &SendSmsCodeResult{Countdown: int(smsVerifyCodeCooldown.Seconds())}, nil
}

// VerifyCode 验证短信验证码（验证后自动删除，防重放）
func (s *SmsService) VerifyCode(ctx context.Context, phone, code string) error {
	data, err := s.cache.GetSmsVerifyCode(ctx, phone)
	if err != nil || data == nil {
		return ErrInvalidSmsCode
	}

	if data.Attempts >= smsMaxVerifyCodeAttempts {
		return ErrSmsCodeMaxAttempts
	}

	if subtle.ConstantTimeCompare([]byte(data.Code), []byte(code)) != 1 {
		data.Attempts++
		remaining := time.Until(data.ExpiresAt)
		if remaining <= 0 {
			return ErrInvalidSmsCode
		}
		if err := s.cache.SetSmsVerifyCode(ctx, phone, data, remaining); err != nil {
			slog.Error("failed to update sms code attempt count", "phone_prefix", phone[:min(4, len(phone))], "error", err)
		}
		if data.Attempts >= smsMaxVerifyCodeAttempts {
			return ErrSmsCodeMaxAttempts
		}
		return ErrInvalidSmsCode
	}

	// 验证成功，删除验证码防重放
	if err := s.cache.DeleteSmsVerifyCode(ctx, phone); err != nil {
		slog.Error("failed to delete sms code after verification", "error", err)
	}

	return nil
}

