package onceasync

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errFromFn    = errors.New("fn error")
	errFromFnAlt = errors.New("fn alt error")
)

func TestNewOnceAsync(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, NewOnceAsync())
}

func TestDoSuccess(t *testing.T) {
	t.Parallel()

	assert.NoError(t, NewOnceAsync().Do(func() error { return nil }))
}

func TestDoError(t *testing.T) {
	t.Parallel()

	assert.ErrorIs(t, NewOnceAsync().Do(func() error { return errFromFn }), errFromFn)
}

func TestDoRunsOnce(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	var callCount atomic.Int32

	for range 10 {
		_ = onceAsync.Do(func() error {
			callCount.Add(1)

			return errFromFn
		})
	}

	assert.Equal(t, int32(1), callCount.Load())
}

func TestDoReturnsSameError(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	firstErr := onceAsync.Do(func() error { return errFromFn })
	secondErr := onceAsync.Do(func() error { return errFromFnAlt })

	require.ErrorIs(t, firstErr, errFromFn)
	require.ErrorIs(t, secondErr, firstErr)
}

func TestDoConcurrentCallers(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	var callCount atomic.Int32

	var waitGroup sync.WaitGroup

	numCallers := 20

	for range numCallers {
		waitGroup.Go(func() {
			err := onceAsync.Do(func() error {
				time.Sleep(10 * time.Millisecond)
				callCount.Add(1)

				return errFromFn
			})

			assert.ErrorIs(t, err, errFromFn)
		})
	}

	waitGroup.Wait()

	assert.Equal(t, int32(1), callCount.Load())
}

func TestDoBlocksUntilCompletion(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	started := make(chan struct{})
	proceed := make(chan struct{})

	go func() {
		_ = onceAsync.Do(func() error {
			close(started)

			<-proceed

			return errFromFn
		})
	}()

	<-started

	close(proceed)

	assert.ErrorIs(t, onceAsync.Do(func() error { return errFromFnAlt }), errFromFn)
}

func TestDoAfterCompletion(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	firstErr := onceAsync.Do(func() error { return errFromFn })
	secondErr := onceAsync.Do(func() error { return errFromFnAlt })

	assert.ErrorIs(t, secondErr, firstErr)
}

func TestDoSlowFnMultipleWaiters(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	results := make(chan error, 10)

	var waitGroup sync.WaitGroup

	for range 10 {
		waitGroup.Go(func() {
			err := onceAsync.Do(func() error {
				time.Sleep(20 * time.Millisecond)

				return errFromFn
			})

			results <- err
		})
	}

	waitGroup.Wait()
	close(results)

	for err := range results {
		assert.ErrorIs(t, err, errFromFn)
	}
}

func TestDoPanic(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	err := onceAsync.Do(func() error {
		panic("boom")
	})

	require.Error(t, err)
	require.ErrorIs(t, err, errPanic)
	assert.Equal(t, "boom: onceasync: fn panicked", err.Error())
}

func TestDoPanicDoesNotBlockOtherCallers(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	started := make(chan struct{})

	go func() {
		close(started)

		_ = onceAsync.Do(func() error {
			panic("boom")
		})
	}()

	<-started

	require.Error(t, onceAsync.Do(func() error { return errFromFnAlt }))
}

func TestDoPanicReturnsSameErrorToAllCallers(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	var waitGroup sync.WaitGroup

	results := make(chan error, 10)

	for range 10 {
		waitGroup.Go(func() {
			err := onceAsync.Do(func() error {
				panic("shared panic")
			})

			results <- err
		})
	}

	waitGroup.Wait()
	close(results)

	for err := range results {
		require.Error(t, err)
	}
}
