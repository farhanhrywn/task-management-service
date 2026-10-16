// Package ctxkey centralizes the keys stored in the Gin context.
package ctxkey

const (
	RequestID = "request_id"
	UserID    = "user_id"
	Role      = "role"

	// Error and Stack carry the internal cause of a 5xx response from the
	// error/recovery middleware to the access logger.
	Error = "internal_error"
	Stack = "panic_stack"
)
