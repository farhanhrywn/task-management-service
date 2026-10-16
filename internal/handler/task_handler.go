package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/dto"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

type TaskCreator interface {
	CreateTask(ctx context.Context, ownerID, key uuid.UUID, in service.TaskInput) (domain.StoredResponse, bool, error)
}

type TaskService interface {
	Get(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.Task, error)
	Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in service.TaskInput) (*domain.Task, error)
	Delete(ctx context.Context, actor domain.Actor, id uuid.UUID) error
	List(ctx context.Context, actor domain.Actor, in service.ListInput) ([]domain.Task, int64, error)
}

type Pagination struct {
	DefaultLimit int
	MaxLimit     int
}

const (
	HeaderIdempotencyKey     = "Idempotency-Key"
	HeaderIdempotentReplayed = "Idempotent-Replayed"
	canonicalUUIDLength      = 36
)

type TaskHandler struct {
	creator    TaskCreator
	tasks      TaskService
	pagination Pagination
}

func NewTaskHandler(creator TaskCreator, tasks TaskService, pagination Pagination) *TaskHandler {
	return &TaskHandler{creator: creator, tasks: tasks, pagination: pagination}
}

// Create handles POST /tasks. The Idempotency-Key header is mandatory; a
// repeated key replays the first response byte for byte.
func (h *TaskHandler) Create(c *gin.Context) {
	key, err := parseIdempotencyKey(c.GetHeader(HeaderIdempotencyKey))
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.CreateTaskRequest
	if err := bindJSON(c, &req); err != nil {
		_ = c.Error(err)
		return
	}

	resp, replayed, err := h.creator.CreateTask(c.Request.Context(), currentActor(c).UserID, key, service.TaskInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.TaskStatus(req.Status),
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	if replayed {
		c.Header(HeaderIdempotentReplayed, "true")
	}
	response.Raw(c, resp.StatusCode, resp.Body)
}

// parseIdempotencyKey accepts only the canonical 8-4-4-4-12 UUID form.
func parseIdempotencyKey(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, apperror.IdempotencyKeyMissing()
	}
	key, err := uuid.Parse(raw)
	if err != nil || len(raw) != canonicalUUIDLength || key == uuid.Nil {
		return uuid.Nil, apperror.IdempotencyKeyInvalid()
	}
	return key, nil
}

// List handles GET /tasks?status=&search=&page=&limit=.
func (h *TaskHandler) List(c *gin.Context) {
	var q dto.ListTasksQuery
	if err := bindQuery(c, &q); err != nil {
		_ = c.Error(err)
		return
	}

	page, limit := 1, h.pagination.DefaultLimit
	if q.Page != nil {
		page = *q.Page
	}
	if q.Limit != nil {
		limit = *q.Limit
	}
	if limit > h.pagination.MaxLimit {
		_ = c.Error(apperror.Validation([]FieldError{{
			Field:   "limit",
			Message: fmt.Sprintf("must be at most %d", h.pagination.MaxLimit),
		}}))
		return
	}

	tasks, total, err := h.tasks.List(c.Request.Context(), currentActor(c), service.ListInput{
		Status: domain.TaskStatus(q.Status),
		Search: q.Search,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}

	data := make([]dto.TaskResponse, 0, len(tasks))
	for i := range tasks {
		data = append(data, dto.NewTaskResponse(&tasks[i]))
	}
	response.List(c, data, response.Meta{
		Page:       page,
		Limit:      limit,
		TotalItems: total,
		TotalPages: int((total + int64(limit) - 1) / int64(limit)),
	})
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
