// Package handler adapts HTTP requests to service calls.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/ctxkey"
)

// FieldError is one entry of the `details` array of a VALIDATION_ERROR.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

var registerOnce sync.Once

// RegisterValidators makes validation errors report JSON/query field names
// and adds the custom `notblank` rule. Safe to call more than once.
func RegisterValidators() {
	registerOnce.Do(func() {
		v, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			return
		}
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			for _, tag := range []string{"json", "form", "uri"} {
				name := strings.SplitN(f.Tag.Get(tag), ",", 2)[0]
				if name == "-" {
					return ""
				}
				if name != "" {
					return name
				}
			}
			return f.Name
		})
		_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
			return strings.TrimSpace(fl.Field().String()) != ""
		})
	})
}

func bindJSON(c *gin.Context, obj any) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		return toValidationError(err)
	}
	return nil
}

func bindQuery(c *gin.Context, obj any) error {
	if err := c.ShouldBindQuery(obj); err != nil {
		return toValidationError(err)
	}
	return nil
}

func toValidationError(err error) *apperror.AppError {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		details := make([]FieldError, 0, len(ve))
		for _, fe := range ve {
			details = append(details, FieldError{Field: fe.Field(), Message: fieldMessage(fe)})
		}
		return apperror.Validation(details)
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return apperror.Validation([]FieldError{{Field: typeErr.Field, Message: "has an invalid type"}})
	}
	if errors.Is(err, io.EOF) {
		return apperror.Validation([]FieldError{{Field: "body", Message: "request body is required"}})
	}
	var numErr *strconv.NumError
	if errors.As(err, &numErr) {
		return apperror.Validation([]FieldError{{Field: "query", Message: "numeric parameters must be valid integers"}})
	}
	return apperror.Validation([]FieldError{{Field: "body", Message: "malformed JSON"}})
}

func fieldMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "notblank":
		return "must not be blank"
	case "email":
		return "must be a valid email address"
	case "uuid", "uuid4":
		return "must be a valid UUID"
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "min":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("must be at least %s characters", fe.Param())
		}
		return "must be at least " + fe.Param()
	case "max":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("must be at most %s characters", fe.Param())
		}
		return "must be at most " + fe.Param()
	}
	return "is invalid"
}

// parseID parses a UUID path parameter.
func parseID(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, apperror.InvalidID()
	}
	return id, nil
}

// currentActor returns the caller set by the Auth middleware.
func currentActor(c *gin.Context) domain.Actor {
	id, _ := uuid.Parse(c.GetString(ctxkey.UserID))
	return domain.Actor{UserID: id, Role: domain.Role(c.GetString(ctxkey.Role))}
}
