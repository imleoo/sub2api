//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendRawUsageLogModelWhereCondition(t *testing.T) {
	t.Run("空白 model 不追加条件", func(t *testing.T) {
		conditions, args := appendRawUsageLogModelWhereCondition(nil, nil, "   ")
		require.Empty(t, conditions)
		require.Empty(t, args)
	})

	t.Run("单个 model 追加条件与占位符", func(t *testing.T) {
		conditions, args := appendRawUsageLogModelWhereCondition(nil, nil, "gpt-5")
		require.Equal(t, []string{"model = $1"}, conditions)
		require.Equal(t, []any{"gpt-5"}, args)
	})

	t.Run("占位符编号跟随已有 args 长度递增", func(t *testing.T) {
		existingConditions := []string{"user_id = $1"}
		existingArgs := []any{int64(42)}

		conditions, args := appendRawUsageLogModelWhereCondition(existingConditions, existingArgs, "claude-3")
		require.Equal(t, []string{"user_id = $1", "model = $2"}, conditions)
		require.Equal(t, []any{int64(42), "claude-3"}, args)
	})

	t.Run("含 SQL 特殊字符的 model 值原样作为参数传递而非拼接进语句", func(t *testing.T) {
		malicious := "gpt-5'; DROP TABLE usage_logs; --"
		conditions, args := appendRawUsageLogModelWhereCondition(nil, nil, malicious)
		require.Equal(t, []string{"model = $1"}, conditions)
		require.Equal(t, []any{malicious}, args)
	})
}

func TestAppendRawUsageLogModelQueryFilter(t *testing.T) {
	t.Run("空白 model 不修改 query", func(t *testing.T) {
		query, args := appendRawUsageLogModelQueryFilter("SELECT 1", nil, "")
		require.Equal(t, "SELECT 1", query)
		require.Empty(t, args)
	})

	t.Run("追加 AND model = 占位符", func(t *testing.T) {
		query, args := appendRawUsageLogModelQueryFilter("SELECT 1 WHERE user_id = $1", []any{int64(1)}, "gpt-5")
		require.Equal(t, "SELECT 1 WHERE user_id = $1 AND model = $2", query)
		require.Equal(t, []any{int64(1), "gpt-5"}, args)
	})
}

func TestAppendUsageLogModelWhereCondition_SourceRouting(t *testing.T) {
	t.Run("source 为空时委托给 raw 条件（向后兼容原始 model 列）", func(t *testing.T) {
		conditionsRaw, argsRaw := appendRawUsageLogModelWhereCondition(nil, nil, "gpt-5")
		conditionsRouted, argsRouted := appendUsageLogModelWhereCondition(nil, nil, "gpt-5", "")
		require.Equal(t, conditionsRaw, conditionsRouted)
		require.Equal(t, argsRaw, argsRouted)
	})

	t.Run("source 非空且 model 为空时不追加任何条件", func(t *testing.T) {
		conditions, args := appendUsageLogModelWhereCondition(nil, nil, "  ", "requested")
		require.Empty(t, conditions)
		require.Empty(t, args)
	})

	t.Run("source 非空时使用 resolveModelDimensionExpression 而非原始 model 列", func(t *testing.T) {
		conditions, args := appendUsageLogModelWhereCondition(nil, nil, "gpt-5", "requested")
		require.Len(t, conditions, 1)
		require.NotEqual(t, "model = $1", conditions[0], "source 非空时不应退化为原始 model 列比较")
		require.Equal(t, []any{"gpt-5"}, args)
	})
}

func TestAppendUsageLogModelQueryFilter_SourceRouting(t *testing.T) {
	t.Run("source 为空时委托给 raw query filter", func(t *testing.T) {
		queryRaw, argsRaw := appendRawUsageLogModelQueryFilter("SELECT 1", nil, "gpt-5")
		queryRouted, argsRouted := appendUsageLogModelQueryFilter("SELECT 1", nil, "gpt-5", "")
		require.Equal(t, queryRaw, queryRouted)
		require.Equal(t, argsRaw, argsRouted)
	})

	t.Run("source 非空且 model 为空时不修改 query", func(t *testing.T) {
		query, args := appendUsageLogModelQueryFilter("SELECT 1", nil, "", "requested")
		require.Equal(t, "SELECT 1", query)
		require.Empty(t, args)
	})
}
