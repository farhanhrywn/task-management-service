package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/farhanhrywn/task-management-service/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create inserts the user. A duplicate email surfaces as ErrDuplicate from the
// unique index, which is race-safe unlike a check-then-insert.
func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	return translate(dbFrom(ctx, r.db).Create(u).Error)
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	if err := dbFrom(ctx, r.db).Where("email = ?", email).Take(&u).Error; err != nil {
		return nil, translate(err)
	}
	return &u, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	if err := dbFrom(ctx, r.db).Where("id = ?", id).Take(&u).Error; err != nil {
		return nil, translate(err)
	}
	return &u, nil
}
