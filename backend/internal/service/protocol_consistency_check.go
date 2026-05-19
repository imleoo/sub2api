package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// ProtocolConsistencyResult 协议字段一致性扫描结果（Phase 2 P2-4）。
//
// 验收（docs/sprint-plan.md P2-4）：
//   - 启动时若发现不一致打 ERROR 日志
//   - 监控面板可见不一致行数（由 Prometheus exporter 读取 GroupMismatch/AccountMismatch metric）
type ProtocolConsistencyResult struct {
	// GroupsTotal 扫描的 group 总行数
	GroupsTotal int64
	// GroupsMismatch 双字段不一致的 group 数：inbound_protocol 非空但与 platform 派生值不同
	GroupsMismatch int64
	// AccountsTotal 扫描的 account 总行数
	AccountsTotal int64
	// AccountsMismatch 双字段不一致的 account 数
	AccountsMismatch int64
}

// 内嵌的 atomic counters 提供 metric exporter 可读取的全局计数（Phase 2 P2-4）。
// 这些不参与业务逻辑——只是 startup check 完成后被读取一次，运行时不会再变化。
var (
	groupProtocolMismatchCount   atomic.Int64
	accountProtocolMismatchCount atomic.Int64
)

// LoadGroupProtocolMismatchCount 暴露给监控/metric exporter 读取最新一次扫描的 group 不一致数。
func LoadGroupProtocolMismatchCount() int64 { return groupProtocolMismatchCount.Load() }

// LoadAccountProtocolMismatchCount 暴露给监控/metric exporter 读取最新一次扫描的 account 不一致数。
func LoadAccountProtocolMismatchCount() int64 { return accountProtocolMismatchCount.Load() }

// RunProtocolConsistencyCheck 扫描 groups/accounts 全表的 platform vs 协议字段双写一致性。
//
// 检查规则（docs/relay-architecture-design.md §3.2）：
//   - 跳过 inbound_protocol/outbound_protocol 为空的行（历史行，按 platform 派生）
//   - 跳过 ResolveX 返回空的 platform（如 lingjing，无通用 inbound_protocol 概念）
//   - 非空字段必须与 ResolveX(stored, platform) 一致
//
// 不一致的行：
//   - 单独打 ERROR 日志（含 id/platform/stored protocol/expected protocol）
//   - 累计计数写入 atomic 全局 counter，供 metric exporter 读取
//
// 错误处理：DB 查询失败立即返回错误，不打 metric（避免在 DB 不可用时误报）。
//
// 由 main.go 在 server 启动前异步调用一次；不阻塞主流程。
func RunProtocolConsistencyCheck(ctx context.Context, db *sql.DB) (*ProtocolConsistencyResult, error) {
	if db == nil {
		return nil, fmt.Errorf("nil sql.DB")
	}
	result := &ProtocolConsistencyResult{}

	if err := checkGroupProtocols(ctx, db, result); err != nil {
		return nil, fmt.Errorf("group protocol check: %w", err)
	}
	if err := checkAccountProtocols(ctx, db, result); err != nil {
		return nil, fmt.Errorf("account protocol check: %w", err)
	}

	// 发布到 atomic counter 供 metric exporter 读取
	groupProtocolMismatchCount.Store(result.GroupsMismatch)
	accountProtocolMismatchCount.Store(result.AccountsMismatch)

	if result.GroupsMismatch > 0 || result.AccountsMismatch > 0 {
		slog.Error("protocol consistency check found mismatches",
			"groups_total", result.GroupsTotal,
			"groups_mismatch", result.GroupsMismatch,
			"accounts_total", result.AccountsTotal,
			"accounts_mismatch", result.AccountsMismatch,
		)
	} else {
		slog.Info("protocol consistency check passed",
			"groups_total", result.GroupsTotal,
			"accounts_total", result.AccountsTotal,
		)
	}

	return result, nil
}

// checkGroupProtocols 扫描 groups 表，验证 platform vs inbound_protocol 一致性。
func checkGroupProtocols(ctx context.Context, db *sql.DB, result *ProtocolConsistencyResult) error {
	rows, err := db.QueryContext(ctx, `
		SELECT id, platform, COALESCE(inbound_protocol, '')
		FROM groups
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id int64
		var platform, storedProtocol string
		if err := rows.Scan(&id, &platform, &storedProtocol); err != nil {
			return err
		}
		result.GroupsTotal++

		// 跳过空 inbound_protocol（历史行，按 platform 派生即可）
		if storedProtocol == "" {
			continue
		}
		expected := domain.ResolveInboundProtocol("", platform)
		// expected 为空表示 platform 没有对应的 inbound_protocol 派生（如 lingjing），跳过
		if expected == "" {
			continue
		}
		if storedProtocol != expected {
			result.GroupsMismatch++
			slog.Error("group protocol mismatch",
				"group_id", id,
				"platform", platform,
				"stored_inbound_protocol", storedProtocol,
				"expected_inbound_protocol", expected,
			)
		}
	}
	return rows.Err()
}

// checkAccountProtocols 扫描 accounts 表，验证 platform vs outbound_protocol 一致性。
func checkAccountProtocols(ctx context.Context, db *sql.DB, result *ProtocolConsistencyResult) error {
	rows, err := db.QueryContext(ctx, `
		SELECT id, platform, COALESCE(outbound_protocol, '')
		FROM accounts
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id int64
		var platform, storedProtocol string
		if err := rows.Scan(&id, &platform, &storedProtocol); err != nil {
			return err
		}
		result.AccountsTotal++

		if storedProtocol == "" {
			continue
		}
		expected := domain.ResolveOutboundProtocol("", platform)
		if expected == "" {
			continue
		}
		if storedProtocol != expected {
			result.AccountsMismatch++
			slog.Error("account protocol mismatch",
				"account_id", id,
				"platform", platform,
				"stored_outbound_protocol", storedProtocol,
				"expected_outbound_protocol", expected,
			)
		}
	}
	return rows.Err()
}
