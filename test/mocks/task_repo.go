package mocks

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
)

// InMemoryTaskRepo stores tasks in a map. Writes made inside a fake
// transaction are undone on rollback.
type InMemoryTaskRepo struct {
	mu    sync.Mutex
	tasks map[uuid.UUID]domain.Task

	// BeforeCreate, when set, runs at the start of Create. Tests use it to
	// slow a request down or to make the insert fail.
	BeforeCreate func(ctx context.Context, t *domain.Task) error
}

var _ repository.TaskRepository = (*InMemoryTaskRepo)(nil)

func NewInMemoryTaskRepo() *InMemoryTaskRepo {
	return &InMemoryTaskRepo{tasks: make(map[uuid.UUID]domain.Task)}
}

// Count returns the number of tasks, including soft-deleted ones.
func (r *InMemoryTaskRepo) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}

// Put inserts a task directly, bypassing hooks and transactions.
func (r *InMemoryTaskRepo) Put(t domain.Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[t.ID] = t
}

// Snapshot returns a copy of the stored task, including soft-deleted ones.
func (r *InMemoryTaskRepo) Snapshot(id uuid.UUID) (domain.Task, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	return t, ok
}

func (r *InMemoryTaskRepo) Create(ctx context.Context, t *domain.Task) error {
	if r.BeforeCreate != nil {
		if err := r.BeforeCreate(ctx, t); err != nil {
			return err
		}
	}

	r.mu.Lock()
	r.tasks[t.ID] = *t
	r.mu.Unlock()

	id := t.ID
	inTx(ctx, nil, func() {
		r.mu.Lock()
		delete(r.tasks, id)
		r.mu.Unlock()
	})
	return nil
}

func (r *InMemoryTaskRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok || t.DeletedAt.Valid {
		return nil, repository.ErrNotFound
	}
	return &t, nil
}

func (r *InMemoryTaskRepo) Update(ctx context.Context, t *domain.Task) error {
	return r.mutate(ctx, t.ID, func(cur *domain.Task) {
		cur.Title = t.Title
		cur.Description = t.Description
		cur.Status = t.Status
		cur.UpdatedAt = t.UpdatedAt
	})
}

func (r *InMemoryTaskRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.mutate(ctx, id, func(cur *domain.Task) {
		cur.DeletedAt = gorm.DeletedAt{Time: time.Now().UTC(), Valid: true}
	})
}

func (r *InMemoryTaskRepo) List(_ context.Context, f repository.TaskFilter) ([]domain.Task, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var matched []domain.Task
	for _, t := range r.tasks {
		if t.DeletedAt.Valid {
			continue
		}
		if f.OwnerID != nil && t.UserID != *f.OwnerID {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(f.Search)) {
			continue
		}
		matched = append(matched, t)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].CreatedAt.After(matched[j].CreatedAt) })

	total := int64(len(matched))
	start := (f.Page - 1) * f.Limit
	if start >= len(matched) {
		return []domain.Task{}, total, nil
	}
	end := min(start+f.Limit, len(matched))
	return matched[start:end], total, nil
}

// mutate applies fn to a live task and restores the previous value on rollback.
func (r *InMemoryTaskRepo) mutate(ctx context.Context, id uuid.UUID, fn func(*domain.Task)) error {
	r.mu.Lock()
	cur, ok := r.tasks[id]
	if !ok || cur.DeletedAt.Valid {
		r.mu.Unlock()
		return repository.ErrNotFound
	}
	prev := cur
	fn(&cur)
	r.tasks[id] = cur
	r.mu.Unlock()

	inTx(ctx, nil, func() {
		r.mu.Lock()
		r.tasks[id] = prev
		r.mu.Unlock()
	})
	return nil
}
