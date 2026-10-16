package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/ctxkey"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

// Recovery turns a panic into a 500 INTERNAL_ERROR response. The panic value
// and stack trace are handed to the access logger and never sent to clients.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				c.Set(ctxkey.Error, fmt.Errorf("panic: %v", r))
				c.Set(ctxkey.Stack, string(debug.Stack()))
				if !c.Writer.Written() {
					response.Error(c, apperror.Internal(nil))
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}
