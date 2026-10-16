package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/farhanhrywn/task-management-service/internal/domain"
)

type TaskRepository interface {
	Create(ctx context.Context, t *domain.Task) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Task, error)
	Update(ctx context.Context, t *domain.Task) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, f TaskFilter) ([]domain.Task, int64, error)
	// FindByIDForUpdate locks the row (SELECT ... FOR UPDATE); call it inside a transaction.
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Task, error)
	UpdateAssignee(ctx context.Context, id, assigneeID uuid.UUID, updatedAt time.Time) error
}

// TaskFilter narrows GET /tasks. OwnerID nil means every owner (super admin).
type TaskFilter struct {
	OwnerID *uuid.UUID
	Status  domain.TaskStatus
	Search  string
	Page    int
	Limit   int
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

func (r *taskRepository) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	var t domain.Task
	err := dbFrom(ctx, r.db).
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("id = ?", id).
		Take(&t).Error
	if err != nil {
		return nil, translate(err)
	}
	return &t, nil
}

func (r *taskRepository) UpdateAssignee(ctx context.Context, id, assigneeID uuid.UUID, updatedAt time.Time) error {
	res := dbFrom(ctx, r.db).Model(&domain.Task{}).
		Where("id = ?", id).
		Updates(map[string]any{"assignee_id": assigneeID, "updated_at": updatedAt})
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
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

// List runs COUNT and SELECT with the same scope. Both hit the partial
// indexes on (user_id[, status], created_at DESC) or the trigram index on title.
func (r *taskRepository) List(ctx context.Context, f TaskFilter) ([]domain.Task, int64, error) {
	// A fresh chain per query: reusing a GORM chain after Count leaks state.
	scope := func(q *gorm.DB) *gorm.DB {
		q = q.Model(&domain.Task{})
		if f.OwnerID != nil {
			q = q.Where("user_id = ?", *f.OwnerID)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		if f.Search != "" {
			q = q.Where("title ILIKE ?", "%"+EscapeLike(f.Search)+"%")
		}
		return q
	}

	var total int64
	if err := dbFrom(ctx, r.db).Scopes(scope).Count(&total).Error; err != nil {
		return nil, 0, translate(err)
	}

	tasks := make([]domain.Task, 0, f.Limit)
	if total == 0 {
		return tasks, 0, nil
	}
	if err := dbFrom(ctx, r.db).Scopes(scope).
		Order("created_at DESC").Order("id DESC").
		Limit(f.Limit).Offset((f.Page - 1) * f.Limit).
		Find(&tasks).Error; err != nil {
		return nil, 0, translate(err)
	}
	return tasks, total, nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike escapes LIKE wildcards so user input is matched literally.
func EscapeLike(s string) string { return likeEscaper.Replace(s) }
