// Package router wires middleware and routes onto a Gin engine.
package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/handler"
	"github.com/farhanhrywn/task-management-service/internal/middleware"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
)

type Deps struct {
	Logger      *slog.Logger
	TokenParser middleware.TokenParser
	Auth        *handler.AuthHandler
	Task        *handler.TaskHandler
	Team        *handler.TeamHandler
}

func New(d Deps) *gin.Engine {
	handler.RegisterValidators()

	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.ContextWithFallback = true

	r.Use(
		middleware.RequestID(d.Logger),
		middleware.Logger(),
		middleware.Recovery(),
		middleware.ErrorHandler(),
	)

	r.NoRoute(func(c *gin.Context) { _ = c.Error(apperror.RouteNotFound()) })
	r.NoMethod(func(c *gin.Context) { _ = c.Error(apperror.MethodNotAllowed()) })

	auth := r.Group("/auth")
	auth.POST("/register", d.Auth.Register)
	auth.POST("/login", d.Auth.Login)

	superAdmin := middleware.RequireRole(domain.RoleSuperAdmin)

	tasks := r.Group("/tasks", middleware.Auth(d.TokenParser))
	tasks.POST("", d.Task.Create)
	tasks.GET("", d.Task.List)
	tasks.GET("/:id", d.Task.Get)
	tasks.PUT("/:id", d.Task.Update)
	tasks.DELETE("/:id", d.Task.Delete)
	tasks.POST("/:id/assign", superAdmin, d.Task.Assign)

	teams := r.Group("/teams", middleware.Auth(d.TokenParser))
	teams.GET("", d.Team.List)
	teams.GET("/:id/members", d.Team.ListMembers)
	teams.POST("/:id/members", superAdmin, d.Team.AddMember)
	teams.DELETE("/:id/members/:userId", superAdmin, d.Team.RemoveMember)

	return r
}
