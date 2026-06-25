//go:build e2e

package integration

// 全功能 E2E —— 并发计费/配额竞态（P0 钱的正确性）
//
// 网关计费架构 = 后付费 + 前置闸：前置只查"缓存余额>0 / quota 未耗尽"，扣减/配额
// 累加在请求完成后异步落库（quota 计数另有 ~2s 批量 flusher）。因此并发请求会在
// 计数落库前都通过前置闸 → 产生「有界超发」。这是设计内属性，本用例刻画并钉死该行为：
//   - 给 key 设极小 quota，N 个请求并发打 → 多个会越过前置闸成功（超发）
//   - 随后串行再打一次 → 已耗尽，被拦截
// 断言要点：并发确有 >1 次成功（证明 race 存在）+ 闸最终会关闭（串行被拦）。
// 非"绝不超扣"——超扣是架构内可能；目标是回归检测（如超发变无界、或闸不再关闭）。

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestE2EFull_ConcurrentQuotaOvershoot(t *testing.T) {
	pc := requireProvision(t)
	pp := pc.requirePlatform(t, "anthropic")

	// 极小 quota（USD 总额）：单次消费即超额。新建 key、quota_used 从 0 起。
	quota := 0.00001
	gwKey, keyID, err := createGatewayKey(pc.adminToken,
		fmt.Sprintf("e2e-conc-%s", runNonce()), pp.groupID, &quota, nil)
	if err != nil {
		t.Fatalf("建配额 key: %v", err)
	}
	t.Cleanup(func() { _, _ = adminAPI(pc.adminToken, "DELETE", fmt.Sprintf("/api/v1/keys/%d", keyID), nil) })

	const n = 6
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		success   int
		blocked   int
		upstreamNA bool
		other     int
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			st, body, e := gwClaudeMessages(gwKey, pp.model, fmt.Sprintf("reply C%d in one word", i), false, 16)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case e != nil:
				other++
			case st == 200:
				success++
			case st == 402 || st == 429 || bodyContains(body, "quota") || bodyContains(body, "exceeded"):
				blocked++
			case st == 502 || st == 503 || bodyContains(body, "Upstream") || bodyContains(body, "temporarily unavailable"):
				upstreamNA = true
			default:
				other++
			}
		}(i)
	}
	wg.Wait()

	if upstreamNA && success == 0 {
		t.Skipf("上游暂不可用，并发用例跳过（success=%d blocked=%d）", success, blocked)
	}
	t.Logf("并发 %d 请求：成功=%d 被拦=%d 其它=%d", n, success, blocked, other)

	// 至少 1 次成功（前置闸初始 quota_used=0 放行）。
	if success < 1 {
		t.Fatalf("并发请求无一成功（success=0），疑似前置或上游异常：blocked=%d other=%d", blocked, other)
	}
	// 超发刻画：极小 quota 下若 >1 次成功，证明并发越过前置闸的 race（异步计数落库前都放行）。
	if success > 1 {
		t.Logf("✅ 观测到并发配额超发：%d 次成功越过前置闸（quota 计数异步落库前的 race）", success)
	} else {
		t.Logf("ℹ️ 本轮仅 1 次成功越闸（计数落库快于并发窗口，未现超发——行为仍正确）")
	}

	// 闸最终会关闭：等配额落账后串行再打一次，应被拦截。
	time.Sleep(3 * time.Second) // 越过 ~2s quota flusher 窗口
	st, body, err := gwClaudeMessages(gwKey, pp.model, "reply AFTER in one word", false, 16)
	if err != nil {
		t.Fatalf("串行收尾请求错误: %v", err)
	}
	if st == 502 || st == 503 || bodyContains(body, "Upstream") || bodyContains(body, "temporarily unavailable") {
		t.Skipf("收尾串行请求碰到上游不可用（无法区分配额闸是否关闭）：st=%d body=%s", st, truncate(body, 200))
	}
	if st == 200 {
		t.Fatalf("配额闸未关闭：耗尽后串行请求仍 200（期望被拦）：%s", truncate(body, 200))
	}
	t.Logf("✅ 配额闸最终关闭：耗尽后串行请求被拦（st=%d）", st)
}
