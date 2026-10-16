package service_test

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/jwtutil"
	"github.com/farhanhrywn/task-management-service/test/mocks"
)

func newAuthService(t *testing.T) (*service.AuthService, *jwtutil.Manager, *mocks.FakeClock) {
	t.Helper()
	clk := mocks.NewFakeClock(time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC))
	jwt := jwtutil.NewManager("test-secret", 24*time.Hour, clk)
	return service.NewAuthService(mocks.NewInMemoryUserRepo(), jwt, bcrypt.MinCost), jwt, clk
}

func TestRegister_NormalizesEmailAndHashesPassword(t *testing.T) {
	svc, _, _ := newAuthService(t)

	user, err := svc.Register(context.Background(), service.RegisterInput{Name: " Farhan ", Email: " Farhan@Mail.COM ", Password: "secret123"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Email != "farhan@mail.com" || user.Name != "Farhan" || user.Role != domain.RoleUser {
		t.Fatalf("unexpected user: %+v", user)
	}
	if user.PasswordHash == "secret123" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("secret123")) != nil {
		t.Fatal("password must be stored as a bcrypt hash")
	}

	_, err = svc.Register(context.Background(), service.RegisterInput{Name: "Dup", Email: "FARHAN@mail.com", Password: "secret123"})
	if !apperror.Is(err, apperror.CodeEmailAlreadyExists) {
		t.Fatalf("duplicate email: want EMAIL_ALREADY_EXISTS, got %v", err)
	}
}

// UT-11 (login): wrong password and unknown email both return 401
// INVALID_CREDENTIALS; valid credentials return a token carrying sub and role.
func TestLogin(t *testing.T) {
	svc, jwt, _ := newAuthService(t)
	ctx := context.Background()
	user, err := svc.Register(ctx, service.RegisterInput{Name: "Farhan", Email: "farhan@mail.com", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Login(ctx, "farhan@mail.com", "wrong-password"); !apperror.Is(err, apperror.CodeInvalidCredentials) {
		t.Fatalf("wrong password: want INVALID_CREDENTIALS, got %v", err)
	}
	if _, err := svc.Login(ctx, "nobody@mail.com", "secret123"); !apperror.Is(err, apperror.CodeInvalidCredentials) {
		t.Fatalf("unknown email: want INVALID_CREDENTIALS, got %v", err)
	}

	token, err := svc.Login(ctx, "FARHAN@mail.com", "secret123")
	if err != nil {
		t.Fatalf("valid login: %v", err)
	}
	sub, role, err := jwt.Parse(token)
	if err != nil || sub != user.ID || role != string(domain.RoleUser) {
		t.Fatalf("token claims: sub=%s role=%s err=%v", sub, role, err)
	}
}
