//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// crudRepoStub 记录 Create/Update/Delete/SeedIfNotExists/BulkUpsertMaas 的调用参数，
// 其余方法沿用嵌入接口的 nil 实现（未被以下用例触达）。
type crudRepoStub struct {
	service.ModelPricingRepository

	createdModel *service.DBModelPricing
	createErr    error

	getByIDModel *service.DBModelPricing
	getByIDErr   error
	updatedModel *service.DBModelPricing
	updateErr    error

	deletedID int64
	deleteErr error

	seededModels []*service.DBModelPricing
	seedErr      error

	upsertedMaas []*service.DBModelPricing
	upsertMaasErr error
}

func (s *crudRepoStub) Create(_ context.Context, m *service.DBModelPricing) error {
	s.createdModel = m
	return s.createErr
}

func (s *crudRepoStub) GetByID(_ context.Context, _ int64) (*service.DBModelPricing, error) {
	return s.getByIDModel, s.getByIDErr
}

func (s *crudRepoStub) Update(_ context.Context, m *service.DBModelPricing) error {
	s.updatedModel = m
	return s.updateErr
}

func (s *crudRepoStub) Delete(_ context.Context, id int64) error {
	s.deletedID = id
	return s.deleteErr
}

func (s *crudRepoStub) SeedIfNotExists(_ context.Context, models []*service.DBModelPricing) error {
	s.seededModels = models
	return s.seedErr
}

func (s *crudRepoStub) BulkUpsertMaas(_ context.Context, models []*service.DBModelPricing) error {
	s.upsertedMaas = models
	return s.upsertMaasErr
}

// stubHTTPUpstream 直接用 http.DefaultClient 发起请求，忽略 TLS 指纹伪装参数，
// 用于把 AccountTestService.FetchModelsByConfig 指向 httptest.Server。
type stubHTTPUpstream struct{}

func (stubHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func (stubHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func newAccountTestServiceForUpstreamSync() *service.AccountTestService {
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true // 允许测试用 httptest(http) server
	return service.NewAccountTestService(nil, nil, nil, stubHTTPUpstream{}, cfg, nil)
}

// ---- Create ----

func TestModelPricingHandlerCreate_MissingModelIDReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings", handler.Create)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, repo.createdModel, "校验失败时不应调用 repo.Create")
}

// TestModelPricingHandlerCreate_NormalizesInvalidPricingUnitToToken 锁定
// normalizePricingUnit 的实际行为：非法/未知 pricing_unit 不会被拒绝，而是静默
// 归一化为默认的 token 计价单位（而非返回校验错误）。
func TestModelPricingHandlerCreate_NormalizesInvalidPricingUnitToToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings", handler.Create)

	body := `{"model_id":"custom-model","pricing_unit":"not-a-real-unit"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.createdModel)
	require.Equal(t, service.ModelPricingUnitToken, repo.createdModel.PricingUnit)
}

func TestModelPricingHandlerCreate_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings", handler.Create)

	body := `{"model_id":"gpt-image-2","pricing_unit":"image_generation","output_cost_per_image":0.04}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.createdModel)
	require.Equal(t, "gpt-image-2", repo.createdModel.ModelID)
	require.Equal(t, service.ModelPricingUnitImage, repo.createdModel.PricingUnit)
	require.NotNil(t, repo.createdModel.OutputCostPerImage)
	require.InDelta(t, 0.04, *repo.createdModel.OutputCostPerImage, 1e-9)
	require.True(t, repo.createdModel.IsCustom)
	require.True(t, repo.createdModel.IsEnabled, "未显式传 is_enabled 时默认应为启用")
	require.Equal(t, "chat", repo.createdModel.Mode, "未显式传 mode 时默认应为 chat")
}

// ---- Update ----

func TestModelPricingHandlerUpdate_InvalidIDReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/model-pricings/:id", handler.Update)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-pricings/not-a-number", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestModelPricingHandlerUpdate_NotFoundPropagatesRepoError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{getByIDErr: assertNotFoundErr}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/model-pricings/:id", handler.Update)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-pricings/999", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Nil(t, repo.updatedModel, "GetByID 失败时不应调用 repo.Update")
}

