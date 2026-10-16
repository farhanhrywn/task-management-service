package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/farhanhrywn/task-management-service/internal/domain"
)

// ClaimResult is the outcome of claiming an idempotency key.
//
//   - Claimed: the caller owns the key and must create the task.
//   - !Claimed && Existing != nil: a completed response exists; replay it.
//   - !Claimed && Existing == nil: another request still holds the key.
type ClaimResult struct {
	Claimed  bool
	Existing *domain.StoredResponse
}

// IdempotencyRepository must be used inside TxManager.WithinTx so the claim,
// the task insert and the stored response commit or roll back together.
type IdempotencyRepository interface {
	Claim(ctx context.Context, userID, key uuid.UUID, now, expiresAt time.Time) (ClaimResult, error)
	SaveResponse(ctx context.Context, userID, key, taskID uuid.UUID, resp domain.StoredResponse) error
}

type idempotencyRepository struct {
	db          *gorm.DB
	lockTimeout time.Duration
}

// NewIdempotencyRepository returns the PostgreSQL implementation. lockTimeout
// bounds how long a duplicate request waits for the first one to commit.
func NewIdempotencyRepository(db *gorm.DB, lockTimeout time.Duration) IdempotencyRepository {
	return &idempotencyRepository{db: db, lockTimeout: lockTimeout}
}

// claimSQL inserts the key, or takes over a row whose window has expired.
// A concurrent transaction holding the same key makes this statement wait on
// the unique index until that transaction commits (0 rows → replay) or rolls
// back (1 row → we own the key).
const claimSQL = `
INSERT INTO idempotency_keys (id, user_id, idem_key, created_at, expires_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (user_id, idem_key) DO UPDATE
   SET created_at      = EXCLUDED.created_at,
       expires_at      = EXCLUDED.expires_at,
       task_id         = NULL,
       response_status = NULL,
       response_body   = NULL
 WHERE idempotency_keys.expires_at <= ?
RETURNING id`

func (r *idempotencyRepository) Claim(ctx context.Context, userID, key uuid.UUID, now, expiresAt time.Time) (ClaimResult, error) {
	db := dbFrom(ctx, r.db)

	// SET cannot take bind parameters; the value is an integer we control.
	if err := db.Exec(fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", r.lockTimeout.Milliseconds())).Error; err != nil {
		return ClaimResult{}, translate(err)
	}

	var claimed []struct{ ID uuid.UUID }
	if err := db.Raw(claimSQL, uuid.New(), userID, key, now, expiresAt, now).Scan(&claimed).Error; err != nil {
		return ClaimResult{}, translate(err)
	}
	if len(claimed) == 1 {
		return ClaimResult{Claimed: true}, nil
	}

	var rec domain.IdempotencyKey
	err := db.Select("response_status", "response_body").
		Where("user_id = ? AND idem_key = ? AND expires_at > ?", userID, key, now).
		Take(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ClaimResult{}, nil
		}
		return ClaimResult{}, translate(err)
	}
	if rec.ResponseStatus == nil {
		return ClaimResult{}, nil
	}
	return ClaimResult{Existing: &domain.StoredResponse{
		StatusCode: *rec.ResponseStatus,
		Body:       rec.ResponseBody,
	}}, nil
}

func (r *idempotencyRepository) SaveResponse(ctx context.Context, userID, key, taskID uuid.UUID, resp domain.StoredResponse) error {
	res := dbFrom(ctx, r.db).Model(&domain.IdempotencyKey{}).
		Where("user_id = ? AND idem_key = ?", userID, key).
		Updates(map[string]any{
			"task_id":         taskID,
			"response_status": resp.StatusCode,
			"response_body":   resp.Body,
		})
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
