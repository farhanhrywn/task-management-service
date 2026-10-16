// Package database owns the PostgreSQL connection, schema migration and seed.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/farhanhrywn/task-management-service/internal/config"
	"github.com/farhanhrywn/task-management-service/pkg/clock"
)

const (
	connectAttempts = 10
	connectBackoff  = 2 * time.Second
)

// Connect opens a GORM connection and retries until PostgreSQL is reachable.
// NowFunc truncates to whole seconds so every timestamp GORM writes matches
// the TIMESTAMPTZ(0) column precision.
func Connect(ctx context.Context, cfg config.DBConfig, log *slog.Logger) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		NowFunc: clock.Real{}.Now,
		Logger:  gormlogger.Discard,
	}

	var lastErr error
	for attempt := 1; attempt <= connectAttempts; attempt++ {
		db, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
		if err == nil {
			sqlDB, err := db.DB()
			if err == nil {
				if err = sqlDB.PingContext(ctx); err == nil {
					sqlDB.SetMaxOpenConns(25)
					sqlDB.SetMaxIdleConns(10)
					sqlDB.SetConnMaxLifetime(30 * time.Minute)
					return db, nil
				}
			}
			lastErr = err
		} else {
			lastErr = err
		}

		log.Warn("database not ready, retrying", "attempt", attempt, "error", lastErr)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(connectBackoff):
		}
	}
	return nil, fmt.Errorf("connect database: %w", lastErr)
}
