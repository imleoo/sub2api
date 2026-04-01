package promptanalytics

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler exposes HTTP endpoints for the prompt analytics plugin.
type Handler struct {
	repo Repository
}

// NewHandler creates a new Handler backed by the given Repository.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// GetTopKeywords handles GET /api/v1/admin/prompt-analytics/top-keywords
// Query params:
//   - user_id:  (optional) filter by specific user ID; omit for global
//   - period:   (optional) year-month string, e.g. "2024-01" (defaults to current month)
//   - limit:    (optional) number of keywords to return (default 50, max 200)
func (h *Handler) GetTopKeywords(c *gin.Context) {
	period := c.Query("period")
	if period == "" {
		period = currentPeriod()
	}

	limit := 50
	if s := c.Query("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			if v > 200 {
				v = 200
			}
			limit = v
		}
	}

	userIDStr := c.Query("user_id")
	if userIDStr != "" {
		uid, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid user_id")
			return
		}
		keywords, err := h.repo.GetTopKeywords(c.Request.Context(), uid, period, limit)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "failed to fetch top keywords")
			return
		}
		response.Success(c, gin.H{
			"period":   period,
			"user_id":  uid,
			"keywords": keywords,
		})
		return
	}

	keywords, err := h.repo.GetGlobalTopKeywords(c.Request.Context(), period, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch top keywords")
		return
	}
	response.Success(c, gin.H{
		"period":   period,
		"keywords": keywords,
	})
}
