// Package middleware contains the Gin middleware chain.
package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/pkg/ctxkey"
	"github.com/farhanhrywn/task-management-service/pkg/logger"
)

const HeaderRequestID = "X-Request-ID"

// RequestID assigns a UUID v4 to every request, exposes it in the
// X-Request-ID header, and attaches a request-scoped logger to the context so
// logs written by services carry the same request_id.
func RequestID(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.NewString()
		c.Set(ctxkey.RequestID, id)
		c.Header(HeaderRequestID, id)

		ctx := logger.WithContext(c.Request.Context(), base.With(slog.String("request_id", id)))
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
