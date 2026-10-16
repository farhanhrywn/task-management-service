// Package response writes the standard success and error JSON envelopes.
package response

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/ctxkey"
)

// TimeFormat renders timestamps as YYYY-MM-DDTHH:mm:ssZ.
const TimeFormat = "2006-01-02T15:04:05Z07:00"

// FormatTime converts t to UTC with second precision.
func FormatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(TimeFormat)
}

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type ErrorBody struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id"`
	Details   any    `json:"details"`
}

func Data(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

func List(c *gin.Context, data any, meta Meta) {
	c.JSON(200, gin.H{"data": data, "meta": meta})
}

func Message(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"message": message})
}

// Raw writes pre-serialized JSON bytes, used to replay idempotent responses
// byte for byte.
func Raw(c *gin.Context, status int, body []byte) {
	c.Data(status, "application/json; charset=utf-8", body)
}

// Error writes the standard error envelope. Only client-safe fields of the
// AppError are serialized; the wrapped cause never leaves the server.
func Error(c *gin.Context, err *apperror.AppError) {
	c.AbortWithStatusJSON(err.HTTPStatus, ErrorBody{
		Status:    err.HTTPStatus,
		Code:      err.Code,
		Message:   err.Message,
		Timestamp: FormatTime(time.Now()),
		RequestID: c.GetString(ctxkey.RequestID),
		Details:   err.Details,
	})
}
