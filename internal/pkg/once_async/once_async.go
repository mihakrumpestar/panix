package once_async

import (
	"sync"
)

// OnceAsync ensures a func() error runs exactly once.
// All callers wait for completion and get the same error result.
type OnceAsync struct {
	once   sync.Once
	done   chan struct{} // Closed after execution
	result error         // Stores the error from fn
}

// NewOnceAsync creates a new OnceAsync.
func NewOnceAsync() *OnceAsync {
	return &OnceAsync{
		done: make(chan struct{}),
	}
}

// Do executes fn (which returns error) exactly once and blocks until completion.
// Returns the error from the first (and only) execution.
func (oa *OnceAsync) Do(fn func() error) error {
	oa.once.Do(func() {
		oa.result = fn() // Capture the error
		close(oa.done)   // Unblock all waiters
	})
	<-oa.done // Wait for completion (immediate if already done)
	return oa.result
}
