package service

import (
	"context"
	"log/slog"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// platformToProtocol 将 platform 派生为对应的 outbound_protocol（双桶影子查询用）。
// 返回 "" 表示该 platform 不参与双桶比较（如 lingjing）。
func platformToProtocol(platform string) string {
	switch platform {
	case PlatformAnthropic:
		return domain.ProtocolAnthropicMessages
	case PlatformOpenAI:
		return domain.ProtocolOpenAIChat
	case PlatformGemini:
		return domain.ProtocolGeminiV1Beta
	case PlatformAntigravity:
		return domain.ProtocolAnthropicMessages
	case PlatformLingjing, PlatformGeneric:
		// lingjing/generic 不参与双桶比较（lingjing 是异步任务；generic 通过 Endpoint 实体路由）
		return ""
	default:
		return ""
	}
}

// runDualBucketShadow 在独立 goroutine 中执行协议维度的影子查询，与 platform 维度结果比较，
// 将分歧以结构化日志埋点（离线 job 按 scheduler.dual_bucket.diverged=true 聚合每日一致率）。
//
// 此函数不阻塞调用方，使用 detached context 避免主请求取消影响埋点。
func (s *SchedulerSnapshotService) runDualBucketShadow(
	platform string,
	groupID int64,
	platformAccounts []Account,
) {
	protocol := platformToProtocol(platform)
	if protocol == "" {
		return // lingjing 或未知 platform，跳过
	}

	go func() {
		ctx := context.Background()
		var protoAccounts []Account
		var err error

		if groupID > 0 {
			protoAccounts, err = s.accountRepo.ListSchedulableByGroupIDAndOutboundProtocol(ctx, groupID, protocol)
		} else if s.isRunModeSimple() {
			protoAccounts, err = s.accountRepo.ListSchedulableByOutboundProtocol(ctx, protocol)
		} else {
			protoAccounts, err = s.accountRepo.ListSchedulableUngroupedByOutboundProtocol(ctx, protocol)
		}
		if err != nil {
			slog.Warn("scheduler.dual_bucket shadow query failed",
				"platform", platform,
				"protocol", protocol,
				"group_id", groupID,
				"err", err,
			)
			return
		}

		oldIDs := extractSortedIDs(platformAccounts)
		newIDs := extractSortedIDs(protoAccounts)
		diverged := !equalInt64Slices(oldIDs, newIDs)

		slog.Info("scheduler.dual_bucket comparison",
			"platform", platform,
			"protocol", protocol,
			"group_id", groupID,
			"platform_count", len(oldIDs),
			"protocol_count", len(newIDs),
			"diverged", diverged,
		)

		if diverged {
			// 记录分歧账号 ID，用于离线排查
			added, removed := diffIDSlices(oldIDs, newIDs)
			slog.Warn("scheduler.dual_bucket divergence detected",
				"platform", platform,
				"protocol", protocol,
				"group_id", groupID,
				"added_ids", added,   // 仅在 protocol 桶有，platform 桶无
				"removed_ids", removed, // 仅在 platform 桶有，protocol 桶无
			)
		}
	}()
}

func extractSortedIDs(accounts []Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func equalInt64Slices(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffIDSlices 返回 (仅在 b 有的 / 仅在 a 有的)。a、b 均已排序。
func diffIDSlices(a, b []int64) (added, removed []int64) {
	aSet := make(map[int64]struct{}, len(a))
	bSet := make(map[int64]struct{}, len(b))
	for _, v := range a {
		aSet[v] = struct{}{}
	}
	for _, v := range b {
		bSet[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := aSet[v]; !ok {
			added = append(added, v)
		}
	}
	for _, v := range a {
		if _, ok := bSet[v]; !ok {
			removed = append(removed, v)
		}
	}
	return added, removed
}
