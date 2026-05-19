package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// Phase 2 P2-4 一致性扫描单测（基于 sqlmock）。

func newPCCMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

const groupQuerySQL = `
		SELECT id, platform, COALESCE(inbound_protocol, '')
		FROM groups
	`
const accountQuerySQL = `
		SELECT id, platform, COALESCE(outbound_protocol, '')
		FROM accounts
		WHERE deleted_at IS NULL
	`

// TestRunProtocolConsistencyCheck_AllConsistent 全表一致 → mismatch 计数 0。
func TestRunProtocolConsistencyCheck_AllConsistent(t *testing.T) {
	db, mock := newPCCMock(t)
	ctx := context.Background()

	mock.ExpectQuery(groupQuerySQL).WillReturnRows(
		sqlmock.NewRows([]string{"id", "platform", "inbound_protocol"}).
			AddRow(1, "anthropic", "anthropic_messages").
			AddRow(2, "openai", "openai_chat").
			AddRow(3, "anthropic", ""), // 空协议字段 = 兜底，不算 mismatch
	)
	mock.ExpectQuery(accountQuerySQL).WillReturnRows(
		sqlmock.NewRows([]string{"id", "platform", "outbound_protocol"}).
			AddRow(10, "anthropic", "anthropic_messages").
			AddRow(11, "openai", ""),
	)

	result, err := RunProtocolConsistencyCheck(ctx, db)
	require.NoError(t, err)
	require.Equal(t, int64(3), result.GroupsTotal)
	require.Equal(t, int64(0), result.GroupsMismatch)
	require.Equal(t, int64(2), result.AccountsTotal)
	require.Equal(t, int64(0), result.AccountsMismatch)
	require.Equal(t, int64(0), LoadGroupProtocolMismatchCount())
	require.Equal(t, int64(0), LoadAccountProtocolMismatchCount())
}

// TestRunProtocolConsistencyCheck_DetectsMismatch 检测出双字段冲突。
func TestRunProtocolConsistencyCheck_DetectsMismatch(t *testing.T) {
	db, mock := newPCCMock(t)
	ctx := context.Background()

	mock.ExpectQuery(groupQuerySQL).WillReturnRows(
		sqlmock.NewRows([]string{"id", "platform", "inbound_protocol"}).
			// platform=openai 但 inbound 写成 anthropic_messages → mismatch
			AddRow(99, "openai", "anthropic_messages"),
	)
	mock.ExpectQuery(accountQuerySQL).WillReturnRows(
		sqlmock.NewRows([]string{"id", "platform", "outbound_protocol"}).
			AddRow(101, "anthropic", "openai_chat"), // mismatch
	)

	result, err := RunProtocolConsistencyCheck(ctx, db)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.GroupsMismatch)
	require.Equal(t, int64(1), result.AccountsMismatch)
	require.Equal(t, int64(1), LoadGroupProtocolMismatchCount(), "metric counter exposed to exporter")
	require.Equal(t, int64(1), LoadAccountProtocolMismatchCount())
}

// TestRunProtocolConsistencyCheck_SkipsLingjingPlatform lingjing 等无 inbound_protocol 概念的 platform 不应误报。
func TestRunProtocolConsistencyCheck_SkipsLingjingPlatform(t *testing.T) {
	db, mock := newPCCMock(t)
	ctx := context.Background()

	mock.ExpectQuery(groupQuerySQL).WillReturnRows(
		sqlmock.NewRows([]string{"id", "platform", "inbound_protocol"}).
			// lingjing 不在 platformDefaultInboundProtocol 派生表里 → expected="" → skip
			AddRow(50, "lingjing", "anthropic_messages"),
	)
	mock.ExpectQuery(accountQuerySQL).WillReturnRows(
		sqlmock.NewRows([]string{"id", "platform", "outbound_protocol"}),
	)

	result, err := RunProtocolConsistencyCheck(ctx, db)
	require.NoError(t, err)
	require.Equal(t, int64(0), result.GroupsMismatch, "lingjing 无 inbound_protocol 派生 → 跳过，不算 mismatch")
}

// TestRunProtocolConsistencyCheck_NilDB nil sql.DB → 返回错误，不更新 counter。
func TestRunProtocolConsistencyCheck_NilDB(t *testing.T) {
	_, err := RunProtocolConsistencyCheck(context.Background(), nil)
	require.Error(t, err)
}

// TestRunProtocolConsistencyCheck_DBErrorPreservesCounter DB 错误时不修改全局 counter。
func TestRunProtocolConsistencyCheck_DBErrorPreservesCounter(t *testing.T) {
	db, mock := newPCCMock(t)
	ctx := context.Background()

	groupProtocolMismatchCount.Store(7)
	t.Cleanup(func() { groupProtocolMismatchCount.Store(0) })

	mock.ExpectQuery(groupQuerySQL).WillReturnError(fmt.Errorf("db unavailable"))

	_, err := RunProtocolConsistencyCheck(ctx, db)
	require.Error(t, err)
	require.Equal(t, int64(7), LoadGroupProtocolMismatchCount(), "DB 失败时 counter 保留旧值")
}
