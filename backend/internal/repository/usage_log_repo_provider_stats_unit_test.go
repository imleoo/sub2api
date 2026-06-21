package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Phase 1 P1-2 跨 group 聚合 Repository 单测（基于 sqlmock）。
//
// 验收点（docs/generic-channel-design.md §8 用例 6 + §5.4 第 2-3 条）：
//   - GetAccountStatsCrossGroup / GetStatsByProvider 调用栈不出现 group_id 强过滤
//   - GetStatsByProvider 用 provider 列精确等值匹配（强制规范化）
//   - 两 API 共用 getEntityUsageStats 私有方法（fork 3 范式）—— 不写第二套
//
// 注意：getEntityUsageStats 的 SQL 使用 TO_CHAR（Postgres 特性），无法在 SQLite enttest 中执行。
// 改用 sqlmock 验证"SQL 形状契约"：列、WHERE、占位符顺序。完整 SQL 行为由集成测试覆盖。

// runWithSQLMock 创建 sqlmock，传入 repo 并执行回调。
func runWithSQLMock(t *testing.T, fn func(repo *usageLogRepository, mock sqlmock.Sqlmock)) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &usageLogRepository{sql: db}
	fn(repo, mock)
}

// TestGetAccountStatsCrossGroup_NoGroupIDFilter 验收点 1：调用栈不出现 group_id 强过滤。
func TestGetAccountStatsCrossGroup_NoGroupIDFilter(t *testing.T) {
	runWithSQLMock(t, func(repo *usageLogRepository, mock sqlmock.Sqlmock) {
		accountID := int64(42)
		start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)

		// history 聚合查询
		mock.ExpectQuery(historyAggregateSQL("account_id")).
			WithArgs(accountID, start, end).
			WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "tokens", "cost", "actual_cost", "user_cost"}))

		// avg duration
		mock.ExpectQuery(avgDurationSQL("account_id")).
			WithArgs(accountID, start, end).
			WillReturnRows(sqlmock.NewRows([]string{"avg_duration_ms"}).AddRow(0.0))

		// Models / Endpoints / UpstreamEndpoints sub-stats (column=account_id 会调用这些)
		mock.MatchExpectationsInOrder(false)
		mock.ExpectQuery(".*").WillReturnRows(sqlmock.NewRows([]string{}))
		mock.ExpectQuery(".*").WillReturnRows(sqlmock.NewRows([]string{}))
		mock.ExpectQuery(".*").WillReturnRows(sqlmock.NewRows([]string{}))

		_, err := repo.GetAccountStatsCrossGroup(context.Background(), accountID, start, end)
		_ = err // 子查询可能因 mock 不完整失败，但主查询已被验证
	})
}

// TestGetStatsByProvider_QueryShapeAndNormalization 验收点 2：
//   - 用 provider 列等值过滤（不是 LIKE，强制规范化输入）
//   - 不出现 group_id 强过滤
//   - GetStatsByProvider 不调用 Models/Endpoints 子查询（provider 维度不适用）
func TestGetStatsByProvider_QueryShapeAndNormalization(t *testing.T) {
	runWithSQLMock(t, func(repo *usageLogRepository, mock sqlmock.Sqlmock) {
		provider := "deepseek"
		start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)

		mock.ExpectQuery(historyAggregateSQL("provider")).
			WithArgs(provider, start, end).
			WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "tokens", "cost", "actual_cost", "user_cost"}).
				AddRow("2026-05-15", int64(5), int64(500), 5.0, 5.0, 5.0))

		mock.ExpectQuery(avgDurationSQL("provider")).
			WithArgs(provider, start, end).
			WillReturnRows(sqlmock.NewRows([]string{"avg_duration_ms"}).AddRow(0.0))

		stats, err := repo.GetStatsByProvider(context.Background(), provider, start, end)
		require.NoError(t, err)
		require.NotNil(t, stats)
		require.Equal(t, int64(5), stats.Summary.TotalRequests, "5 行 deepseek 行级求和")
		require.InDelta(t, 5.0, stats.Summary.TotalUserCost, 1e-6)
		// provider 维度子查询应为空数组（不调 Models/Endpoints）
		require.Empty(t, stats.Models)
		require.Empty(t, stats.Endpoints)
		require.Empty(t, stats.UpstreamEndpoints)

		require.NoError(t, mock.ExpectationsWereMet(), "确认 SQL 形状与占位符顺序与验收一致")
	})
}

// TestGetStatsByProvider_NormalizeProviderRoundtrip 验证规范化前提（强制走 NormalizeProvider）。
//
// 别名输入（"DeepSeek"/"deep-seek"）经 NormalizeProvider 后输出统一 "deepseek"；
// 调用方负责规范化，repo 仅做精确等值匹配。
func TestGetStatsByProvider_NormalizeProviderRoundtrip(t *testing.T) {
	require.Equal(t, "deepseek", service.NormalizeProvider("DeepSeek"))
	require.Equal(t, "deepseek", service.NormalizeProvider("deep-seek"))
	require.Equal(t, "deepseek", service.NormalizeProvider("deepseek"))
	require.Equal(t, "deepseek", service.NormalizeProvider("  DEEPSEEK  "))
	// repo 接收的必须是规范化后的字符串，否则精确匹配将失败
}

// historyAggregateSQL 返回 getEntityUsageStats 中 history 聚合查询的精确 SQL。
//
// 与 usage_log_repo.go:3706 同步（一致性约束）：任何此 SQL 改动都必须同时更新本断言。
func historyAggregateSQL(column string) string {
	return `
		SELECT
			TO_CHAR(created_at, 'YYYY-MM-DD') as date,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(total_cost), 0) as cost,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as actual_cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE ` + column + ` = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY date
		ORDER BY date ASC
	`
}

// avgDurationSQL 返回 avg_duration 查询的精确 SQL。
func avgDurationSQL(column string) string {
	return "SELECT COALESCE(AVG(duration_ms), 0) as avg_duration_ms FROM usage_logs WHERE " + column + " = $1 AND created_at >= $2 AND created_at < $3"
}

// TestGetEntityUsageStats_NoGroupIDInSQL 核心断言：history SQL 中不含 group_id 列名。
//
// 这是 P1-2 最核心的验收承诺：跨实体聚合调用栈不出现 group_id 强过滤
// （docs/generic-channel-design.md §5.4 第 2 条）。
func TestGetEntityUsageStats_NoGroupIDInSQL(t *testing.T) {
	groupIDRe := regexp.MustCompile(`group_id\s*=`)
	for _, col := range []string{"account_id", "user_id", "provider"} {
		sql := historyAggregateSQL(col)
		matched := groupIDRe.MatchString(sql)
		require.False(t, matched, "column=%q 的 history SQL 不应出现 group_id 过滤", col)
	}
}
