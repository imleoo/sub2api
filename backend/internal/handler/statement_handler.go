package handler

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xlsxreport"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// StatementHandler 用户端月度对账单（Vendor Report）。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
type StatementHandler struct {
	statementService *service.StatementService
}

// NewStatementHandler creates a new StatementHandler.
func NewStatementHandler(statementService *service.StatementService) *StatementHandler {
	return &StatementHandler{statementService: statementService}
}

// GetStatement GET /usage/statement?month=YYYY-MM&timezone=Asia/Shanghai
func (h *StatementHandler) GetStatement(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	month := strings.TrimSpace(c.Query("month"))
	if month == "" {
		response.BadRequest(c, "month is required (YYYY-MM)")
		return
	}
	st, err := h.statementService.GetStatement(c.Request.Context(), subject.UserID, month, strings.TrimSpace(c.Query("timezone")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, st)
}

// GetMonths GET /usage/statement/months
func (h *StatementHandler) GetMonths(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	months, err := h.statementService.ListAvailableMonths(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, months)
}

// Export GET /usage/statement/export?month=YYYY-MM&timezone=...
func (h *StatementHandler) Export(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	month := strings.TrimSpace(c.Query("month"))
	if month == "" {
		// 缺省导出上一个已封账月。
		month = timezone.StartOfMonth(time.Now()).AddDate(0, -1, 0).Format("2006-01")
	}
	st, err := h.statementService.GetStatement(c.Request.Context(), subject.UserID, month, strings.TrimSpace(c.Query("timezone")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	xlsxreport.WriteVendorReportAttachment(c, st)
}
