//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "china mainland 11 digits", input: "13800000000", want: "+8613800000000"},
		{name: "already E.164 with country code", input: "+8613800000000", want: "+8613800000000"},
		{name: "already E.164 non-china", input: "+14155552671", want: "+14155552671"},
		{name: "trims surrounding whitespace", input: "  13800000000  ", want: "+8613800000000"},
		{name: "empty string", input: "", wantErr: true},
		{name: "whitespace only", input: "   ", wantErr: true},
		{name: "too short for mainland", input: "1380000000", wantErr: true},
		{name: "too long for mainland", input: "138000000001", wantErr: true},
		{name: "mainland number not starting with 1", input: "23800000000", wantErr: true},
		{name: "contains letters, wrong length still rejected", input: "138000000a", wantErr: true},
		{name: "11 chars with trailing non-digit rejected", input: "1380000000a", wantErr: true},
		{name: "plus without digits", input: "+", wantErr: true},
		{name: "plus with leading zero digit", input: "+0123456789", wantErr: true},
		{name: "plus with dashes not normalized", input: "+861380-000-0000", wantErr: true},
		{name: "plus too short", input: "+123456", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePhone(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidPhoneNumber)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// phoneAuthSettingRepoStub 只实现手机认证路径用到的 SettingRepository 方法。
type phoneAuthSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func (s *phoneAuthSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *phoneAuthSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := s.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func newPhoneRegisterEnabledSettingService() *SettingService {
	repo := &phoneAuthSettingRepoStub{values: map[string]string{
		SettingKeyPhoneRegisterEnabled: "true",
	}}
	return NewSettingService(repo, nil)
}

// phoneAuthSmsCacheStub 固定返回预置验证码数据，供 VerifyCode 走真实校验逻辑。
type phoneAuthSmsCacheStub struct {
	data    *VerificationCodeData
	deleted bool
}

func (c *phoneAuthSmsCacheStub) GetSmsVerifyCode(ctx context.Context, phone string) (*VerificationCodeData, error) {
	if c.deleted || c.data == nil {
		return nil, ErrInvalidSmsCode
	}
	return c.data, nil
}

func (c *phoneAuthSmsCacheStub) SetSmsVerifyCode(ctx context.Context, phone string, data *VerificationCodeData, ttl time.Duration) error {
	c.data = data
	return nil
}

func (c *phoneAuthSmsCacheStub) DeleteSmsVerifyCode(ctx context.Context, phone string) error {
	c.deleted = true
	return nil
}

func (c *phoneAuthSmsCacheStub) IncrSmsVerifyAttempts(ctx context.Context, phone string, ttl time.Duration) (int64, error) {
	return 1, nil
}

// phoneAuthUserRepoStub 只实现手机登录路径用到的 UserRepository 方法。
type phoneAuthUserRepoStub struct {
	UserRepository
	byPhone       *User
	existsByPhone bool
}

func (s *phoneAuthUserRepoStub) GetByPhone(ctx context.Context, phone string) (*User, error) {
	if s.byPhone == nil {
		return nil, ErrUserNotFound
	}
	return s.byPhone, nil
}

func (s *phoneAuthUserRepoStub) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	return s.existsByPhone, nil
}

func TestSendSmsCodeForAuth_RegistrationDisabled(t *testing.T) {
	svc := &AuthService{} // settingService nil
	_, err := svc.SendSmsCodeForAuth(context.Background(), "13800000000")
	assert.ErrorIs(t, err, ErrRegDisabled)
}

func TestSendSmsCodeForAuth_SmsNotConfigured(t *testing.T) {
	svc := &AuthService{settingService: newPhoneRegisterEnabledSettingService()}
	_, err := svc.SendSmsCodeForAuth(context.Background(), "13800000000")
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
}

func TestSendSmsCodeForAuth_InvalidPhone(t *testing.T) {
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, &phoneAuthSmsCacheStub{}),
	}
	_, err := svc.SendSmsCodeForAuth(context.Background(), "not-a-phone")
	assert.ErrorIs(t, err, ErrInvalidPhoneNumber)
}

