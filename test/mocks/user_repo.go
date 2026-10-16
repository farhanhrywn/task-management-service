package mocks

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
)

// InMemoryUserRepo enforces a unique email like the users table.
type InMemoryUserRepo struct {
	mu    sync.Mutex
	users map[uuid.UUID]domain.User
}

var _ repository.UserRepository = (*InMemoryUserRepo)(nil)

func NewInMemoryUserRepo() *InMemoryUserRepo {
	return &InMemoryUserRepo{users: make(map[uuid.UUID]domain.User)}
}

func (r *InMemoryUserRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if existing.Email == u.Email {
			return repository.ErrDuplicate
		}
	}
	r.users[u.ID] = *u
	return nil
}

func (r *InMemoryUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.Email == email {
			return &u, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *InMemoryUserRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}
