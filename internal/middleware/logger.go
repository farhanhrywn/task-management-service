package middleware

import (
	"log/slog"
	"math"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/pkg/ctxkey"
	"github.com/farhanhrywn/task-management-service/pkg/logger"
)

// Logger writes exactly one structured access log per request once the rest
// of the chain has finished. Level follows the status code: <400 INFO,
// 4xx WARN, 5xx ERROR (with the internal error and panic stack, if any).
// Request bodies, headers and tokens are never logged.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Float64("latency_ms", math.Round(float64(latency.Microseconds())/10)/100),
			slog.String("client_ip", c.ClientIP()),
		}
		if userID := c.GetString(ctxkey.UserID); userID != "" {
			attrs = append(attrs, slog.String("user_id", userID))
		}

		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
			if err, ok := c.Get(ctxkey.Error); ok {
				if e, ok := err.(error); ok {
					attrs = append(attrs, slog.String("error", e.Error()))
				}
			}
			if stack := c.GetString(ctxkey.Stack); stack != "" {
				attrs = append(attrs, slog.String("stack", stack))
			}
		case status >= 400:
			level = slog.LevelWarn
		}

		// The request-scoped logger already carries request_id.
		logger.FromContext(c.Request.Context()).LogAttrs(c.Request.Context(), level, "request completed", attrs...)
	}
}
