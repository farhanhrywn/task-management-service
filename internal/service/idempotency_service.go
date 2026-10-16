package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/dto"
	"github.com/farhanhrywn/task-management-service/internal/repository"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/clock"
)

type IdempotencyConfig struct {
	// TTL is how long a key is remembered (24h).
	TTL time.Duration
	// WaitTimeout is how long a duplicate waits for the first request (10s).
	WaitTimeout time.Duration
	// PollInterval is how often a waiting duplicate re-checks the key (100ms).
	PollInterval time.Duration
}

// IdempotencyService creates tasks exactly once per (user, Idempotency-Key).
type IdempotencyService struct {
	tx    repository.TxManager
	keys  repository.IdempotencyRepository
	tasks repository.TaskRepository
	clock clock.Clock
	cfg   IdempotencyConfig
}

func NewIdempotencyService(
	tx repository.TxManager,
	keys repository.IdempotencyRepository,
	tasks repository.TaskRepository,
	clk clock.Clock,
	cfg IdempotencyConfig,
) *IdempotencyService {
	return &IdempotencyService{tx: tx, keys: keys, tasks: tasks, clock: clk, cfg: cfg}
}

// CreateTask returns the response for (ownerID, key). The first request
// creates the task and stores its response in the same transaction; any later
// request within the TTL replays that exact response, whatever its body.
// A request that arrives while the first is still running waits for it, up
// to WaitTimeout, then fails with 409 IDEMPOTENCY_IN_PROGRESS.
func (s *IdempotencyService) CreateTask(ctx context.Context, ownerID, key uuid.UUID, in TaskInput) (domain.StoredResponse, bool, error) {
	waitCtx, cancel := context.WithTimeout(ctx, s.cfg.WaitTimeout)
	defer cancel()

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		resp, replayed, done, err := s.attempt(ctx, ownerID, key, in)
		if err != nil {
			if errors.Is(err, repository.ErrLockTimeout) {
				return domain.StoredResponse{}, false, apperror.IdempotencyInProgress()
			}
			if _, ok := apperror.As(err); ok {
				return domain.StoredResponse{}, false, err
			}
			return domain.StoredResponse{}, false, apperror.Internal(err)
		}
		if done {
			return resp, replayed, nil
		}

		// Another request holds the key: wait and look again.
		select {
		case <-waitCtx.Done():
			if ctx.Err() != nil {
				return domain.StoredResponse{}, false, ctx.Err()
			}
			return domain.StoredResponse{}, false, apperror.IdempotencyInProgress()
		case <-ticker.C:
		}
	}
}

// attempt runs one claim in a single transaction. done=false means the key is
// held by an in-flight request.
func (s *IdempotencyService) attempt(ctx context.Context, ownerID, key uuid.UUID, in TaskInput) (domain.StoredResponse, bool, bool, error) {
	var (
		resp     domain.StoredResponse
		replayed bool
		done     bool
	)
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		now := s.clock.Now()

		claim, err := s.keys.Claim(ctx, ownerID, key, now, now.Add(s.cfg.TTL))
		if err != nil {
			return err
		}
		if !claim.Claimed {
			if claim.Existing != nil {
				resp, replayed, done = *claim.Existing, true, true
			}
			return nil
		}

		task := NewTask(ownerID, in, now)
		if err := s.tasks.Create(ctx, task); err != nil {
			return err
		}

		body, err := json.Marshal(dataEnvelope{Data: dto.NewTaskResponse(task)})
		if err != nil {
			return err
		}
		resp = domain.StoredResponse{StatusCode: http.StatusCreated, Body: body}
		if err := s.keys.SaveResponse(ctx, ownerID, key, task.ID, resp); err != nil {
			return err
		}
		done = true
		return nil
	})
	if err != nil {
		return domain.StoredResponse{}, false, false, err
	}
	return resp, replayed, done, nil
}

// dataEnvelope matches response.Data so stored bodies look like live ones.
type dataEnvelope struct {
	Data any `json:"data"`
}
