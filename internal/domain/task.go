package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusTodo, TaskStatusInProgress, TaskStatusDone:
		return true
	}
	return false
}

type Task struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null"`
	AssigneeID  *uuid.UUID     `gorm:"type:uuid"`
	Title       string         `gorm:"type:varchar(255);not null"`
	Description *string        `gorm:"type:text"`
	Status      TaskStatus     `gorm:"type:varchar(20);not null;default:todo;check:chk_tasks_status,status IN ('todo','in_progress','done')"`
	CreatedAt   time.Time      `gorm:"type:timestamptz(0);not null"`
	UpdatedAt   time.Time      `gorm:"type:timestamptz(0);not null"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamptz(0)"`

	Owner    *User `gorm:"foreignKey:UserID"`
	Assignee *User `gorm:"foreignKey:AssigneeID"`
}

const TaskActionAssign = "ASSIGN"

type TaskLog struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TaskID        uuid.UUID  `gorm:"type:uuid;not null;index:idx_task_logs_task_created,priority:1"`
	ActorID       uuid.UUID  `gorm:"type:uuid;not null"`
	Action        string     `gorm:"type:varchar(30);not null"`
	OldAssigneeID *uuid.UUID `gorm:"type:uuid"`
	NewAssigneeID uuid.UUID  `gorm:"type:uuid;not null"`
	CreatedAt     time.Time  `gorm:"type:timestamptz(0);not null;index:idx_task_logs_task_created,priority:2"`

	Task  *Task `gorm:"foreignKey:TaskID"`
	Actor *User `gorm:"foreignKey:ActorID"`
}

// IdempotencyKey records the outcome of POST /tasks per (user, key).
// ResponseBody is stored as `json` (not `jsonb`) so a replay returns the
// exact bytes of the first response.
type IdempotencyKey struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_idempotency_user_key,priority:1"`
	IdemKey        uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_idempotency_user_key,priority:2"`
	TaskID         *uuid.UUID      `gorm:"type:uuid"`
	ResponseStatus *int            `gorm:"type:integer"`
	ResponseBody   json.RawMessage `gorm:"type:json"`
	CreatedAt      time.Time       `gorm:"type:timestamptz(0);not null"`
	ExpiresAt      time.Time       `gorm:"type:timestamptz(0);not null"`

	User *User `gorm:"foreignKey:UserID"`
	Task *Task `gorm:"foreignKey:TaskID"`
}

// StoredResponse is the HTTP response captured for idempotent replay.
type StoredResponse struct {
	StatusCode int
	Body       json.RawMessage
}
