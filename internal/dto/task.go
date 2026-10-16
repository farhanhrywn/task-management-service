package dto

import (
	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

// CreateTaskRequest: user_id, assignee_id and timestamps are not accepted
// from the client; unknown fields are ignored.
type CreateTaskRequest struct {
	Title       string  `json:"title" binding:"required,notblank,max=255"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	Status      string  `json:"status" binding:"omitempty,oneof=todo in_progress done"`
}

type UpdateTaskRequest struct {
	Title       string  `json:"title" binding:"required,notblank,max=255"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	Status      string  `json:"status" binding:"required,oneof=todo in_progress done"`
}

type TaskResponse struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	AssigneeID  *string `json:"assignee_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func NewTaskResponse(t *domain.Task) TaskResponse {
	var assignee *string
	if t.AssigneeID != nil {
		s := t.AssigneeID.String()
		assignee = &s
	}
	return TaskResponse{
		ID:          t.ID.String(),
		UserID:      t.UserID.String(),
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		AssigneeID:  assignee,
		CreatedAt:   response.FormatTime(t.CreatedAt),
		UpdatedAt:   response.FormatTime(t.UpdatedAt),
	}
}
