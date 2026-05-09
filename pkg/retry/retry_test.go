package retry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewTaskRetry(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	if retry == nil {
		t.Fatal("NewTaskRetry() returned nil")
	}
}

func TestWaitTriggered(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	go func() {
		time.Sleep(5 * time.Millisecond)
		retry.Trigger()
	}()

	err := retry.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() = %v, want nil", err)
	}
}

func TestWaitContextCanceled(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	err := retry.Wait(ctx)
	if err == nil {
		t.Error("Wait() should return error when context is canceled")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false, got %v", err)
	}
}

func TestWaitContextDeadline(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx)
	if err == nil {
		t.Error("Wait() should return error when deadline exceeds")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, got %v", err)
	}
}

func TestTriggerWakesOneWaiter(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	woken := make(chan struct{}, 1)

	go func() {
		err := retry.Wait(ctx)
		if err != nil {
			t.Errorf("Wait() = %v, want nil", err)

			return
		}

		woken <- struct{}{}
	}()

	time.Sleep(5 * time.Millisecond)
	retry.Trigger()

	select {
	case <-woken:
	case <-time.After(100 * time.Millisecond):
		t.Error("waiter was not woken by Trigger")
	}
}

func TestMultipleWaitTriggerCycles(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	for range 3 {
		done := make(chan struct{})

		go func() {
			defer close(done)

			err := retry.Wait(ctx)
			if err != nil {
				t.Errorf("Wait() cycle = %v, want nil", err)
			}
		}()

		time.Sleep(2 * time.Millisecond)
		retry.Trigger()

		<-done
	}
}

func TestTriggerMultipleTimesSequential(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	// With broadcast semantics, triggers without waiters are no-ops.
	// Verify that trigger AFTER waiters start broadcasts to all of them.
	var waitGroup sync.WaitGroup

	count := 3
	results := make([]error, count)

	for idx := range count {
		waitGroup.Add(1)

		go func(idx int) {
			defer waitGroup.Done()

			results[idx] = retry.Wait(ctx)
		}(idx)
	}

	time.Sleep(5 * time.Millisecond)
	retry.Trigger()

	waitGroup.Wait()

	for i, err := range results {
		if err != nil {
			t.Errorf("wait goroutine %d: Wait() = %v, want nil (broadcast should wake all)", i, err)
		}
	}

	// After broadcast, next Wait should block (channel is fresh).
	ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx2)
	if err == nil {
		t.Error("Wait() after broadcast should block (no pending trigger)")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, got %v", err)
	}
}

func TestConcurrentWaitAndTrigger(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	var waitGroup sync.WaitGroup

	waitGroup.Go(func() {
		err := retry.Wait(ctx)
		if err != nil {
			t.Errorf("Wait() = %v, want nil", err)
		}
	})

	time.Sleep(5 * time.Millisecond)
	retry.Trigger()

	waitGroup.Wait()
}

func TestWaitReturnsWrappedError(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	err := retry.Wait(ctx)
	if err == nil {
		t.Fatal("Wait() should return error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false, got %v", err)
	}
}

func TestConcurrentTriggerNoPanic(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	var waitGroup sync.WaitGroup

	for range 20 {
		waitGroup.Go(func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Errorf("Trigger() panicked: %v", rec)
				}
			}()

			retry.Trigger()
		})
	}

	waitGroup.Wait()
}

func TestTriggerBeforeWaitIsConsumed(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	// With broadcast semantics, Trigger before Wait is a no-op
	// (no waiters to broadcast to). Wait must block.
	retry.Trigger()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx)
	if err == nil {
		t.Error("Wait() after pre-Trigger should block (broadcast semantics, no pre-signal)")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, got %v", err)
	}
}

func TestTriggerBeforeWaitConsumedOnce(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	// Trigger before any waiter → broadcast is a no-op, channel is fresh.
	// Both Waits must block.
	retry.Trigger()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx)
	if err == nil {
		t.Error("first Wait() after pre-Trigger should block")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, got %v", err)
	}

	// Second Wait also blocks — channel is still empty.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	err = retry.Wait(ctx2)
	if err == nil {
		t.Error("second Wait() should also block")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, got %v", err)
	}
}

func TestTriggerWakesWaitThenResets(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	waitDone := make(chan error, 1)

	go func() {
		waitDone <- retry.Wait(ctx)
	}()

	time.Sleep(5 * time.Millisecond)
	retry.Trigger()

	err := <-waitDone
	if err != nil {
		t.Errorf("first Wait() = %v, want nil", err)
	}

	newWaitDone := make(chan error, 1)

	go func() {
		newWaitDone <- retry.Wait(ctx)
	}()

	time.Sleep(5 * time.Millisecond)
	retry.Trigger()

	err = <-newWaitDone
	if err != nil {
		t.Errorf("second Wait() = %v, want nil", err)
	}
}
