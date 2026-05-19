// Package main 实现 UsageLog.provider 历史细颗粒回填（Phase 1 P1-1）。
//
// 设计依据：docs/upstream-cost-snapshot.md §2.3 + generic-channel-design.md §5.4 第 5 条。
//
// 关键约束：
//  1. 仅按 account.extra.provider 推导（不读 account.platform）
//  2. 仅回填存活账号关联的历史行；已删账号或未写 extra.provider 的行保持 NULL
//  3. 分批 ≤10000 行 + 批间 sleep 500ms，避免锁表
//  4. 仅 UPDATE provider IS NULL 的行（避免覆盖 Phase 0 P0-5 后已写入的行）
//  5. provider 值经 service.NormalizeProvider 规范化（docs/glossary.md §1.3 别名表）
//
// 调用方式：
//
//	cd backend && go run ./cmd/migrate_provider/ [-dry-run] [-batch-size 10000] [-sleep-ms 500]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "仅打印将要回填的账号清单与影响行数，不执行 UPDATE")
	batchSize := flag.Int("batch-size", 10000, "单批最多更新的 usage_logs 行数")
	sleepMs := flag.Int("sleep-ms", 500, "批次间 sleep 毫秒数，避免锁表")
	flag.Parse()

	if *batchSize <= 0 {
		log.Fatalf("invalid batch-size: %d", *batchSize)
	}
	if *batchSize > 10000 {
		log.Printf("[warn] batch-size=%d 超过文档建议上限 10000，可能引发长事务锁表", *batchSize)
	}

	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	client, sqlDB, err := repository.InitEnt(cfg)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close db: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Step 1：查询所有"存活账号 + extra.provider 非空"的账号清单
	// 使用原生 SQL（避免与 service.AccountRepository 的密钥解密链耦合）
	rows, err := sqlDB.QueryContext(ctx, `
		SELECT id, COALESCE(extra->>'provider', '') AS provider
		FROM accounts
		WHERE extra IS NOT NULL
		  AND extra ? 'provider'
		  AND extra->>'provider' <> ''
		  AND (deleted_at IS NULL)
	`)
	if err != nil {
		log.Fatalf("query accounts with extra.provider failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	type accountProvider struct {
		ID       int64
		Provider string
	}
	var pendings []accountProvider
	for rows.Next() {
		var ap accountProvider
		if err := rows.Scan(&ap.ID, &ap.Provider); err != nil {
			log.Fatalf("scan account row failed: %v", err)
		}
		// 规范化 provider（docs/glossary.md §1.3 别名表）
		ap.Provider = service.NormalizeProvider(ap.Provider)
		if ap.Provider == "" {
			continue
		}
		pendings = append(pendings, ap)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows.Err: %v", err)
	}

	if len(pendings) == 0 {
		log.Printf("no accounts with extra.provider found; nothing to backfill")
		return
	}
	log.Printf("found %d accounts to backfill", len(pendings))

	// Step 2：对每个账号执行批量 UPDATE
	totalUpdated := int64(0)
	startTime := time.Now()
	for i, ap := range pendings {
		log.Printf("[%d/%d] account_id=%d provider=%s ...", i+1, len(pendings), ap.ID, ap.Provider)

		// 单账号循环更新，直到没有 NULL provider 的行
		for {
			if *dryRun {
				var pendingRows int64
				if err := sqlDB.QueryRowContext(ctx, `
					SELECT COUNT(*) FROM usage_logs
					WHERE account_id = $1 AND provider IS NULL
				`, ap.ID).Scan(&pendingRows); err != nil {
					log.Fatalf("dry-run count failed: %v", err)
				}
				log.Printf("  [dry-run] would update %d rows", pendingRows)
				totalUpdated += pendingRows
				break
			}

			result, err := sqlDB.ExecContext(ctx, `
				UPDATE usage_logs
				SET provider = $1
				WHERE id IN (
					SELECT id FROM usage_logs
					WHERE account_id = $2 AND provider IS NULL
					ORDER BY id
					LIMIT $3
				)
			`, ap.Provider, ap.ID, *batchSize)
			if err != nil {
				log.Fatalf("update batch failed: %v", err)
			}
			updated, err := result.RowsAffected()
			if err != nil {
				log.Fatalf("RowsAffected failed: %v", err)
			}
			totalUpdated += updated
			log.Printf("  updated %d rows (running total: %d)", updated, totalUpdated)
			if updated == 0 || updated < int64(*batchSize) {
				// 本账号已无可更新行
				break
			}
			// 批间 sleep，避免锁表
			time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
		}
	}

	elapsed := time.Since(startTime)
	mode := "applied"
	if *dryRun {
		mode = "dry-run"
	}
	log.Printf("%s: %d rows updated across %d accounts in %s",
		mode, totalUpdated, len(pendings), elapsed.Round(time.Second))
	fmt.Printf("done: %s\n", strings.ToUpper(mode))
}
