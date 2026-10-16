package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/ctxkey"
)

type TokenParser interface {
	Parse(token string) (uuid.UUID, string, error)
}

// Auth requires a valid `Authorization: Bearer <token>` header and stores the
// caller's user_id and role in the context.
func Auth(parser TokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			_ = c.Error(apperror.Unauthorized())
			c.Abort()
			return
		}

		userID, role, err := parser.Parse(token)
		if err != nil {
			_ = c.Error(apperror.Unauthorized())
			c.Abort()
			return
		}

		c.Set(ctxkey.UserID, userID.String())
		c.Set(ctxkey.Role, role)
		c.Next()
	}
}

// RequireRole rejects callers whose role differs from role with 403 FORBIDDEN.
// It must run after Auth.
func RequireRole(role domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(ctxkey.Role) != string(role) {
			_ = c.Error(apperror.Forbidden())
			c.Abort()
			return
		}
		c.Next()
	}
}
