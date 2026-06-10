//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestCloneAntigravityUsageWithRemaining_Independent 验证拷贝助手返回独立副本，
// 重算 RemainingSeconds，且不改动原始（共享缓存）对象。
func TestCloneAntigravityUsageWithRemaining_Independent(t *testing.T) {
	resetAt := time.Now().Add(2 * time.Hour)
	orig := &UsageInfo{
		Source:   "antigravity",
		FiveHour: &UsageProgress{ResetsAt: &resetAt, RemainingSeconds: 12345},
	}

	clone := cloneAntigravityUsageWithRemaining(orig)
	if clone == orig {
		t.Fatal("expected a distinct UsageInfo pointer")
	}
	if clone.FiveHour == orig.FiveHour {
		t.Fatal("expected a distinct FiveHour pointer")
	}
	if clone.FiveHour.RemainingSeconds == 12345 {
		t.Fatal("expected RemainingSeconds recomputed on the clone")
	}
	if orig.FiveHour.RemainingSeconds != 12345 {
		t.Fatalf("original object mutated: got %d, want 12345", orig.FiveHour.RemainingSeconds)
	}
	if cloneAntigravityUsageWithRemaining(nil) != nil {
		t.Fatal("expected nil clone for nil input")
	}
}

// TestGetAntigravityUsage_CacheHitNoRace 并发命中同一账号缓存，验证：
//  1. -race 下无数据竞争（copy-on-read 生效）；
//  2. 每次命中返回的是副本而非共享指针；
//  3. 共享缓存对象不被任何并发命中改写。
func TestGetAntigravityUsage_CacheHitNoRace(t *testing.T) {
	svc := &AccountUsageService{
		antigravityQuotaFetcher: NewAntigravityQuotaFetcher(nil),
		cache:                   NewUsageCache(),
	}
	account := &Account{
		ID:          1,
		Platform:    PlatformAntigravity,
		Credentials: map[string]any{"access_token": "tok"},
	}

	resetAt := time.Now().Add(2 * time.Hour)
	shared := &UsageInfo{
		Source:   "antigravity",
		FiveHour: &UsageProgress{ResetsAt: &resetAt, RemainingSeconds: 999},
	}
	svc.cache.antigravityCache.Store(account.ID, &antigravityUsageCache{
		usageInfo: shared,
		timestamp: time.Now(),
	})

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			u, err := svc.getAntigravityUsage(context.Background(), account)
			if err != nil {
				t.Errorf("getAntigravityUsage: %v", err)
				return
			}
			if u == shared {
				t.Error("cache hit returned the shared pointer, expected a copy")
			}
			_ = u.FiveHour.RemainingSeconds // 读取，配合 -race 检测
		}()
	}
	wg.Wait()

	if shared.FiveHour.RemainingSeconds != 999 {
		t.Fatalf("shared cache object was mutated: got %d, want 999", shared.FiveHour.RemainingSeconds)
	}
}
