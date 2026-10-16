package service_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/service"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
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

// UT-03: same key with a different body replays the first response.
func TestCreateTask_SameKeyDifferentBody_ReplaysFirstResponse(t *testing.T) {
	f := newIdemFixture(t, service.IdempotencyConfig{})
	ctx := context.Background()
	userID, key := uuid.New(), uuid.New()

	first, _, err := f.svc.CreateTask(ctx, userID, key, input("First title"))
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	second, replayed, err := f.svc.CreateTask(ctx, userID, key, input("Completely different"))
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if !replayed || !bytes.Equal(first.Body, second.Body) {
		t.Fatalf("want replay of the first body, got replayed=%v body=%s", replayed, second.Body)
	}
	if !bytes.Contains(second.Body, []byte(`"First title"`)) {
		t.Fatalf("replayed body should carry the first title: %s", second.Body)
	}
	if got := f.tasks.Count(); got != 1 {
		t.Fatalf("task count = %d, want 1", got)
	}
}

// UT-04: keys are scoped per user.
func TestCreateTask_SameKeyDifferentUsers_CreatesTwoTasks(t *testing.T) {
	f := newIdemFixture(t, service.IdempotencyConfig{})
	ctx := context.Background()
	key := uuid.New()

	for _, user := range []uuid.UUID{uuid.New(), uuid.New()} {
		_, replayed, err := f.svc.CreateTask(ctx, user, key, input("Same key"))
		if err != nil {
			t.Fatalf("user %s: %v", user, err)
		}
		if replayed {
			t.Fatalf("user %s: a key from another user must not be replayed", user)
		}
	}
	if got := f.tasks.Count(); got != 2 {
		t.Fatalf("task count = %d, want 2", got)
	}
}

// UT-05: a key older than the 24h window is treated as new.
func TestCreateTask_ExpiredKey_CreatesNewTask(t *testing.T) {
	f := newIdemFixture(t, service.IdempotencyConfig{TTL: 24 * time.Hour})
	ctx := context.Background()
	userID, key := uuid.New(), uuid.New()

	first, _, err := f.svc.CreateTask(ctx, userID, key, input("Day one"))
	if err != nil {
		t.Fatalf("first request: %v", err)
	}

	f.clock.Advance(23*time.Hour + 59*time.Minute)
	if _, replayed, err := f.svc.CreateTask(ctx, userID, key, input("Day one")); err != nil || !replayed {
		t.Fatalf("within the window: want replay, got replayed=%v err=%v", replayed, err)
	}

	f.clock.Advance(time.Minute) // exactly 24h after the first request
	second, replayed, err := f.svc.CreateTask(ctx, userID, key, input("Day two"))
	if err != nil {
		t.Fatalf("after expiry: %v", err)
	}
	if replayed || bytes.Equal(first.Body, second.Body) {
		t.Fatal("after expiry: want a fresh response, got a replay")
	}
	if got := f.tasks.Count(); got != 2 {
		t.Fatalf("task count = %d, want 2", got)
	}
}

// UT-06: a failure inside the transaction rolls the key back so a retry works.
func TestCreateTask_InsertFails_KeyRolledBackAndRetrySucceeds(t *testing.T) {
	f := newIdemFixture(t, service.IdempotencyConfig{})
	ctx := context.Background()
	userID, key := uuid.New(), uuid.New()

	var calls atomic.Int32
	f.tasks.BeforeCreate = func(context.Context, *domain.Task) error {
		if calls.Add(1) == 1 {
			return errors.New("connection reset")
		}
		return nil
	}

	_, _, err := f.svc.CreateTask(ctx, userID, key, input("Flaky"))
	if !apperror.Is(err, apperror.CodeInternal) {
		t.Fatalf("first request: want INTERNAL_ERROR, got %v", err)
	}
	if got := f.tasks.Count(); got != 0 {
		t.Fatalf("after failure: task count = %d, want 0", got)
	}

	resp, replayed, err := f.svc.CreateTask(ctx, userID, key, input("Flaky"))
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if replayed || resp.StatusCode != http.StatusCreated {
		t.Fatalf("retry: want a fresh 201, got status=%d replayed=%v", resp.StatusCode, replayed)
	}
	if got := f.tasks.Count(); got != 1 {
		t.Fatalf("after retry: task count = %d, want 1", got)
	}
}

// UT-07: a duplicate that waits longer than WaitTimeout gets 409.
func TestCreateTask_FirstRequestTooSlow_DuplicateGets409(t *testing.T) {
	f := newIdemFixture(t, service.IdempotencyConfig{WaitTimeout: 100 * time.Millisecond})
	ctx := context.Background()
	userID, key := uuid.New(), uuid.New()

	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.tasks.BeforeCreate = func(context.Context, *domain.Task) error {
		once.Do(func() { close(entered) })
		<-release
		return nil
	}

	type result struct {
		resp domain.StoredResponse
		err  error
	}
	firstDone := make(chan result, 1)
	go func() {
		resp, _, err := f.svc.CreateTask(ctx, userID, key, input("Slow"))
		firstDone <- result{resp, err}
	}()
	<-entered

	start := time.Now()
	_, _, err := f.svc.CreateTask(ctx, userID, key, input("Slow"))
	if !apperror.Is(err, apperror.CodeIdempotencyInProgress) {
		t.Fatalf("duplicate: want IDEMPOTENCY_IN_PROGRESS, got %v", err)
	}
	if waited := time.Since(start); waited < 100*time.Millisecond {
		t.Fatalf("duplicate returned after %v, before the wait timeout", waited)
	}

	close(release)
	first := <-firstDone
	if first.err != nil || first.resp.StatusCode != http.StatusCreated {
		t.Fatalf("first request: want 201, got %d err=%v", first.resp.StatusCode, first.err)
	}
	if _, replayed, err := f.svc.CreateTask(ctx, userID, key, input("Slow")); err != nil || !replayed {
		t.Fatalf("after completion: want replay, got replayed=%v err=%v", replayed, err)
	}
	if got := f.tasks.Count(); got != 1 {
		t.Fatalf("task count = %d, want 1", got)
	}
}
