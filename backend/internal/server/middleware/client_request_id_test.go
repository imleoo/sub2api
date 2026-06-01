package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClientRequestIDGeneratesAndExposesID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Body.String())
	require.Equal(t, w.Body.String(), w.Header().Get(clientRequestIDHeader))
}

func TestClientRequestIDPreservesExistingContextID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "existing-client-request-id"))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "existing-client-request-id", w.Body.String())
	require.Equal(t, "existing-client-request-id", w.Header().Get(clientRequestIDHeader))
}

// --- BillRequestID tests ---

func TestBillRequestIDUpstreamValueIsEchoedBack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		billID, _ := c.Request.Context().Value(ctxkey.BillRequestID).(string)
		c.String(http.StatusOK, billID)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(billRequestIDHeader, "downstream-order-abc123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "downstream-order-abc123", w.Body.String())
	require.Equal(t, "downstream-order-abc123", w.Header().Get(billRequestIDHeader))
}

func TestBillRequestIDFallsBackToClientRequestIDWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		clientID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		billID, _ := c.Request.Context().Value(ctxkey.BillRequestID).(string)
		require.Equal(t, clientID, billID, "bill_request_id should equal client_request_id when header absent")
		c.String(http.StatusOK, billID)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Body.String())
	require.Equal(t, w.Body.String(), w.Header().Get(billRequestIDHeader))
}

func TestBillRequestIDFallsBackWhenValueExceedsMaxLen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		clientID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		billID, _ := c.Request.Context().Value(ctxkey.BillRequestID).(string)
		require.Equal(t, clientID, billID, "should fall back to client_request_id for oversized header")
		c.String(http.StatusOK, billID)
	})

	oversized := make([]byte, 65)
	for i := range oversized {
		oversized[i] = 'x'
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(billRequestIDHeader, string(oversized))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Body.String())
	require.Len(t, w.Body.String(), 36, "fallback should be a UUID")
}

func TestBillRequestIDNotOverwrittenByUpstreamXRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		// Simulate what gateway service does: overwrite X-Request-Id with upstream value.
		c.Header("X-Request-Id", "upstream-id-from-ai-provider")
		billID, _ := c.Request.Context().Value(ctxkey.BillRequestID).(string)
		c.String(http.StatusOK, billID)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(billRequestIDHeader, "my-order-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "my-order-id", w.Body.String(), "Bill-Request-ID must not be overwritten by X-Request-Id manipulation")
	require.Equal(t, "my-order-id", w.Header().Get(billRequestIDHeader))
	require.Equal(t, "upstream-id-from-ai-provider", w.Header().Get("X-Request-Id"))
}
