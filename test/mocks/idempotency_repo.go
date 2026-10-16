package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
)

type idemKey struct {
	userID uuid.UUID
	key    uuid.UUID
}

type idemRecord struct {
	committed bool
	expiresAt time.Time
	taskID    uuid.UUID
	resp      *domain.StoredResponse
}

// InMemoryIdempotencyRepo emulates the unique constraint (user_id, idem_key).
// A claimed key stays "pending" until its fake transaction commits, which is
// what concurrent requests observe while PostgreSQL holds the row lock. A
// rollback removes the claim (restoring an expired record it replaced).
type InMemoryIdempotencyRepo struct {
	mu      sync.Mutex
	records map[idemKey]*idemRecord
}

var _ repository.IdempotencyRepository = (*InMemoryIdempotencyRepo)(nil)

func NewInMemoryIdempotencyRepo() *InMemoryIdempotencyRepo {
	return &InMemoryIdempotencyRepo{records: make(map[idemKey]*idemRecord)}
}

func (r *InMemoryIdempotencyRepo) Claim(ctx context.Context, userID, key uuid.UUID, now, expiresAt time.Time) (repository.ClaimResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	k := idemKey{userID, key}
	prev, exists := r.records[k]
	if exists {
		if !prev.committed {
			return repository.ClaimResult{}, nil // held by an in-flight request
		}
		if prev.expiresAt.After(now) {
			if prev.resp == nil {
				return repository.ClaimResult{}, nil
			}
			resp := cloneResponse(*prev.resp)
			return repository.ClaimResult{Existing: &resp}, nil
		}
	}

	rec := &idemRecord{expiresAt: expiresAt}
	r.records[k] = rec
	inTx(ctx,
		func() {
			r.mu.Lock()
			rec.committed = true
			r.mu.Unlock()
		},
		func() {
			r.mu.Lock()
			if r.records[k] == rec {
				if exists {
					r.records[k] = prev
				} else {
					delete(r.records, k)
				}
			}
			r.mu.Unlock()
		},
	)
	return repository.ClaimResult{Claimed: true}, nil
}

func (r *InMemoryIdempotencyRepo) SaveResponse(_ context.Context, userID, key, taskID uuid.UUID, resp domain.StoredResponse) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[idemKey{userID, key}]
	if !ok {
		return repository.ErrNotFound
	}
	stored := cloneResponse(resp)
	rec.taskID = taskID
	rec.resp = &stored
	return nil
}

func cloneResponse(r domain.StoredResponse) domain.StoredResponse {
	body := make([]byte, len(r.Body))
	copy(body, r.Body)
	return domain.StoredResponse{StatusCode: r.StatusCode, Body: body}
}
