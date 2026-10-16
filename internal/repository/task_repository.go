package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/farhanhrywn/task-management-service/internal/domain"
)

type TaskRepository interface {
	Create(ctx context.Context, t *domain.Task) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Task, error)
	Update(ctx context.Context, t *domain.Task) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(ctx context.Context, t *domain.Task) error {
	return translate(dbFrom(ctx, r.db).Create(t).Error)
}

// FindByID excludes soft-deleted rows (GORM adds deleted_at IS NULL).
func (r *taskRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	var t domain.Task
	if err := dbFrom(ctx, r.db).Where("id = ?", id).Take(&t).Error; err != nil {
		return nil, translate(err)
	}
	return &t, nil
}

// Update persists the client-editable fields only. assignee_id and user_id
// are never touched here.
func (r *taskRepository) Update(ctx context.Context, t *domain.Task) error {
	res := dbFrom(ctx, r.db).Model(&domain.Task{}).
		Where("id = ?", t.ID).
		Updates(map[string]any{
			"title":       t.Title,
			"description": t.Description,
			"status":      t.Status,
			"updated_at":  t.UpdatedAt,
		})
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDelete sets deleted_at; the row stays for task_logs and idempotency history.
func (r *taskRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	res := dbFrom(ctx, r.db).Where("id = ?", id).Delete(&domain.Task{})
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
