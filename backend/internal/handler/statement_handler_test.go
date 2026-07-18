//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- fakes（实现 service 层端口，供真实 StatementService 使用） ----

type stmtHandlerFakeRepo struct{}

func (stmtHandlerFakeRepo) ListDeposits(ctx context.Context, userID int64, start, end time.Time) ([]service.StatementCashflow, error) {
	return []service.StatementCashflow{{Time: start, Amount: 5, Note: "order", Source: "payment_order"}}, nil
}

func (stmtHandlerFakeRepo) ListWithdrawals(ctx context.Context, userID int64, start, end time.Time) ([]service.StatementCashflow, error) {
	return nil, nil
}

func (stmtHandlerFakeRepo) ListCredits(ctx context.Context, userID int64, start, end time.Time) ([]service.StatementCashflow, error) {
	return nil, nil
}

func (stmtHandlerFakeRepo) GetDailyUtilisation(ctx context.Context, userID int64, start, end time.Time, tzName string) ([]service.StatementDailyUsage, error) {
	return nil, nil
}

func (stmtHandlerFakeRepo) SumNetChange(ctx context.Context, userID int64, start, end time.Time) (*service.StatementNetChange, error) {
	return &service.StatementNetChange{}, nil
}

type stmtHandlerFakeSnapRepo struct{}

func (stmtHandlerFakeSnapRepo) Upsert(ctx context.Context, snap *service.BalanceSnapshotRecord) error {
	return nil
}

func (stmtHandlerFakeSnapRepo) GetByUserPeriod(ctx context.Context, userID int64, period string) (*service.BalanceSnapshotRecord, error) {
	return nil, nil
}

func (stmtHandlerFakeSnapRepo) ListPeriodsByUser(ctx context.Context, userID int64) ([]string, error) {
	return nil, nil
}

type stmtHandlerFakeUserRepo struct {
	service.UserRepository
}

func (stmtHandlerFakeUserRepo) GetByID(ctx context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Email: "stmt@test.com", Balance: 10, CreatedAt: time.Now().AddDate(0, -2, 0)}, nil
}

func (stmtHandlerFakeUserRepo) List(ctx context.Context, params pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{Pages: 1}, nil
}

func newStatementTestRouter(t *testing.T, withAuth bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := service.NewStatementService(stmtHandlerFakeRepo{}, stmtHandlerFakeSnapRepo{}, nil, stmtHandlerFakeUserRepo{})
	h := NewStatementHandler(svc)

	r := gin.New()
	if withAuth {
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		})
	}
	usage := r.Group("/api/v1/usage")
	usage.GET("/statement", h.GetStatement)
	usage.GET("/statement/months", h.GetMonths)
	usage.GET("/statement/export", h.Export)
	// 模拟真实路由并存的 /:id，验证静态段优先。
	usage.GET("/:id", func(c *gin.Context) { c.String(http.StatusTeapot, "byid") })
	return r
}

func doStatementReq(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestStatementHandler_Unauthorized(t *testing.T) {
	r := newStatementTestRouter(t, false)
	for _, path := range []string{
		"/api/v1/usage/statement?month=2026-06",
		"/api/v1/usage/statement/months",
		"/api/v1/usage/statement/export",
	} {
		w := doStatementReq(r, path)
		assert.Equal(t, http.StatusUnauthorized, w.Code, path)
	}
}

func TestStatementHandler_MonthRequired(t *testing.T) {
	r := newStatementTestRouter(t, true)
	w := doStatementReq(r, "/api/v1/usage/statement")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStatementHandler_GetStatementOK(t *testing.T) {
	r := newStatementTestRouter(t, true)
	month := time.Now().AddDate(0, -1, 0).Format("2006-01")
	w := doStatementReq(r, "/api/v1/usage/statement?month="+month)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"period":"`+month+`"`)
	assert.Contains(t, w.Body.String(), `"rows"`)
}

func TestStatementHandler_StaticSegmentBeatsIDParam(t *testing.T) {
	// GET /usage/statement 不得落入 /usage/:id。
	r := newStatementTestRouter(t, true)
	w := doStatementReq(r, "/api/v1/usage/statement?month=2026-06")
	assert.NotEqual(t, http.StatusTeapot, w.Code)
}

func TestStatementHandler_ExportHeaders(t *testing.T) {
	r := newStatementTestRouter(t, true)

	// 上月（已封账）：无 -partial 后缀。
	month := time.Now().AddDate(0, -1, 0).Format("2006-01")
	w := doStatementReq(r, "/api/v1/usage/statement/export?month="+month)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "statement-stmt@test.com-"+month+".xlsx")
	assert.NotContains(t, w.Header().Get("Content-Disposition"), "-partial")
	assert.NotEmpty(t, w.Body.Bytes())

	// 当月（未封账）：带 -partial。
	current := time.Now().Format("2006-01")
	w = doStatementReq(r, "/api/v1/usage/statement/export?month="+current)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "-partial")
}

func TestStatementHandler_MonthsOK(t *testing.T) {
	r := newStatementTestRouter(t, true)
	w := doStatementReq(r, "/api/v1/usage/statement/months")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), time.Now().Format("2006-01"))
}
