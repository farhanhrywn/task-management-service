// Package service holds the application use cases.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
)

type TokenIssuer interface {
	Generate(userID uuid.UUID, role string) (string, error)
}

type AuthService struct {
	users      repository.UserRepository
	tokens     TokenIssuer
	bcryptCost int
	// dummyHash is compared against when the email does not exist, so a
	// failed login takes the same time whether or not the account exists.
	dummyHash []byte
}

func NewAuthService(users repository.UserRepository, tokens TokenIssuer, bcryptCost int) *AuthService {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
	return &AuthService{users: users, tokens: tokens, bcryptCost: bcryptCost, dummyHash: dummy}
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.bcryptCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Name:         strings.TrimSpace(in.Name),
		Email:        NormalizeEmail(in.Email),
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.EmailAlreadyExists()
		}
		return nil, apperror.Internal(err)
	}
	return user, nil
}

// Login returns a signed access token. Unknown email and wrong password
// produce the same INVALID_CREDENTIALS error.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.users.FindByEmail(ctx, NormalizeEmail(email))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
			return "", apperror.InvalidCredentials()
		}
		return "", apperror.Internal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", apperror.InvalidCredentials()
	}

	token, err := s.tokens.Generate(user.ID, string(user.Role))
	if err != nil {
		return "", apperror.Internal(err)
	}
	return token, nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
