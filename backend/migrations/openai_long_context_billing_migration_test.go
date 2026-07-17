package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// fork：上游此迁移原本还包含 spark 影子账号（parent_account_id/quota_dimension）向子账号
// 传播 openai_long_context_billing_enabled 的逻辑；该账号层级体系已随订阅逆向清理（功能 35）
// 整体移除，accounts 表没有 parent_account_id/quota_dimension 列，故本迁移只保留与扁平
// openai 账号相关的默认值回填 + 布尔类型强制校验部分，测试相应精简。
func TestMigration175DefaultsOrdinaryOpenAIAndBackfillsBooleanDefault(t *testing.T) {
	content, err := FS.ReadFile("175_default_openai_long_context_billing.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "NEW.platform IS DISTINCT FROM 'openai'")
	require.Contains(t, sql, "jsonb_typeof")
	require.Contains(t, sql, "openai_long_context_billing_enabled")
}

func TestMigration175GuardsMixedVersionAccountWrites(t *testing.T) {
	content, err := FS.ReadFile("175_default_openai_long_context_billing.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "RETURNS TRIGGER")
	require.Contains(t, sql, "BEFORE INSERT OR UPDATE")
	require.Contains(t, sql, "CREATE TRIGGER")
	require.Contains(t, sql, "must be a boolean")
	require.Contains(t, sql, "jsonb_typeof(NEW.extra->'openai_long_context_billing_enabled') IS DISTINCT FROM 'boolean'")
	require.Contains(t, sql, "TG_OP = 'UPDATE'")
	require.Contains(t, sql, "OLD.extra->'openai_long_context_billing_enabled'")
}
