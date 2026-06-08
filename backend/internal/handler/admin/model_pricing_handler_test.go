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
