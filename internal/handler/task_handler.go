package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/dto"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

type TaskService interface {
	Create(ctx context.Context, ownerID uuid.UUID, in service.TaskInput) (*domain.Task, error)
	Get(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Task, error)
	Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in service.TaskInput) (*domain.Task, error)
	Delete(ctx context.Context, actor domain.Actor, id uuid.UUID) error
}

type TaskHandler struct {
	tasks TaskService
}

func NewTaskHandler(tasks TaskService) *TaskHandler {
	return &TaskHandler{tasks: tasks}
}

// Create handles POST /tasks.
func (h *TaskHandler) Create(c *gin.Context) {
	var req dto.CreateTaskRequest
	if err := bindJSON(c, &req); err != nil {
		_ = c.Error(err)
		return
	}

	task, err := h.tasks.Create(c.Request.Context(), currentActor(c).UserID, service.TaskInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.TaskStatus(req.Status),
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Data(c, http.StatusCreated, dto.NewTaskResponse(task))
}

// Get handles GET /tasks/:id.
func (h *TaskHandler) Get(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}

	task, err := h.tasks.Get(c.Request.Context(), currentActor(c), id)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Data(c, http.StatusOK, dto.NewTaskResponse(task))
}

// Update handles PUT /tasks/:id.
func (h *TaskHandler) Update(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.UpdateTaskRequest
	if err := bindJSON(c, &req); err != nil {
		_ = c.Error(err)
		return
	}

	task, err := h.tasks.Update(c.Request.Context(), currentActor(c), id, service.TaskInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.TaskStatus(req.Status),
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Data(c, http.StatusOK, dto.NewTaskResponse(task))
}

// Delete handles DELETE /tasks/:id (soft delete).
func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.tasks.Delete(c.Request.Context(), currentActor(c), id); err != nil {
		_ = c.Error(err)
		return
	}
	response.Message(c, http.StatusOK, "task deleted")
}
