//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestSettingService_UpdateSettings_PersistsForkCustomFields 回归守护。
//
// 上游 0.1.147 同步合并（commit 7c9e09d29）曾静默删除 buildSystemSettingsUpdates 尾部
// 一整段 fork 自定义字段的 DB 写入，导致 currency_mode/ui_theme/phone_register 等设置
// 只进内存缓存、重启即丢——表现为后台每次弹「请选择货币显示模式」向导且保存不生效。
// 此测试确保这些 fork 字段始终被写入 SetMultiple 的 updates（落库），防止下次上游合并再次覆盖。
func TestSettingService_UpdateSettings_PersistsForkCustomFields(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		CurrencyMode:         "cny",
		CNYRate:              7.2,
		UITheme:              "violet",
		ShowOverseasModels:   false,
		PhoneRegisterEnabled: true,
		PasswordLoginEnabled: false,
		SmsProvider:          "volcengine",
	})
	require.NoError(t, err)

	require.Equal(t, "cny", repo.updates[SettingKeyCurrencyMode], "currency_mode 必须持久化到 DB（0.1.147 合并回归点）")
	require.Equal(t, "violet", repo.updates[SettingKeyUITheme])
	require.Equal(t, "false", repo.updates[SettingKeyShowOverseasModels])
	require.Equal(t, "volcengine", repo.updates[SettingKeySmsFrontend])
	require.Equal(t, "true", repo.updates[SettingKeyPhoneRegisterEnabled])
	require.Equal(t, "false", repo.updates[SettingKeyPasswordLoginEnabled])
	require.Contains(t, repo.updates, SettingKeyCNYRate, "cny_rate 必须持久化")
	// phone_register 启用时互斥关闭 email_verify（原 fork 逻辑）
	require.Equal(t, "false", repo.updates[SettingKeyEmailVerifyEnabled])
}

// forkEmptyPreserveKeys 空值必须「保留原值、不写库」的 fork 字符串设置全集。
// 新增同类字段（枚举/凭证类，空值无业务意义）时必须同步加进这里。
var forkEmptyPreserveKeys = []string{
	SettingKeyUITheme,
	SettingKeyCurrencyMode,
	SettingKeyCNYRate,
	SettingKeySmsFrontend,
	SettingKeyVolcengineAccessKeyID,
	SettingKeyVolcengineSmsAccountID,
	SettingKeyVolcengineSmsSign,
	SettingKeyVolcengineSmsTemplateID,
	SettingKeyVolcengineAccessKeySecret,
	SettingKeyTencentSmsSdkAppID,
	SettingKeyTencentSmsSign,
	SettingKeyTencentSmsTemplateID,
	SettingKeyTencentSecretID,
	SettingKeyTencentSecretKey,
	SettingKeyAliyunSmsSign,
	SettingKeyAliyunSmsTemplateCode,
	SettingKeyAliyunAccessKeyID,
	SettingKeyAliyunAccessKeySecret,
}

// TestSettingService_UpdateSettings_EmptyForkFieldsDoNotWipe 回归守护。
//
// 全量 PUT 语义下，客户端带显式空值（表单未回填、部署窗口竞态——旧后端 GET 响应无
// 新字段 → 表单空默认值 → 新后端保存、旧缓存前端 bundle）曾把 DB 已配置的
// currency_mode 冲成空串，货币选择弹窗重现（线上 2026-07-19 实际发生）。同类风险
// 覆盖所有枚举/凭证类 fork 字符串字段与 cny_rate：空值/非正数必须视为
// 「未设置、保留原值」——即不写入对应 key。
func TestSettingService_UpdateSettings_EmptyForkFieldsDoNotWipe(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		UITheme:      "  ",
		CurrencyMode: "  ",
		CNYRate:      0,
		SmsProvider:  "",
	})
	require.NoError(t, err)

	for _, key := range forkEmptyPreserveKeys {
		require.NotContains(t, repo.updates, key,
			"%s 为空值时不得写入 DB，否则全量 PUT 会清空已配置的值", key)
	}
}

// TestSettingService_UpdateSettings_NonEmptyForkFieldsStillPersist 非空值必须照常落库
// （防止「空值保留」被误写成「永不写入」）。
func TestSettingService_UpdateSettings_NonEmptyForkFieldsStillPersist(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		UITheme:                      "violet",
		CurrencyMode:                 "cny",
		CNYRate:                      7.2,
		SmsProvider:                  "tencent",
		VolcengineSmsAccessKeyID:     "vk",
		VolcengineSmsAccountID:       "va",
		VolcengineSmsSign:            "vs",
		VolcengineSmsTemplateID:      "vt",
		VolcengineSmsAccessKeySecret: "vsec",
		TencentSmsSdkAppID:           "ta",
		TencentSmsSign:               "ts",
		TencentSmsTemplateID:         "tt",
		TencentSmsSecretID:           "tid",
		TencentSmsSecretKey:          "tkey",
		AliyunSmsSign:                "as",
		AliyunSmsTemplateCode:        "at",
		AliyunSmsAccessKeyID:         "ak",
		AliyunSmsAccessKeySecret:     "asec",
	})
	require.NoError(t, err)

	for _, key := range forkEmptyPreserveKeys {
		require.Contains(t, repo.updates, key, "%s 非空时必须落库", key)
	}
	require.Equal(t, "cny", repo.updates[SettingKeyCurrencyMode])
	require.Equal(t, "tencent", repo.updates[SettingKeySmsFrontend])
}
