//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/sms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// smsSettingRepoStub 只实现 loadSmsClient 用到的 GetMultiple。
type smsSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func (s *smsSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := s.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func TestGenerateSmsCode(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		code, err := generateSmsCode()
		require.NoError(t, err)
		require.Len(t, code, 6)
		for _, r := range code {
			assert.True(t, r >= '0' && r <= '9', "expected digit, got %q", r)
		}
		seen[code] = true
	}
	// 50 次生成不应完全相同（极小概率碰撞不视为失败，只验证不是恒定值）。
	assert.Greater(t, len(seen), 1, "expected varying codes across generations")
}

func TestLoadSmsClient_DefaultsToVolcengine(t *testing.T) {
	repo := &smsSettingRepoStub{values: map[string]string{
		SettingKeyVolcengineAccessKeyID:     "ak",
		SettingKeyVolcengineAccessKeySecret: "sk",
		SettingKeyVolcengineSmsAccountID:    "acc",
		SettingKeyVolcengineSmsSign:         "sign",
		SettingKeyVolcengineSmsTemplateID:   "tpl",
	}}
	svc := NewSmsService(repo, nil)
	client, err := svc.loadSmsClient(context.Background())
	require.NoError(t, err)
	_, ok := client.(*sms.VolcengineClient)
	assert.True(t, ok, "expected *sms.VolcengineClient, got %T", client)
}

func TestLoadSmsClient_VolcengineMissingCredentials(t *testing.T) {
	repo := &smsSettingRepoStub{values: map[string]string{
		SettingKeySmsFrontend: "volcengine",
	}}
	svc := NewSmsService(repo, nil)
	_, err := svc.loadSmsClient(context.Background())
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
}

func TestLoadSmsClient_Tencent(t *testing.T) {
	repo := &smsSettingRepoStub{values: map[string]string{
		SettingKeySmsFrontend:          "tencent",
		SettingKeyTencentSecretID:      "id",
		SettingKeyTencentSecretKey:     "key",
		SettingKeyTencentSmsSdkAppID:   "app",
		SettingKeyTencentSmsSign:       "sign",
		SettingKeyTencentSmsTemplateID: "tpl",
	}}
	svc := NewSmsService(repo, nil)
	client, err := svc.loadSmsClient(context.Background())
	require.NoError(t, err)
	_, ok := client.(*sms.TencentClient)
	assert.True(t, ok, "expected *sms.TencentClient, got %T", client)
}

func TestLoadSmsClient_TencentMissingCredentials(t *testing.T) {
	repo := &smsSettingRepoStub{values: map[string]string{
		SettingKeySmsFrontend: "tencent",
	}}
	svc := NewSmsService(repo, nil)
	_, err := svc.loadSmsClient(context.Background())
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
}

func TestLoadSmsClient_Aliyun(t *testing.T) {
	repo := &smsSettingRepoStub{values: map[string]string{
		SettingKeySmsFrontend:           "aliyun",
		SettingKeyAliyunAccessKeyID:     "id",
		SettingKeyAliyunAccessKeySecret: "secret",
		SettingKeyAliyunSmsSign:         "sign",
		SettingKeyAliyunSmsTemplateCode: "tpl",
	}}
	svc := NewSmsService(repo, nil)
	client, err := svc.loadSmsClient(context.Background())
	require.NoError(t, err)
	_, ok := client.(*sms.AliyunClient)
	assert.True(t, ok, "expected *sms.AliyunClient, got %T", client)
}

func TestLoadSmsClient_AliyunMissingCredentials(t *testing.T) {
	repo := &smsSettingRepoStub{values: map[string]string{
		SettingKeySmsFrontend: "aliyun",
	}}
	svc := NewSmsService(repo, nil)
	_, err := svc.loadSmsClient(context.Background())
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
}

func TestLoadSmsClient_UnknownProviderFallsBackToVolcengineBranch(t *testing.T) {
	repo := &smsSettingRepoStub{values: map[string]string{
		SettingKeySmsFrontend: "unknown-provider",
	}}
	svc := NewSmsService(repo, nil)
	_, err := svc.loadSmsClient(context.Background())
	// 未知 provider 落入 default 分支（volcengine），凭证缺失时返回未配置错误。
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
}

// sendVerifySettingRepoStub 供 SendVerifyCode 测试复用（无凭证 → loadSmsClient 报错，
// 不会触发真实网络调用）。
type sendVerifySettingRepoStub struct {
	SettingRepository
}

func (s *sendVerifySettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	return map[string]string{}, nil
}

// sendVerifyCacheStub 记录 SetSmsVerifyCode 调用与 cooldown 场景下的既有数据。
type sendVerifyCacheStub struct {
	existing  *VerificationCodeData
	getErr    error
	setCalled bool
	setData   *VerificationCodeData
}

func (c *sendVerifyCacheStub) GetSmsVerifyCode(ctx context.Context, phone string) (*VerificationCodeData, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return c.existing, nil
}

func (c *sendVerifyCacheStub) SetSmsVerifyCode(ctx context.Context, phone string, data *VerificationCodeData, ttl time.Duration) error {
	c.setCalled = true
	c.setData = data
	return nil
}

func (c *sendVerifyCacheStub) DeleteSmsVerifyCode(ctx context.Context, phone string) error {
	return nil
}

func (c *sendVerifyCacheStub) IncrSmsVerifyAttempts(ctx context.Context, phone string, ttl time.Duration) (int64, error) {
	return 1, nil
}

func TestSendVerifyCode_CooldownBlocksResend(t *testing.T) {
	cache := &sendVerifyCacheStub{existing: &VerificationCodeData{
		Code:      "123456",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}}
	svc := NewSmsService(&sendVerifySettingRepoStub{}, cache)

	res, err := svc.SendVerifyCode(context.Background(), "13800000000")
	assert.ErrorIs(t, err, ErrSmsCodeTooFrequent)
	require.NotNil(t, res)
	assert.Greater(t, res.Countdown, 0)
	assert.False(t, cache.setCalled, "should not overwrite existing code within cooldown")
}

func TestSendVerifyCode_AllowsResendAfterCooldown(t *testing.T) {
	cache := &sendVerifyCacheStub{existing: &VerificationCodeData{
		Code:      "123456",
		CreatedAt: time.Now().Add(-2 * time.Minute), // 已超过 1 分钟冷却
		ExpiresAt: time.Now().Add(13 * time.Minute),
	}}
	svc := NewSmsService(&sendVerifySettingRepoStub{}, cache)

	_, err := svc.SendVerifyCode(context.Background(), "13800000000")
	// 未配置任何短信厂商凭证，loadSmsClient 报错，但说明冷却检查已放行、走到发送阶段。
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
	assert.True(t, cache.setCalled, "expected new code to be persisted before send attempt")
}

func TestSendVerifyCode_NoExistingCode_ProceedsToSend(t *testing.T) {
	cache := &sendVerifyCacheStub{}
	svc := NewSmsService(&sendVerifySettingRepoStub{}, cache)

	_, err := svc.SendVerifyCode(context.Background(), "13800000000")
	assert.ErrorIs(t, err, ErrSmsNotConfigured)
	assert.True(t, cache.setCalled)
	require.NotNil(t, cache.setData)
	assert.Len(t, cache.setData.Code, 6)
}
