package onceasync

import (
	"sync"

	"github.com/pkg/errors"
)

var errPanic = errors.New("onceasync: fn panicked")

// OnceAsync ensures a func() error runs exactly once.
// All callers wait for completion and get the same error result.
// If task panics, the panic is recovered and returned as an error.
//
// Synchronization Guarantees:
// The result field is safely shared between goroutines due to the
// memory ordering guarantees of sync.Once and channel operations:
//   - sync.Once ensures fn() executes exactly once
//   - The channel close happens-after the result assignment
//   - Channel receive happens-before reading the result field
//   - Therefore, all goroutines see the fully written result
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

// Do executes task exactly once and blocks until completion.
// Returns the error from the first (and only) execution.
// If task panics, the panic is recovered and returned as an error,
// ensuring no callers block forever.
func (oa *OnceAsync) Do(task func() error) error {
	oa.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				oa.result = errors.Wrapf(errPanic, "%v", r)
			}

			close(oa.done)
		}()

		oa.result = task()
	})
	<-oa.done // Wait for completion (immediate if already done)

	return oa.result
}
