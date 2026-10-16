package service_test

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/test/mocks"
)

type idemFixture struct {
	svc   *service.IdempotencyService
	tasks *mocks.InMemoryTaskRepo
	keys  *mocks.InMemoryIdempotencyRepo
	clock *mocks.FakeClock
}

func newIdemFixture(t *testing.T, cfg service.IdempotencyConfig) *idemFixture {
	t.Helper()
	if cfg.TTL == 0 {
		cfg.TTL = 24 * time.Hour
	}
	if cfg.WaitTimeout == 0 {
		cfg.WaitTimeout = 10 * time.Second
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = time.Millisecond
	}
	f := &idemFixture{
		tasks: mocks.NewInMemoryTaskRepo(),
		keys:  mocks.NewInMemoryIdempotencyRepo(),
		clock: mocks.NewFakeClock(time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)),
	}
	f.svc = service.NewIdempotencyService(mocks.NewFakeTxManager(), f.keys, f.tasks, f.clock, cfg)
	return f
}

func input(title string) service.TaskInput {
	return service.TaskInput{Title: title, Status: domain.TaskStatusTodo}
}

// UT-01: a second request with the same key replays the first response.
func TestCreateTask_SequentialDuplicate_ReplaysFirstResponse(t *testing.T) {
	f := newIdemFixture(t, service.IdempotencyConfig{})
	ctx := context.Background()
	userID, key := uuid.New(), uuid.New()

	first, replayed, err := f.svc.CreateTask(ctx, userID, key, input("Setup CI"))
	if err != nil {
		t.Fatalf("first request: unexpected error: %v", err)
	}
	if first.StatusCode != http.StatusCreated || replayed {
		t.Fatalf("first request: got status %d replayed=%v, want 201 replayed=false", first.StatusCode, replayed)
	}

	second, replayed, err := f.svc.CreateTask(ctx, userID, key, input("Setup CI"))
	if err != nil {
		t.Fatalf("second request: unexpected error: %v", err)
	}
	if !replayed {
		t.Fatal("second request: want replayed=true")
	}
	if second.StatusCode != first.StatusCode || !bytes.Equal(second.Body, first.Body) {
		t.Fatalf("second response differs:\nfirst:  %d %s\nsecond: %d %s", first.StatusCode, first.Body, second.StatusCode, second.Body)
	}
	if got := f.tasks.Count(); got != 1 {
		t.Fatalf("task count = %d, want 1", got)
	}
}

// UT-02: 100 goroutines released at once with the same key create exactly
// one task and all receive the identical response. Run with -race.
func TestCreateTask_ConcurrentDuplicates_CreateExactlyOneTask(t *testing.T) {
	const workers = 100

	f := newIdemFixture(t, service.IdempotencyConfig{})
	// Keep the winning transaction open briefly so the other goroutines
	// genuinely overlap with it and have to wait, not just replay.
	f.tasks.BeforeCreate = func(context.Context, *domain.Task) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}

	userID, key := uuid.New(), uuid.New()
	var (
		start     = make(chan struct{})
		wg        sync.WaitGroup
		responses = make([]domain.StoredResponse, workers)
		replays   = make([]bool, workers)
		errs      = make([]error, workers)
	)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			responses[i], replays[i], errs[i] = f.svc.CreateTask(context.Background(), userID, key, input("Concurrent"))
		}(i)
	}
	close(start)
	wg.Wait()

	created := 0
	for i := range workers {
		if errs[i] != nil {
			t.Fatalf("worker %d: unexpected error: %v", i, errs[i])
		}
		if responses[i].StatusCode != http.StatusCreated {
			t.Fatalf("worker %d: status = %d, want 201", i, responses[i].StatusCode)
		}
		if !bytes.Equal(responses[i].Body, responses[0].Body) {
			t.Fatalf("worker %d: body differs from worker 0:\n%s\n%s", i, responses[i].Body, responses[0].Body)
		}
		if !replays[i] {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("non-replayed responses = %d, want exactly 1", created)
	}
	if got := f.tasks.Count(); got != 1 {
		t.Fatalf("task count = %d, want 1", got)
	}
}
