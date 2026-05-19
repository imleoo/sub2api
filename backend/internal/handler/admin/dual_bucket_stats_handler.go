package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// DualBucketStatsHandler 提供双桶比较监控统计 API（P5-5）。
type DualBucketStatsHandler struct {
	scheduler *service.SchedulerSnapshotService
}

func NewDualBucketStatsHandler(scheduler *service.SchedulerSnapshotService) *DualBucketStatsHandler {
	return &DualBucketStatsHandler{scheduler: scheduler}
}

type dualBucketDayDTO struct {
	Date        string  `json:"date"`
	Total       int64   `json:"total"`
	Diverged    int64   `json:"diverged"`
	DivergenceRate float64 `json:"divergence_rate"` // 0–1，0 表示 total=0
}

// GetStats 返回指定平台最近 N 天的双桶比较统计。
// GET /api/v1/admin/scheduler/dual-bucket-stats?platform=anthropic&days=7
func (h *DualBucketStatsHandler) GetStats(c *gin.Context) {
	if h.scheduler == nil {
		response.Error(c, http.StatusServiceUnavailable, "Scheduler not available")
		return
	}

	platform := strings.TrimSpace(c.Query("platform"))
	if platform == "" {
		response.BadRequest(c, "platform is required")
		return
	}

	days := 7
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 30 {
			days = n
		}
	}

	stats, err := h.scheduler.GetDualBucketStats(c.Request.Context(), platform, days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	items := make([]dualBucketDayDTO, 0, len(stats))
	for _, s := range stats {
		rate := 0.0
		if s.Total > 0 {
			rate = float64(s.Diverged) / float64(s.Total)
		}
		items = append(items, dualBucketDayDTO{
			Date:           s.Date,
			Total:          s.Total,
			Diverged:       s.Diverged,
			DivergenceRate: rate,
		})
	}
	response.Success(c, gin.H{"stats": items, "platform": platform, "days": days})
}
