package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/dto"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

type AuthService interface {
	Register(ctx context.Context, in service.RegisterInput) (*domain.User, error)
	Login(ctx context.Context, email, password string) (string, error)
}

type AuthHandler struct {
	svc      AuthService
	tokenTTL time.Duration
}

func NewAuthHandler(svc AuthService, tokenTTL time.Duration) *AuthHandler {
	return &AuthHandler{svc: svc, tokenTTL: tokenTTL}
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := bindJSON(c, &req); err != nil {
		_ = c.Error(err)
		return
	}

	user, err := h.svc.Register(c.Request.Context(), service.RegisterInput{
		Name: req.Name, Email: req.Email, Password: req.Password,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Data(c, http.StatusCreated, dto.NewUserResponse(user))
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := bindJSON(c, &req); err != nil {
		_ = c.Error(err)
		return
	}

	token, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Data(c, http.StatusOK, dto.TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(h.tokenTTL.Seconds()),
	})
}