// TestModelPricingHandlerUpdate_OutputCostPerImagePropagatesToRepo 锁定风险表标注的
// 「output_cost_per_image 表单」写入路径：patch 字段必须原样落到 repo.Update 的参数上。
func TestModelPricingHandlerUpdate_OutputCostPerImagePropagatesToRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existing := &service.DBModelPricing{ID: 42, ModelID: "img-model", Mode: "image_generation", PricingUnit: service.ModelPricingUnitImage}
	repo := &crudRepoStub{getByIDModel: existing}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/model-pricings/:id", handler.Update)

	body := `{"output_cost_per_image":0.08}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-pricings/42", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.updatedModel)
	require.NotNil(t, repo.updatedModel.OutputCostPerImage)
	require.InDelta(t, 0.08, *repo.updatedModel.OutputCostPerImage, 1e-9)
	// 未在 patch 中出现的字段必须保持 existing 原值不被清空。
	require.Equal(t, "img-model", repo.updatedModel.ModelID)
}

// ---- Delete ----

func TestModelPricingHandlerDelete_InvalidIDReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.DELETE("/api/v1/admin/model-pricings/:id", handler.Delete)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/model-pricings/0", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestModelPricingHandlerDelete_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.DELETE("/api/v1/admin/model-pricings/:id", handler.Delete)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/model-pricings/7", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 7, repo.deletedID)
}

// ---- SyncFromUpstream ----

func TestModelPricingHandlerSyncFromUpstream_NoAccountTestServiceReturnsInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings/sync-from-upstream", handler.SyncFromUpstream)

	body := `{"base_url":"http://example.com","api_key":"k"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings/sync-from-upstream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Nil(t, repo.seededModels)
}

func TestModelPricingHandlerSyncFromUpstream_SuccessSeedsUnpricedDisabledModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"model-a"},{"id":"model-b"}]}`))
	}))
	defer upstream.Close()

	repo := &crudRepoStub{}
	ats := newAccountTestServiceForUpstreamSync()
	handler := NewModelPricingHandler(nil, repo, nil, nil, ats, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings/sync-from-upstream", handler.SyncFromUpstream)

	body := `{"base_url":"` + upstream.URL + `","api_key":"test-key","provider":"custom-provider"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings/sync-from-upstream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, repo.seededModels, 2)
	for _, m := range repo.seededModels {
		require.Equal(t, "custom-provider", m.Provider)
		require.True(t, m.IsCustom)
		require.False(t, m.IsEnabled, "同步进来的新模型必须默认禁用，等待管理员补价格")
		require.Equal(t, service.ModelPricingStatusUnpriced, m.PricingStatus)
	}
}

func TestModelPricingHandlerSyncFromUpstream_UpstreamErrorReturnsBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	repo := &crudRepoStub{}
	ats := newAccountTestServiceForUpstreamSync()
	handler := NewModelPricingHandler(nil, repo, nil, nil, ats, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings/sync-from-upstream", handler.SyncFromUpstream)

	body := `{"base_url":"` + upstream.URL + `","api_key":"test-key"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings/sync-from-upstream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Nil(t, repo.seededModels)
}

// ---- SyncMaas ----

func TestModelPricingHandlerSyncMaas_MissingSourceReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings/sync-maas", handler.SyncMaas)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings/sync-maas", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestModelPricingHandlerSyncMaas_MissingSourceAndCredentialsReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings/sync-maas", handler.SyncMaas)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings/sync-maas", strings.NewReader(`{"source":"wanjie"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "既未提供 json_data，也未提供 url+access_token，价格不内置必须拒绝")
	require.Nil(t, repo.upsertedMaas)
}

func TestModelPricingHandlerSyncMaas_InvalidJsonDataReturnsBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings/sync-maas", handler.SyncMaas)

	body := `{"source":"wanjie","json_data":"not-json"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings/sync-maas", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Nil(t, repo.upsertedMaas)
}

func TestModelPricingHandlerSyncMaas_SuccessUpsertsParsedModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &crudRepoStub{}
	handler := NewModelPricingHandler(nil, repo, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/model-pricings/sync-maas", handler.SyncMaas)

	jsonData := `{"result":[{"modelName":"maas-model-1","officialProvider":"minimax","modelType":1}]}`
	body := `{"source":"minimax","json_data":` + strconvQuote(jsonData) + `}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/model-pricings/sync-maas", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, repo.upsertedMaas, 1)
	require.Equal(t, "maas-model-1", repo.upsertedMaas[0].ModelID)
}

// strconvQuote 把原始 JSON 字符串安全地编码为可嵌入外层 JSON body 的带引号字符串字面量。
func strconvQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

var assertNotFoundErr = notFoundErr{}

type notFoundErr struct{}

func (notFoundErr) Error() string { return "model pricing not found" }
