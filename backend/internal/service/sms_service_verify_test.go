//go:build unit

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// smsVerifyCacheStub 是可控的 SmsCache 实现，用尝试计数器复现原子自增语义，
// 便于断言失败次数上限与锁定行为。
type smsVerifyCacheStub struct {
	data     *service.VerificationCodeData
	attempts int64
	deleted  bool
}

func (s *smsVerifyCacheStub) GetSmsVerifyCode(context.Context, string) (*service.VerificationCodeData, error) {
	if s.deleted || s.data == nil {
		return nil, errors.New("not found")
	}
	return s.data, nil
}

func (s *smsVerifyCacheStub) SetSmsVerifyCode(context.Context, string, *service.VerificationCodeData, time.Duration) error {
	return nil
}

func (s *smsVerifyCacheStub) DeleteSmsVerifyCode(context.Context, string) error {
	s.deleted = true
	return nil
}

func (s *smsVerifyCacheStub) IncrSmsVerifyAttempts(context.Context, string, time.Duration) (int64, error) {
	s.attempts++
	return s.attempts, nil
}

func newSmsServiceForVerify(cache service.SmsCache) *service.SmsService {
	return service.NewSmsService(nil, cache)
}

func TestSmsVerifyCode_Success(t *testing.T) {
	cache := &smsVerifyCacheStub{data: &service.VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	svc := newSmsServiceForVerify(cache)

	if err := svc.VerifyCode(context.Background(), "13800000000", "123456"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !cache.deleted {
		t.Fatal("expected code to be deleted after successful verification")
	}
}

func TestSmsVerifyCode_WrongBelowMax(t *testing.T) {
	cache := &smsVerifyCacheStub{data: &service.VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	svc := newSmsServiceForVerify(cache)

	// 前 4 次错误：返回无效验证码，逐次累加，未锁定。
	for i := 1; i <= 4; i++ {
		err := svc.VerifyCode(context.Background(), "13800000000", "000000")
		if !errors.Is(err, service.ErrInvalidSmsCode) {
			t.Fatalf("attempt %d: expected ErrInvalidSmsCode, got %v", i, err)
		}
		if cache.deleted {
			t.Fatalf("attempt %d: code should not be deleted before reaching max", i)
		}
	}
	if cache.attempts != 4 {
		t.Fatalf("expected attempts counter 4, got %d", cache.attempts)
	}
}

func TestSmsVerifyCode_LocksOnMaxAttempts(t *testing.T) {
	cache := &smsVerifyCacheStub{data: &service.VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	svc := newSmsServiceForVerify(cache)

	var lastErr error
	for i := 1; i <= 5; i++ {
		lastErr = svc.VerifyCode(context.Background(), "13800000000", "000000")
	}
	if !errors.Is(lastErr, service.ErrSmsCodeMaxAttempts) {
		t.Fatalf("expected ErrSmsCodeMaxAttempts on 5th attempt, got %v", lastErr)
	}
	if !cache.deleted {
		t.Fatal("expected code to be deleted (locked) after reaching max attempts")
	}
}

func TestSmsVerifyCode_Expired(t *testing.T) {
	cache := &smsVerifyCacheStub{data: &service.VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(-time.Minute),
	}}
	svc := newSmsServiceForVerify(cache)

	if err := svc.VerifyCode(context.Background(), "13800000000", "123456"); !errors.Is(err, service.ErrInvalidSmsCode) {
		t.Fatalf("expected ErrInvalidSmsCode for expired code, got %v", err)
	}
}
