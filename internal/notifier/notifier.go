// Package notifier sends task notifications. The only implementation is a
// mock that writes a structured log line.
package notifier

import (
	"context"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/pkg/logger"
)

type Notifier interface {
	TaskAssigned(ctx context.Context, task domain.Task, assigneeID uuid.UUID) error
}

// LogNotifier "sends" a notification by logging it.
type LogNotifier struct{}

func NewLogNotifier() *LogNotifier { return &LogNotifier{} }

func (LogNotifier) TaskAssigned(ctx context.Context, task domain.Task, assigneeID uuid.UUID) error {
	logger.FromContext(ctx).InfoContext(ctx, "task assigned notification sent",
		"event", "task_assigned",
		"task_id", task.ID.String(),
		"assignee_id", assigneeID.String(),
	)
	return nil
}
