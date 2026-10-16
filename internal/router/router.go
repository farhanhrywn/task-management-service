// Package router wires middleware and routes onto a Gin engine.
package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/internal/middleware"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
)

type Deps struct {
	Logger *slog.Logger
}

func New(d Deps) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.ContextWithFallback = true

	r.Use(
		middleware.RequestID(d.Logger),
		middleware.Recovery(),
		middleware.ErrorHandler(),
	)

	r.NoRoute(func(c *gin.Context) { _ = c.Error(apperror.RouteNotFound()) })
	r.NoMethod(func(c *gin.Context) { _ = c.Error(apperror.MethodNotAllowed()) })

	return r
}
