package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/ctxkey"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

// ErrorHandler converts the last error pushed with c.Error into the standard
// error envelope. Unknown errors become 500 INTERNAL_ERROR with a generic
// message; their cause is kept in the context for the access logger.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}

		err := c.Errors.Last().Err
		appErr, ok := apperror.As(err)
		if !ok {
			appErr = apperror.Internal(err)
		}
		if appErr.HTTPStatus >= 500 {
			cause := appErr.Err
			if cause == nil {
				cause = appErr
			}
			c.Set(ctxkey.Error, cause)
		}
		response.Error(c, appErr)
	}
}
