package retry

import "context"

// Retry provides a mechanism for goroutines to wait and be signaled to retry.
// It is safe for concurrent use by multiple goroutines.
type Retry struct {
	trigger chan struct{}
}

// NewTaskRetry creates a new Retry instance.
func NewTaskRetry() *Retry {
	return &Retry{
		trigger: make(chan struct{}),
	}
}

// Wait blocks until Trigger is called or context is cancelled.
// After waking up, it is ready to wait for the next trigger.
// Returns ctx.Err() if context is cancelled before trigger.
func (r *Retry) Wait(ctx context.Context) error {
	select {
	case <-r.trigger:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Trigger wakes up all goroutines currently waiting in Wait.
// It is safe to call Trigger multiple times.
func (r *Retry) Trigger() {
	close(r.trigger)
	r.trigger = make(chan struct{})
}
