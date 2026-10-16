package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/test/mocks"
)

func newTaskServiceWithTask(t *testing.T, owner uuid.UUID) (*service.TaskService, *mocks.InMemoryTaskRepo, uuid.UUID) {
	t.Helper()
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	repo := mocks.NewInMemoryTaskRepo()
	id := uuid.New()
	repo.Put(domain.Task{ID: id, UserID: owner, Title: "Owner's task", Status: domain.TaskStatusTodo, CreatedAt: now, UpdatedAt: now})
	return service.NewTaskService(repo, mocks.NewFakeClock(now)), repo, id
}

// UT-10: another user cannot read, update or delete the task (403).
func TestTaskAccess_OtherUserIsForbidden(t *testing.T) {
	owner := uuid.New()
	svc, repo, id := newTaskServiceWithTask(t, owner)
	other := domain.Actor{UserID: uuid.New(), Role: domain.RoleUser}
	ctx := context.Background()

	if _, err := svc.Get(ctx, other, id); !apperror.Is(err, apperror.CodeForbidden) {
		t.Fatalf("Get: want FORBIDDEN, got %v", err)
	}
	if _, err := svc.Update(ctx, other, id, service.TaskInput{Title: "hijack", Status: domain.TaskStatusDone}); !apperror.Is(err, apperror.CodeForbidden) {
		t.Fatalf("Update: want FORBIDDEN, got %v", err)
	}
	if err := svc.Delete(ctx, other, id); !apperror.Is(err, apperror.CodeForbidden) {
		t.Fatalf("Delete: want FORBIDDEN, got %v", err)
	}

	stored, _ := repo.Snapshot(id)
	if stored.Title != "Owner's task" || stored.DeletedAt.Valid {
		t.Fatalf("forbidden calls must not modify the task: %+v", stored)
	}
}

func TestTaskAccess_OwnerAndSuperAdminAllowed(t *testing.T) {
	owner := uuid.New()
	svc, _, id := newTaskServiceWithTask(t, owner)
	ctx := context.Background()

	for _, actor := range []domain.Actor{
		{UserID: owner, Role: domain.RoleUser},
		{UserID: uuid.New(), Role: domain.RoleSuperAdmin},
	} {
		if _, err := svc.Get(ctx, actor, id); err != nil {
			t.Fatalf("Get as %s: %v", actor.Role, err)
		}
	}

	admin := domain.Actor{UserID: uuid.New(), Role: domain.RoleSuperAdmin}
	task, err := svc.Update(ctx, admin, id, service.TaskInput{Title: "Edited by admin", Status: domain.TaskStatusInProgress})
	if err != nil || task.Title != "Edited by admin" {
		t.Fatalf("Update as super admin: task=%+v err=%v", task, err)
	}
}

func TestTaskAccess_DeletedTaskIsNotFound(t *testing.T) {
	owner := uuid.New()
	svc, _, id := newTaskServiceWithTask(t, owner)
	actor := domain.Actor{UserID: owner, Role: domain.RoleUser}
	ctx := context.Background()

	if err := svc.Delete(ctx, actor, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, actor, id); !apperror.Is(err, apperror.CodeTaskNotFound) {
		t.Fatalf("Get after delete: want TASK_NOT_FOUND, got %v", err)
	}
	if err := svc.Delete(ctx, actor, id); !apperror.Is(err, apperror.CodeTaskNotFound) {
		t.Fatalf("Delete twice: want TASK_NOT_FOUND, got %v", err)
	}
}
