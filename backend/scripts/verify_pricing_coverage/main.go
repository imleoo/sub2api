// SSOT 重构 PR-3：verify_pricing_coverage CLI 工具。
//
// 用途：在 PR-6 catalog 切流之前的发布门禁，检验 DB model_pricings 表是否包含
// 所有「计费路径会用到的模型 ID」。任何 miss 退出码 1，CI 接入后阻止有缺口的部署。
//
// 检查集合：
//  1. BootstrapPricingSeedModelIDs()：20 条 fallback + 灵境 seed
//  2. 家族锚点：10 个 Claude 家族至少各有一条匹配 of model_id
//  3. (可选) 最近 30 天 usage_logs 中实际调用过的 model_id —— 通过
//     -with-usage 开关启用
//
// 用法：
//
//	cd backend && DATABASE_URL=postgres://... go run ./scripts/verify_pricing_coverage
//	exit 0 = 全覆盖；exit 1 = miss；exit 2 = DB 连接 / 查询失败
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const exitDBError = 2

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN; 默认读 DATABASE_URL env")
	withUsage := flag.Bool("with-usage", false, "同时检查最近 30 天 usage_logs 中调用过的 model_id")
	verbose := flag.Bool("verbose", false, "输出全部已覆盖模型 ID")
	flag.Parse()

	if strings.TrimSpace(*dsn) == "" {
		fmt.Fprintln(os.Stderr, "ERROR: -dsn or DATABASE_URL is required")
		os.Exit(exitDBError)
	}

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: open db: %v\n", err)
		os.Exit(exitDBError)
	}
	defer func() { _ = db.Close() }()
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: ping db: %v\n", err)
		os.Exit(exitDBError)
	}

	// 收集 DB 中已存在的 model_id 集合（is_enabled 不约束，包含 disabled 行也算"覆盖"）
	existing, err := loadExistingModelIDs(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: load existing model_ids: %v\n", err)
		os.Exit(exitDBError)
	}

	// 期望集合 = bootstrap seeds + 10 个家族锚点
	expected := map[string]string{} // model_id → 来源标签
	for _, id := range service.BootstrapPricingSeedModelIDs() {
		expected[strings.ToLower(id)] = "bootstrap"
	}
	for _, id := range familyAnchorRequirements() {
		expected[strings.ToLower(id)] = "family-anchor"
	}

	// 可选：usage_logs 最近 30 天
	if *withUsage {
		usageIDs, err := loadRecentUsageModelIDs(db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: load usage model_ids: %v\n", err)
			os.Exit(exitDBError)
		}
		for _, id := range usageIDs {
			id = strings.ToLower(strings.TrimSpace(id))
			if id == "" {
				continue
			}
			if _, ok := expected[id]; !ok {
				expected[id] = "usage-30d"
			}
		}
	}

	// diff
	var missing []string
	var covered []string
	for id, source := range expected {
		if _, ok := existing[id]; ok {
			covered = append(covered, fmt.Sprintf("%s [%s]", id, source))
			continue
		}
		missing = append(missing, fmt.Sprintf("%s [%s]", id, source))
	}
	sort.Strings(missing)
	sort.Strings(covered)

	if *verbose && len(covered) > 0 {
		fmt.Printf("✓ COVERED (%d):\n", len(covered))
		for _, line := range covered {
			fmt.Printf("  ✓ %s\n", line)
		}
	}

	if len(missing) == 0 {
		fmt.Printf("OK: %d expected models all present in model_pricings\n", len(expected))
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "FAIL: %d expected models missing from model_pricings:\n", len(missing))
	for _, line := range missing {
		fmt.Fprintf(os.Stderr, "  ✗ %s\n", line)
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run RunBootstrapSeed during service startup, or manually seed via admin API.")
	os.Exit(1)
}

// loadExistingModelIDs 从 model_pricings 读出全部 model_id（小写归一化）。
func loadExistingModelIDs(db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.Query(`SELECT model_id FROM model_pricings`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]struct{}, 256)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
	}
	return out, rows.Err()
}

// loadRecentUsageModelIDs 从 usage_logs 读最近 30 天调用过的全部 model_id。
// 注意：usage_logs 表可能很大；为避免长查询，只 SELECT DISTINCT 并加时间过滤。
func loadRecentUsageModelIDs(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT model
		FROM usage_logs
		WHERE created_at > NOW() - INTERVAL '30 days'
		  AND COALESCE(model, '') <> ''
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// familyAnchorRequirements 列出 matchByModelFamily 命中链所必需的 10 个家族锚点 model_id。
// 任何缺失都会让 Phase3 fuzzy 塌成 nil。这里给出每个家族一个代表 ID
// （任一日期变体均可，Phase3 子串匹配是 contains，所以列锚点 LiteLLM 命名形式即可）。
func familyAnchorRequirements() []string {
	return []string{
		// opus 系列
		"claude-opus-4.7",
		"claude-opus-4.6",
		"claude-opus-4.5",
		"claude-3-opus", // opus-4 family 通过 claude-3-opus 作 fallback
		// sonnet 系列
		"claude-sonnet-4.5",
		"claude-sonnet-4",
		"claude-3-5-sonnet",
		"claude-3-sonnet",
		// haiku 系列
		"claude-3-5-haiku",
		"claude-3-haiku",
	}
}
