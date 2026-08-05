package retry

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

// Retry provides a mechanism for goroutines to wait and be signaled to retry.
// It is safe for concurrent use by multiple goroutines.
//
// Trigger closes the current channel, waking ALL waiting goroutines, then
// creates a fresh channel for the next Wait cycle.
type Retry struct {
	mu      sync.Mutex
	trigger chan struct{}
}

// NewTaskRetry creates a new Retry instance.
func NewTaskRetry() *Retry {
	return &Retry{
		trigger: make(chan struct{}),
	}
}

// Wait blocks until Trigger is called or context is cancelled.
// Returns ctx.Err() if context is cancelled before a trigger.
// Multiple goroutines may call Wait concurrently, all are woken by a single
// Trigger (broadcast via channel close).
func (r *Retry) Wait(ctx context.Context) error {
	r.mu.Lock()
	ch := r.trigger
	r.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "context canceled")
	}
}

// Trigger broadcasts to all goroutines currently waiting in Wait
// by closing the channel and creating a fresh one.
// Safe to call concurrently.
func (r *Retry) Trigger() {
	r.mu.Lock()
	defer r.mu.Unlock()

	close(r.trigger)
	r.trigger = make(chan struct{})
}
