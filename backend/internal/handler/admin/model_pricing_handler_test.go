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

func TestModelPricingHandlerListDefaultsVisibleOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelPricingHandlerRepoStub{}
	handler := NewModelPricingHandler(service.NewModelPricingService(repo), repo, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/model-pricings", handler.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-pricings", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.filter.VisibleOnly)
}

func TestModelPricingHandlerListAllowsVisibleOnlyFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelPricingHandlerRepoStub{}
	handler := NewModelPricingHandler(service.NewModelPricingService(repo), repo, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/model-pricings", handler.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-pricings?visible_only=false", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, repo.filter.VisibleOnly)
}
