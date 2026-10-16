package mocks

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
)

// InMemoryTaskLogRepo records task logs; inserts are undone on rollback.
type InMemoryTaskLogRepo struct {
	mu   sync.Mutex
	logs []domain.TaskLog
}

var _ repository.TaskLogRepository = (*InMemoryTaskLogRepo)(nil)

func NewInMemoryTaskLogRepo() *InMemoryTaskLogRepo { return &InMemoryTaskLogRepo{} }

func (r *InMemoryTaskLogRepo) Create(ctx context.Context, l *domain.TaskLog) error {
	r.mu.Lock()
	r.logs = append(r.logs, *l)
	r.mu.Unlock()

	id := l.ID
	inTx(ctx, nil, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for i := range r.logs {
			if r.logs[i].ID == id {
				r.logs = append(r.logs[:i], r.logs[i+1:]...)
				return
			}
		}
	})
	return nil
}

// ForTask returns the logs of one task.
func (r *InMemoryTaskLogRepo) ForTask(taskID uuid.UUID) []domain.TaskLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.TaskLog
	for _, l := range r.logs {
		if l.TaskID == taskID {
			out = append(out, l)
		}
	}
	return out
}

// StubNotifier records calls and returns Err when set.
type StubNotifier struct {
	mu    sync.Mutex
	Err   error
	Calls int
}

func (n *StubNotifier) TaskAssigned(context.Context, domain.Task, uuid.UUID) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Calls++
	return n.Err
}
