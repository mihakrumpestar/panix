package retry

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

// Retry provides a mechanism for goroutines to wait and be signaled to retry.
// It is safe for concurrent use by multiple goroutines.
//
// Trigger sets a signal that is consumed by the next Wait call.
// If Trigger is called before Wait, Wait returns immediately.
// Each Trigger unblocks exactly one Wait (the signal is not a broadcast).
type Retry struct {
	mu       sync.Mutex
	cond     *sync.Cond
	signaled bool
}

// NewTaskRetry creates a new Retry instance.
func NewTaskRetry() *Retry {
	r := &Retry{}
	r.cond = sync.NewCond(&r.mu)

	return r
}

// Wait blocks until Trigger is called or context is cancelled.
// If Trigger was called before Wait, Wait returns immediately
// (the signal persists until consumed).
// Returns ctx.Err() if context is cancelled before trigger.
func (r *Retry) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.signaled {
		r.signaled = false

		return nil
	}

	cancelCh := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			r.cond.Broadcast()
		case <-cancelCh:
		}
	}()

	for !r.signaled {
		if ctx.Err() != nil {
			close(cancelCh)

			return errors.Wrap(ctx.Err(), "context canceled")
		}

		r.cond.Wait()
	}

	r.signaled = false

	close(cancelCh)

	return nil
}

// Trigger signals the next Wait call to return.
// It is safe to call Trigger multiple times.
// If no goroutine is currently waiting, the signal persists until the next Wait.
func (r *Retry) Trigger() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.signaled = true
	r.cond.Broadcast()
}