func TestRegisterWithPhone_RegistrationDisabled(t *testing.T) {
	svc := &AuthService{}
	_, err := svc.RegisterWithPhone(context.Background(), "13800000000", "123456", "u", "", "", "")
	assert.ErrorIs(t, err, ErrRegDisabled)
}

func TestRegisterWithPhone_SmsNotConfigured(t *testing.T) {
	svc := &AuthService{settingService: newPhoneRegisterEnabledSettingService()}
	_, err := svc.RegisterWithPhone(context.Background(), "13800000000", "123456", "u", "", "", "")
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
}

func TestRegisterWithPhone_InvalidPhone(t *testing.T) {
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, &phoneAuthSmsCacheStub{}),
	}
	_, err := svc.RegisterWithPhone(context.Background(), "123", "123456", "u", "", "", "")
	assert.ErrorIs(t, err, ErrInvalidPhoneNumber)
}

func TestRegisterWithPhone_WrongCode(t *testing.T) {
	cache := &phoneAuthSmsCacheStub{data: &VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, cache),
	}
	_, err := svc.RegisterWithPhone(context.Background(), "13800000000", "000000", "u", "", "", "")
	assert.ErrorIs(t, err, ErrInvalidSmsCode)
}

func TestRegisterWithPhone_ExpiredCode(t *testing.T) {
	cache := &phoneAuthSmsCacheStub{data: &VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(-time.Minute),
	}}
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, cache),
	}
	_, err := svc.RegisterWithPhone(context.Background(), "13800000000", "123456", "u", "", "", "")
	assert.ErrorIs(t, err, ErrInvalidSmsCode)
}

func TestLoginWithPhone_RegistrationDisabled(t *testing.T) {
	svc := &AuthService{}
	_, err := svc.LoginWithPhone(context.Background(), "13800000000", "123456", "")
	assert.ErrorIs(t, err, ErrRegDisabled)
}

func TestLoginWithPhone_SmsNotConfigured(t *testing.T) {
	svc := &AuthService{settingService: newPhoneRegisterEnabledSettingService()}
	_, err := svc.LoginWithPhone(context.Background(), "13800000000", "123456", "")
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
}

func TestLoginWithPhone_InvalidPhone(t *testing.T) {
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, &phoneAuthSmsCacheStub{}),
	}
	_, err := svc.LoginWithPhone(context.Background(), "123", "123456", "")
	assert.ErrorIs(t, err, ErrInvalidPhoneNumber)
}

func TestLoginWithPhone_WrongCode(t *testing.T) {
	cache := &phoneAuthSmsCacheStub{data: &VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, cache),
	}
	_, err := svc.LoginWithPhone(context.Background(), "13800000000", "000000", "")
	assert.ErrorIs(t, err, ErrInvalidSmsCode)
}

func TestLoginWithPhone_ExistingActiveUser(t *testing.T) {
	cache := &phoneAuthSmsCacheStub{data: &VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	existing := &User{ID: 42, Status: StatusActive}
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, cache),
		userRepo:       &phoneAuthUserRepoStub{byPhone: existing},
	}
	user, err := svc.LoginWithPhone(context.Background(), "13800000000", "123456", "")
	require.NoError(t, err)
	assert.Equal(t, int64(42), user.ID)
}

func TestLoginWithPhone_ExistingInactiveUser(t *testing.T) {
	cache := &phoneAuthSmsCacheStub{data: &VerificationCodeData{
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	existing := &User{ID: 42, Status: StatusDisabled}
	svc := &AuthService{
		settingService: newPhoneRegisterEnabledSettingService(),
		smsService:     NewSmsService(nil, cache),
		userRepo:       &phoneAuthUserRepoStub{byPhone: existing},
	}
	_, err := svc.LoginWithPhone(context.Background(), "13800000000", "123456", "")
	assert.ErrorIs(t, err, ErrUserNotActive)
}
