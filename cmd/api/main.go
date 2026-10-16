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
	"golang.org/x/crypto/bcrypt"

	"github.com/farhanhrywn/task-management-service/internal/config"
	"github.com/farhanhrywn/task-management-service/internal/database"
	"github.com/farhanhrywn/task-management-service/internal/handler"
	"github.com/farhanhrywn/task-management-service/internal/notifier"
	"github.com/farhanhrywn/task-management-service/internal/repository"
	"github.com/farhanhrywn/task-management-service/internal/router"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/clock"
	"github.com/farhanhrywn/task-management-service/pkg/jwtutil"
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

	clk := clock.Real{}
	jwtManager := jwtutil.NewManager(cfg.JWT.Secret, cfg.JWT.ExpiresIn, clk)

	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	idemRepo := repository.NewIdempotencyRepository(db, cfg.Idem.WaitTimeout)
	teamRepo := repository.NewTeamRepository(db)
	taskLogRepo := repository.NewTaskLogRepository(db)
	txManager := repository.NewTxManager(db)

	authSvc := service.NewAuthService(userRepo, jwtManager, bcrypt.DefaultCost)
	taskSvc := service.NewTaskService(taskRepo, clk)
	teamSvc := service.NewTeamService(teamRepo, userRepo, clk)
	assignSvc := service.NewAssignService(txManager, taskRepo, userRepo, teamRepo, taskLogRepo, notifier.NewLogNotifier(), clk)
	idemSvc := service.NewIdempotencyService(txManager, idemRepo, taskRepo, clk, service.IdempotencyConfig{
		TTL:          cfg.Idem.TTL,
		WaitTimeout:  cfg.Idem.WaitTimeout,
		PollInterval: cfg.Idem.PollInterval,
	})

	engine := router.New(router.Deps{
		Logger:      log,
		TokenParser: jwtManager,
		Auth:        handler.NewAuthHandler(authSvc, jwtManager.TTL()),
		Task: handler.NewTaskHandler(idemSvc, taskSvc, assignSvc, handler.Pagination{
			DefaultLimit: cfg.Pagination.DefaultLimit,
			MaxLimit:     cfg.Pagination.MaxLimit,
		}),
		Team: handler.NewTeamHandler(teamSvc),
	})

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
