package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/farhanhrywn/task-management-service/internal/config"
	"github.com/farhanhrywn/task-management-service/internal/database"
	"github.com/farhanhrywn/task-management-service/internal/router"
	"github.com/farhanhrywn/task-management-service/pkg/logger"
)

func main() {
	log := logger.New(os.Stdout, slog.LevelInfo)
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Release mode keeps Gin's plain-text debug output out of the JSON log stream.
	gin.SetMode(gin.ReleaseMode)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	if err := database.Seed(ctx, db, cfg.SuperAdmin); err != nil {
		return err
	}
	log.Info("database ready")

	engine := router.New(router.Deps{Logger: log})

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	return nil
}
