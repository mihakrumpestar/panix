package onceasync

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	errFromFn    = errors.New("fn error")
	errFromFnAlt = errors.New("fn alt error")
)

func TestNewOnceAsync(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()
	if onceAsync == nil {
		t.Fatal("NewOnceAsync() returned nil")
	}
}

func TestDoSuccess(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	err := onceAsync.Do(func() error { return nil })
	if err != nil {
		t.Errorf("Do() = %v, want nil", err)
	}
}

func TestDoError(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	err := onceAsync.Do(func() error { return errFromFn })
	if !errors.Is(err, errFromFn) {
		t.Errorf("Do() = %v, want %v", err, errFromFn)
	}
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

	if callCount.Load() != 1 {
		t.Errorf("fn called %d times, want 1", callCount.Load())
	}
}

func TestDoReturnsSameError(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	firstErr := onceAsync.Do(func() error { return errFromFn })
	secondErr := onceAsync.Do(func() error { return errFromFnAlt })

	if !errors.Is(firstErr, errFromFn) {
		t.Errorf("first Do() = %v, want %v", firstErr, errFromFn)
	}

	if !errors.Is(secondErr, firstErr) {
		t.Errorf("second Do() = %v, want same error as first %v", secondErr, firstErr)
	}
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

			if !errors.Is(err, errFromFn) {
				t.Errorf("Do() = %v, want %v", err, errFromFn)
			}
		})
	}

	waitGroup.Wait()

	if callCount.Load() != 1 {
		t.Errorf("fn called %d times, want 1", callCount.Load())
	}
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

	err := onceAsync.Do(func() error { return errFromFnAlt })
	if !errors.Is(err, errFromFn) {
		t.Errorf("Do() = %v, want %v", err, errFromFn)
	}
}

func TestDoAfterCompletion(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	firstErr := onceAsync.Do(func() error { return errFromFn })

	secondErr := onceAsync.Do(func() error { return errFromFnAlt })
	if !errors.Is(secondErr, firstErr) {
		t.Errorf("Do() after completion = %v, want %v", secondErr, firstErr)
	}
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
		if !errors.Is(err, errFromFn) {
			t.Errorf("Do() = %v, want %v", err, errFromFn)
		}
	}
}

func TestDoPanic(t *testing.T) {
	t.Parallel()

	onceAsync := NewOnceAsync()

	err := onceAsync.Do(func() error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("Do() returned nil error after panic")
	}

	if !errors.Is(err, errPanic) {
		t.Errorf("errors.Is(err, errPanic) = false, want true")
	}

	if err.Error() != "onceasync: fn panicked: boom" {
		t.Errorf("Do() = %q, want %q", err.Error(), "onceasync: fn panicked: boom")
	}
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

	err := onceAsync.Do(func() error { return errFromFnAlt })
	if err == nil {
		t.Fatal("Do() after panic returned nil")
	}
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
		if err == nil {
			t.Error("Do() returned nil after panic")
		}
	}
}
