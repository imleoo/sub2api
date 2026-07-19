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

// TestSettingService_UpdateSettings_EmptyCurrencyModeDoesNotWipe 回归守护。
//
// 全量 PUT 语义下，若客户端带上 currency_mode: ""（表单未回填、旧缓存 bundle、异常客户端），
// 曾会把 DB 已配置的货币模式冲成空串，导致管理端货币选择弹窗重现（线上 2026-07-19 实际发生）。
// 系统没有合法路径主动清空货币模式，空串必须视为「未设置、保留原值」——即不写入该 key。
func TestSettingService_UpdateSettings_EmptyCurrencyModeDoesNotWipe(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		CurrencyMode: "  ",
		CNYRate:      7.2,
	})
	require.NoError(t, err)

	require.NotContains(t, repo.updates, SettingKeyCurrencyMode,
		"currency_mode 为空串时不得写入 DB，否则会清空已配置的货币模式")
	require.Contains(t, repo.updates, SettingKeyCNYRate)
}
