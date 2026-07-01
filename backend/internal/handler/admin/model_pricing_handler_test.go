//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelPricingHandlerRepoStub struct {
	service.ModelPricingRepository
	filter service.ModelPricingListFilter
}

func (s *modelPricingHandlerRepoStub) List(ctx context.Context, filter service.ModelPricingListFilter) ([]*service.DBModelPricing, int, error) {
	s.filter = filter
	return []*service.DBModelPricing{}, 0, nil
}

// 后台默认改为「等于用户模型广场」口径（VisibleOnly=true + 广场可见集），由 ModelRoutingService 提供。
// 当 modelRouting 不可用时（此处注入 nil）退回全集（VisibleOnly=false），避免后台空白——本测试锁该兜底。
func TestModelPricingHandlerListFallsBackToFullSetWithoutRouting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelPricingHandlerRepoStub{}
	handler := NewModelPricingHandler(service.NewModelPricingService(repo), repo, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/model-pricings", handler.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-pricings", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, repo.filter.VisibleOnly)
}

// TestModelPricingHandlerListForWhitelistSkipsRoutableFilter 验证 for_whitelist=true 时跳过
// 广场可路由集交集过滤（修复 ModelWhitelistSelector.vue 搜不到「已同步定价数据但还没被任何
// 账号引用过」的新模型的循环依赖 bug，见 claudedocs/待办任务列表.md）。
// modelRouting 用 accountRepo=nil 构造：OperatorRoutableModelInfos 在这种情况下返回
// ([]ModelInfo{}, nil)（无错误、空集），足以让 hasRouting=true 而 routable 集合为空——
// 正好模拟"新模型完全不在可路由集合里"的场景。
func TestModelPricingHandlerListForWhitelistSkipsRoutableFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelPricingHandlerRepoStub{}
	mr := service.NewModelRoutingService(nil, nil, nil)
	handler := NewModelPricingHandler(service.NewModelPricingService(repo), repo, nil, nil, nil, mr)
	router := gin.New()
	router.GET("/api/v1/admin/model-pricings", handler.List)

	t.Run("默认（模型定价管理页）仍套广场口径", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-pricings", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.True(t, repo.filter.VisibleOnly, "不带 for_whitelist 时应保持广场可见集过滤（后台=广场口径）")
	})

	t.Run("for_whitelist=true 跳过广场口径", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-pricings?for_whitelist=true", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.False(t, repo.filter.VisibleOnly, "for_whitelist=true 时应跳过广场可路由集过滤，否则新模型永远搜不到")
	})
}

func TestComputePricingHealth(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	routable := map[string]struct{}{"m-routable": {}}

	cases := []struct {
		name string
		m    *service.DBModelPricing
		want string
	}{
		{"unit_contamination", &service.DBModelPricing{Mode: "chat", OutputCostPerImage: f(0.001)}, pricingHealthUnitContaminated},
		{"missing_token", &service.DBModelPricing{Mode: "chat"}, pricingHealthMissingPricing},
		{"missing_visual", &service.DBModelPricing{Mode: "image_generation"}, pricingHealthMissingPricing},
		{"orphan", &service.DBModelPricing{ModelID: "m-x", Mode: "chat", InputCostPerToken: f(1e-6), IsEnabled: true}, pricingHealthOrphan},
		{"ok_routable", &service.DBModelPricing{ModelID: "m-routable", Mode: "chat", InputCostPerToken: f(1e-6), IsEnabled: true}, pricingHealthOK},
		{"ok_disabled_not_orphan", &service.DBModelPricing{ModelID: "m-y", Mode: "chat", InputCostPerToken: f(1e-6), IsEnabled: false}, pricingHealthOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, computePricingHealth(c.m, routable))
		})
	}
}
