package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/farhanhrywn/task-management-service/internal/domain"
)

// customIndexes are the partial and trigram indexes AutoMigrate cannot express.
var customIndexes = []string{
	`CREATE EXTENSION IF NOT EXISTS pg_trgm`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_user_created ON tasks (user_id, created_at DESC) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_user_status  ON tasks (user_id, status, created_at DESC) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_created      ON tasks (created_at DESC) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_title_trgm   ON tasks USING GIN (title gin_trgm_ops)`,
}

// Migrate creates or updates the schema, then the custom indexes.
func Migrate(ctx context.Context, db *gorm.DB) error {
	db = db.WithContext(ctx)

	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Team{},
		&domain.TeamMember{},
		&domain.Task{},
		&domain.TaskLog{},
		&domain.IdempotencyKey{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	for _, stmt := range customIndexes {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create custom index: %w", err)
		}
	}
	return nil
}
