package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xlsxreport"

	"github.com/gin-gonic/gin"
)

// ExportStatement GET /admin/users/:id/statement/export?month=YYYY-MM&timezone=...
// 导出指定用户的月度对账单 xlsx（Vendor Report 模板）。
//
// zhiguofan fork-only: 月度对账（功能 45）。
func (h *UserHandler) ExportStatement(c *gin.Context) {
	if h.statementService == nil {
		response.InternalError(c, "Statement service not available")
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	month := strings.TrimSpace(c.Query("month"))
	if month == "" {
		// 缺省导出上一个已封账月。
		month = timezone.StartOfMonth(time.Now()).AddDate(0, -1, 0).Format("2006-01")
	}
	st, err := h.statementService.GetStatement(c.Request.Context(), userID, month, strings.TrimSpace(c.Query("timezone")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	xlsxreport.WriteVendorReportAttachment(c, st)
}
