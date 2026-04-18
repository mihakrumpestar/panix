package hook

import "sync"

// Hook provides a lightweight notification mechanism for signaling updates.
// It uses a buffered channel to ensure non-blocking sends.
// Close must be called exactly once when done.
type Hook struct {
	mu   sync.Mutex
	ch   chan struct{}
	once sync.Once
}

// NewHook creates a new Hook with a buffer size of 1.
func NewHook() *Hook {
	return &Hook{ch: make(chan struct{}, 1)}
}

// WaitForUpdate returns a receive-only channel for update notifications.
// The channel is closed when the hook is closed.
func (h *Hook) WaitForUpdate() <-chan struct{} {
	return h.ch
}

// Signal notifies all listeners of an update.
// This is non-blocking and drops updates if the buffer is full.
// Safe to call after Close (no-op).
func (h *Hook) Signal() {
	h.mu.Lock()
	defer h.mu.Unlock()

	select {
	case h.ch <- struct{}{}:
	default:
	}
}

// Close closes the hook, signaling completion to all listeners.
// Safe to call multiple times (subsequent calls are no-ops).
func (h *Hook) Close() {
	h.once.Do(func() {
		h.mu.Lock()
		close(h.ch)
		h.mu.Unlock()
	})
}
