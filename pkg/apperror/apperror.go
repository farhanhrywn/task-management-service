// Package apperror defines the application error type and the error catalog.
package apperror

import (
	"errors"
	"net/http"
)

// Error codes exposed to clients.
const (
	CodeValidation            = "VALIDATION_ERROR"
	CodeInvalidID             = "INVALID_ID"
	CodeIdempotencyKeyMissing = "IDEMPOTENCY_KEY_MISSING"
	CodeIdempotencyKeyInvalid = "IDEMPOTENCY_KEY_INVALID"
	CodeUnauthorized          = "UNAUTHORIZED"
	CodeInvalidCredentials    = "INVALID_CREDENTIALS"
	CodeForbidden             = "FORBIDDEN"
	CodeTaskNotFound          = "TASK_NOT_FOUND"
	CodeUserNotFound          = "USER_NOT_FOUND"
	CodeTeamNotFound          = "TEAM_NOT_FOUND"
	CodeMemberNotFound        = "MEMBER_NOT_FOUND"
	CodeRouteNotFound         = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed      = "METHOD_NOT_ALLOWED"
	CodeEmailAlreadyExists    = "EMAIL_ALREADY_EXISTS"
	CodeMemberAlreadyExists   = "MEMBER_ALREADY_EXISTS"
	CodeAlreadyAssigned       = "ALREADY_ASSIGNED"
	CodeIdempotencyInProgress = "IDEMPOTENCY_IN_PROGRESS"
	CodeAssigneeNotInTeam     = "ASSIGNEE_NOT_IN_TEAM"
	CodeInternal              = "INTERNAL_ERROR"
)

// AppError is a known, client-safe error. Err holds the underlying cause and
// is only ever written to logs, never to the response.
type AppError struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    any
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func New(status int, code, message string) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: message}
}

// As reports whether err is (or wraps) an *AppError.
func As(err error) (*AppError, bool) {
	var appErr *AppError
	ok := errors.As(err, &appErr)
	return appErr, ok
}

// Is reports whether err is an *AppError with the given code.
func Is(err error, code string) bool {
	appErr, ok := As(err)
	return ok && appErr.Code == code
}

func Validation(details any) *AppError {
	e := New(http.StatusBadRequest, CodeValidation, "Request validation failed")
	e.Details = details
	return e
}

func InvalidID() *AppError {
	return New(http.StatusBadRequest, CodeInvalidID, "The given id is not a valid UUID")
}

func IdempotencyKeyMissing() *AppError {
	return New(http.StatusBadRequest, CodeIdempotencyKeyMissing, "Idempotency-Key header is required")
}

func IdempotencyKeyInvalid() *AppError {
	return New(http.StatusBadRequest, CodeIdempotencyKeyInvalid, "Idempotency-Key header must be a valid UUID")
}

func Unauthorized() *AppError {
	return New(http.StatusUnauthorized, CodeUnauthorized, "Authentication token is missing, invalid, or expired")
}

func InvalidCredentials() *AppError {
	return New(http.StatusUnauthorized, CodeInvalidCredentials, "Invalid email or password")
}

func Forbidden() *AppError {
	return New(http.StatusForbidden, CodeForbidden, "You do not have access to this resource")
}

func TaskNotFound() *AppError {
	return New(http.StatusNotFound, CodeTaskNotFound, "Task not found")
}

func UserNotFound() *AppError {
	return New(http.StatusNotFound, CodeUserNotFound, "User not found")
}

func TeamNotFound() *AppError {
	return New(http.StatusNotFound, CodeTeamNotFound, "Team not found")
}

func MemberNotFound() *AppError {
	return New(http.StatusNotFound, CodeMemberNotFound, "User is not a member of this team")
}

func RouteNotFound() *AppError {
	return New(http.StatusNotFound, CodeRouteNotFound, "Route not found")
}

func MethodNotAllowed() *AppError {
	return New(http.StatusMethodNotAllowed, CodeMethodNotAllowed, "Method not allowed")
}

func EmailAlreadyExists() *AppError {
	return New(http.StatusConflict, CodeEmailAlreadyExists, "Email is already registered")
}

func MemberAlreadyExists() *AppError {
	return New(http.StatusConflict, CodeMemberAlreadyExists, "User is already a member of this team")
}

func AlreadyAssigned() *AppError {
	return New(http.StatusConflict, CodeAlreadyAssigned, "Task is already assigned to this user")
}

func IdempotencyInProgress() *AppError {
	return New(http.StatusConflict, CodeIdempotencyInProgress, "A request with this Idempotency-Key is still being processed")
}

func AssigneeNotInTeam() *AppError {
	return New(http.StatusUnprocessableEntity, CodeAssigneeNotInTeam, "Assignee does not share a team with the task owner")
}

// Internal wraps an unexpected error. The cause is kept for logging only.
func Internal(err error) *AppError {
	e := New(http.StatusInternalServerError, CodeInternal, "An unexpected error occurred")
	e.Err = err
	return e
}
