// Package mocks provides thread-safe in-memory fakes of the repository layer
// so services can be unit tested without PostgreSQL.
package mocks

import (
	"context"
	"sync"
	"time"
)

type fakeTxKey struct{}

// fakeTx collects commit/rollback callbacks registered by the in-memory
// repositories, emulating what a real transaction would make visible or undo.
type fakeTx struct {
	mu         sync.Mutex
	onCommit   []func()
	onRollback []func()
}

func (t *fakeTx) commit() {
	t.mu.Lock()
	fns := t.onCommit
	t.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func (t *fakeTx) rollback() {
	t.mu.Lock()
	fns := t.onRollback
	t.mu.Unlock()
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

// inTx registers commit/rollback callbacks when ctx carries a fake
// transaction. Outside a transaction the change is final, so commit runs now.
func inTx(ctx context.Context, onCommit, onRollback func()) {
	tx, ok := ctx.Value(fakeTxKey{}).(*fakeTx)
	if !ok {
		if onCommit != nil {
			onCommit()
		}
		return
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if onCommit != nil {
		tx.onCommit = append(tx.onCommit, onCommit)
	}
	if onRollback != nil {
		tx.onRollback = append(tx.onRollback, onRollback)
	}
}

// FakeTxManager implements repository.TxManager. An error or panic from fn
// rolls back every change the in-memory repositories made inside it.
type FakeTxManager struct{}

func NewFakeTxManager() *FakeTxManager { return &FakeTxManager{} }

func (m *FakeTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(fakeTxKey{}).(*fakeTx); ok {
		return fn(ctx) // join the outer transaction
	}

	tx := &fakeTx{}
	ctx = context.WithValue(ctx, fakeTxKey{}, tx)
	defer func() {
		if r := recover(); r != nil {
			tx.rollback()
			panic(r)
		}
	}()

	if err := fn(ctx); err != nil {
		tx.rollback()
		return err
	}
	tx.commit()
	return nil
}

// FakeClock is a manually advanced clock.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start.UTC().Truncate(time.Second)}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
