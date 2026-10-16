package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/clock"
)

type TaskInput struct {
	Title       string
	Description *string
	Status      domain.TaskStatus
}

type TaskService struct {
	tasks repository.TaskRepository
	clock clock.Clock
}

func NewTaskService(tasks repository.TaskRepository, clk clock.Clock) *TaskService {
	return &TaskService{tasks: tasks, clock: clk}
}

func (s *TaskService) Create(ctx context.Context, ownerID uuid.UUID, in TaskInput) (*domain.Task, error) {
	task := NewTask(ownerID, in, s.clock.Now())
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, apperror.Internal(err)
	}
	return task, nil
}

func (s *TaskService) Get(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Task, error) {
	return s.findAccessible(ctx, actor, id)
}

// Update replaces title, description and status. assignee_id is not editable here.
func (s *TaskService) Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in TaskInput) (*domain.Task, error) {
	task, err := s.findAccessible(ctx, actor, id)
	if err != nil {
		return nil, err
	}

	task.Title = strings.TrimSpace(in.Title)
	task.Description = normalizeDescription(in.Description)
	task.Status = in.Status
	task.UpdatedAt = s.clock.Now()

	if err := s.tasks.Update(ctx, task); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.TaskNotFound()
		}
		return nil, apperror.Internal(err)
	}
	return task, nil
}

func (s *TaskService) Delete(ctx context.Context, actor domain.Actor, id uuid.UUID) error {
	if _, err := s.findAccessible(ctx, actor, id); err != nil {
		return err
	}
	if err := s.tasks.SoftDelete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperror.TaskNotFound()
		}
		return apperror.Internal(err)
	}
	return nil
}

type ListInput struct {
	Status domain.TaskStatus
	Search string
	Page   int
	Limit  int
}

// List returns the caller's tasks, or every task for the super admin,
// newest first.
func (s *TaskService) List(ctx context.Context, actor domain.Actor, in ListInput) ([]domain.Task, int64, error) {
	filter := repository.TaskFilter{
		Status: in.Status,
		Search: strings.TrimSpace(in.Search),
		Page:   in.Page,
		Limit:  in.Limit,
	}
	if !actor.IsSuperAdmin() {
		filter.OwnerID = &actor.UserID
	}

	tasks, total, err := s.tasks.List(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return tasks, total, nil
}

// findAccessible applies the access rule: missing or deleted → 404,
// not owner and not super admin → 403.
func (s *TaskService) findAccessible(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Task, error) {
	task, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.TaskNotFound()
		}
		return nil, apperror.Internal(err)
	}
	if !actor.CanAccessTask(task) {
		return nil, apperror.Forbidden()
	}
	return task, nil
}

// NewTask builds a task owned by ownerID. id, user_id, assignee_id and the
// timestamps are always set by the server, never by the client.
func NewTask(ownerID uuid.UUID, in TaskInput, now time.Time) *domain.Task {
	status := in.Status
	if status == "" {
		status = domain.TaskStatusTodo
	}
	return &domain.Task{
		ID:          uuid.New(),
		UserID:      ownerID,
		Title:       strings.TrimSpace(in.Title),
		Description: normalizeDescription(in.Description),
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// normalizeDescription treats an empty description as "no description".
func normalizeDescription(d *string) *string {
	if d == nil || *d == "" {
		return nil
	}
	return d
}
