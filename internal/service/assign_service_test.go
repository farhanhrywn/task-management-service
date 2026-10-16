package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/test/mocks"
)

type assignFixture struct {
	svc      *service.AssignService
	tasks    *mocks.InMemoryTaskRepo
	logs     *mocks.InMemoryTaskLogRepo
	notifier *mocks.StubNotifier

	admin, owner, teammate, outsider uuid.UUID
	taskID                           uuid.UUID
}

// newAssignFixture: owner and teammate are in Team Alpha, outsider is in
// Team Beta, and owner has one unassigned task.
func newAssignFixture(t *testing.T) *assignFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)

	users := mocks.NewInMemoryUserRepo()
	teams := mocks.NewInMemoryTeamRepo()
	f := &assignFixture{
		tasks:    mocks.NewInMemoryTaskRepo(),
		logs:     mocks.NewInMemoryTaskLogRepo(),
		notifier: &mocks.StubNotifier{},
		admin:    uuid.New(),
		owner:    uuid.New(),
		teammate: uuid.New(),
		outsider: uuid.New(),
		taskID:   uuid.New(),
	}
	for _, id := range []uuid.UUID{f.admin, f.owner, f.teammate, f.outsider} {
		if err := users.Create(ctx, &domain.User{ID: id, Email: id.String() + "@mail.com"}); err != nil {
			t.Fatal(err)
		}
	}

	alpha, beta := uuid.New(), uuid.New()
	teams.PutTeam(domain.Team{ID: alpha, Name: "Team Alpha"})
	teams.PutTeam(domain.Team{ID: beta, Name: "Team Beta"})
	for _, m := range []domain.TeamMember{
		{TeamID: alpha, UserID: f.owner},
		{TeamID: alpha, UserID: f.teammate},
		{TeamID: beta, UserID: f.outsider},
	} {
		if err := teams.AddMember(ctx, &m); err != nil {
			t.Fatal(err)
		}
	}

	f.tasks.Put(domain.Task{ID: f.taskID, UserID: f.owner, Title: "Task", Status: domain.TaskStatusTodo, CreatedAt: now, UpdatedAt: now})
	f.svc = service.NewAssignService(mocks.NewFakeTxManager(), f.tasks, users, teams, f.logs, f.notifier, mocks.NewFakeClock(now))
	return f
}

func (f *assignFixture) assignee(t *testing.T) *uuid.UUID {
	t.Helper()
	task, ok := f.tasks.Snapshot(f.taskID)
	if !ok {
		t.Fatal("task missing")
	}
	return task.AssigneeID
}

func TestAssign_Success_UpdatesAssigneeAndWritesLog(t *testing.T) {
	f := newAssignFixture(t)

	task, err := f.svc.Assign(context.Background(), f.taskID, f.teammate, f.admin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.AssigneeID == nil || *task.AssigneeID != f.teammate {
		t.Fatalf("returned assignee = %v, want %s", task.AssigneeID, f.teammate)
	}
	if got := f.assignee(t); got == nil || *got != f.teammate {
		t.Fatalf("stored assignee = %v, want %s", got, f.teammate)
	}

	logs := f.logs.ForTask(f.taskID)
	if len(logs) != 1 {
		t.Fatalf("task logs = %d, want 1", len(logs))
	}
	l := logs[0]
	if l.Action != domain.TaskActionAssign || l.ActorID != f.admin || l.OldAssigneeID != nil || l.NewAssigneeID != f.teammate {
		t.Fatalf("unexpected task log: %+v", l)
	}
	if f.notifier.Calls != 1 {
		t.Fatalf("notifier calls = %d, want 1", f.notifier.Calls)
	}
}

// Re-assigning to another eligible user (here the owner) is recorded with
// the previous assignee.
func TestAssign_Reassign_RecordsPreviousAssignee(t *testing.T) {
	f := newAssignFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Assign(ctx, f.taskID, f.teammate, f.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Assign(ctx, f.taskID, f.owner, f.admin); err != nil {
		t.Fatalf("reassign to owner: %v", err)
	}

	logs := f.logs.ForTask(f.taskID)
	if len(logs) != 2 {
		t.Fatalf("task logs = %d, want 2", len(logs))
	}
	if logs[1].OldAssigneeID == nil || *logs[1].OldAssigneeID != f.teammate || logs[1].NewAssigneeID != f.owner {
		t.Fatalf("unexpected reassign log: %+v", logs[1])
	}
}

// UT-08: if the notification fails, the assignee update and the task log are
// rolled back.
func TestAssign_NotifierFails_RollsBackEverything(t *testing.T) {
	f := newAssignFixture(t)
	f.notifier.Err = errors.New("notification service unavailable")

	_, err := f.svc.Assign(context.Background(), f.taskID, f.teammate, f.admin)
	if !apperror.Is(err, apperror.CodeInternal) {
		t.Fatalf("want INTERNAL_ERROR, got %v", err)
	}
	if got := f.assignee(t); got != nil {
		t.Fatalf("assignee = %s, want unchanged (nil)", got)
	}
	if logs := f.logs.ForTask(f.taskID); len(logs) != 0 {
		t.Fatalf("task logs = %d, want 0 after rollback", len(logs))
	}
}

// UT-09: assignee outside the owner's teams → 422; same assignee → 409.
func TestAssign_ValidationErrors(t *testing.T) {
	t.Run("assignee not in owner's team", func(t *testing.T) {
		f := newAssignFixture(t)
		_, err := f.svc.Assign(context.Background(), f.taskID, f.outsider, f.admin)
		if !apperror.Is(err, apperror.CodeAssigneeNotInTeam) {
			t.Fatalf("want ASSIGNEE_NOT_IN_TEAM, got %v", err)
		}
		if f.assignee(t) != nil || len(f.logs.ForTask(f.taskID)) != 0 {
			t.Fatal("rejected assignment must not change state")
		}
	})

	t.Run("same assignee", func(t *testing.T) {
		f := newAssignFixture(t)
		ctx := context.Background()
		if _, err := f.svc.Assign(ctx, f.taskID, f.teammate, f.admin); err != nil {
			t.Fatal(err)
		}
		_, err := f.svc.Assign(ctx, f.taskID, f.teammate, f.admin)
		if !apperror.Is(err, apperror.CodeAlreadyAssigned) {
			t.Fatalf("want ALREADY_ASSIGNED, got %v", err)
		}
		if n := len(f.logs.ForTask(f.taskID)); n != 1 {
			t.Fatalf("task logs = %d, want 1", n)
		}
	})

	t.Run("task not found", func(t *testing.T) {
		f := newAssignFixture(t)
		_, err := f.svc.Assign(context.Background(), uuid.New(), f.teammate, f.admin)
		if !apperror.Is(err, apperror.CodeTaskNotFound) {
			t.Fatalf("want TASK_NOT_FOUND, got %v", err)
		}
	})

	t.Run("assignee not found", func(t *testing.T) {
		f := newAssignFixture(t)
		_, err := f.svc.Assign(context.Background(), f.taskID, uuid.New(), f.admin)
		if !apperror.Is(err, apperror.CodeUserNotFound) {
			t.Fatalf("want USER_NOT_FOUND, got %v", err)
		}
	})
}
