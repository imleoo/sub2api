package middleware

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const clientRequestIDHeader = "X-Client-Request-ID"
const billRequestIDHeader = "Bill-Request-ID"

// ClientRequestID ensures every request has a unique client_request_id in request.Context().
// It also reads the downstream Bill-Request-ID header for billing reconciliation, falling back to
// the generated UUID when the header is absent or invalid (empty / exceeds 64 chars).
func ClientRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil {
			c.Next()
			return
		}

		if v, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(v) != "" {
			id := strings.TrimSpace(v)
			c.Header(clientRequestIDHeader, id)
			billID := strings.TrimSpace(c.GetHeader(billRequestIDHeader))
			if billID == "" || len(billID) > 64 {
				billID = id
			}
			c.Header(billRequestIDHeader, billID)
			ctx := context.WithValue(c.Request.Context(), ctxkey.BillRequestID, billID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		id := uuid.New().String()
		c.Header(clientRequestIDHeader, id)

		billID := strings.TrimSpace(c.GetHeader(billRequestIDHeader))
		if billID == "" || len(billID) > 64 {
			billID = id
		}
		c.Header(billRequestIDHeader, billID)

		ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, id)
		ctx = context.WithValue(ctx, ctxkey.BillRequestID, billID)
		requestLogger := logger.FromContext(ctx).With(zap.String("client_request_id", id))
		ctx = logger.IntoContext(ctx, requestLogger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
