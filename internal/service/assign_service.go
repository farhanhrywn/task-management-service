package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/notifier"
	"github.com/farhanhrywn/task-management-service/internal/repository"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/clock"
)

type AssignService struct {
	tx       repository.TxManager
	tasks    repository.TaskRepository
	users    repository.UserRepository
	teams    repository.TeamRepository
	logs     repository.TaskLogRepository
	notifier notifier.Notifier
	clock    clock.Clock
}

func NewAssignService(
	tx repository.TxManager,
	tasks repository.TaskRepository,
	users repository.UserRepository,
	teams repository.TeamRepository,
	logs repository.TaskLogRepository,
	n notifier.Notifier,
	clk clock.Clock,
) *AssignService {
	return &AssignService{tx: tx, tasks: tasks, users: users, teams: teams, logs: logs, notifier: n, clock: clk}
}

// Assign sets the task's assignee in one transaction: lock the task, validate,
// update assignee_id, write a task_logs row and send the notification. Any
// failure rolls back every step, so the assignee and the log never diverge.
func (s *AssignService) Assign(ctx context.Context, taskID, assigneeID, actorID uuid.UUID) (*domain.Task, error) {
	var result *domain.Task
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		task, err := s.tasks.FindByIDForUpdate(ctx, taskID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperror.TaskNotFound()
			}
			return err
		}

		if _, err := s.users.FindByID(ctx, assigneeID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperror.UserNotFound()
			}
			return err
		}

		if task.AssigneeID != nil && *task.AssigneeID == assigneeID {
			return apperror.AlreadyAssigned()
		}

		shared, err := s.teams.ShareTeam(ctx, task.UserID, assigneeID)
		if err != nil {
			return err
		}
		if !shared {
			return apperror.AssigneeNotInTeam()
		}

		now := s.clock.Now()
		previous := task.AssigneeID
		if err := s.tasks.UpdateAssignee(ctx, taskID, assigneeID, now); err != nil {
			return err
		}
		if err := s.logs.Create(ctx, &domain.TaskLog{
			ID:            uuid.New(),
			TaskID:        taskID,
			ActorID:       actorID,
			Action:        domain.TaskActionAssign,
			OldAssigneeID: previous,
			NewAssigneeID: assigneeID,
			CreatedAt:     now,
		}); err != nil {
			return err
		}

		task.AssigneeID = &assigneeID
		task.UpdatedAt = now
		if err := s.notifier.TaskAssigned(ctx, *task, assigneeID); err != nil {
			return err
		}

		result = task
		return nil
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, apperror.Internal(err)
	}
	return result, nil
}
