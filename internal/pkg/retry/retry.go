package retry

import "sync"

// Retry provides a mechanism for goroutines to wait and be signaled to retry.
// It is safe for concurrent use by multiple goroutines.
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

// Wait blocks until Trigger is called.
// After waking up, it is ready to wait for the next trigger.
func (r *Retry) Wait() {
	r.mu.Lock()
	ch := r.trigger
	r.mu.Unlock()
	<-ch
}

// Trigger wakes up all goroutines currently waiting in Wait.
// It is safe to call Trigger multiple times.
func (r *Retry) Trigger() {
	r.mu.Lock()
	close(r.trigger)
	r.trigger = make(chan struct{})
	r.mu.Unlock()
}
