package service

import "context"

// GetDualBucketStats 返回指定平台最近 days 天的双桶比较统计。
// days 限制在 1–30 之间；platform="" 表示查询所有已知平台。
func (s *SchedulerSnapshotService) GetDualBucketStats(ctx context.Context, platform string, days int) ([]DualBucketDayStats, error) {
	if s.cache == nil {
		return nil, nil
	}
	return s.cache.GetDualBucketStats(ctx, platform, days)
}
