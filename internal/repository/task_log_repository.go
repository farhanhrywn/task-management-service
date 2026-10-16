package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/farhanhrywn/task-management-service/internal/domain"
)

type TaskLogRepository interface {
	Create(ctx context.Context, l *domain.TaskLog) error
}

type taskLogRepository struct {
	db *gorm.DB
}

func NewTaskLogRepository(db *gorm.DB) TaskLogRepository {
	return &taskLogRepository{db: db}
}

func (r *taskLogRepository) Create(ctx context.Context, l *domain.TaskLog) error {
	return translate(dbFrom(ctx, r.db).Create(l).Error)
}
