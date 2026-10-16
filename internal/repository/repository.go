// Package repository defines persistence interfaces and their GORM
// implementations. Services depend only on the interfaces.
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Sentinel errors returned by every repository implementation (GORM and mocks)
// so services can map them to AppErrors without knowing the driver.
var (
	ErrNotFound    = errors.New("repository: record not found")
	ErrDuplicate   = errors.New("repository: duplicate key")
	ErrLockTimeout = errors.New("repository: lock timeout")
)

const (
	pgUniqueViolation = "23505"
	pgLockNotAvailable = "55P03"
)

// TxManager runs fn inside a single database transaction. Returning an error
// or panicking rolls the transaction back; returning nil commits it.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txKey struct{}

type GormTxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *GormTxManager {
	return &GormTxManager{db: db}
}

func (m *GormTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return dbFrom(ctx, m.db).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// dbFrom returns the transaction stored in ctx, or the base connection. This
// lets one repository run both inside and outside a transaction.
func dbFrom(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return base.WithContext(ctx)
}

// translate maps driver errors to repository sentinels.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return errors.Join(ErrDuplicate, err)
		case pgLockNotAvailable:
			return errors.Join(ErrLockTimeout, err)
		}
	}
	return err
}
