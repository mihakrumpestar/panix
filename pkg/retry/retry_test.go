package retry

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskRetry(t *testing.T) {
	t.Parallel()

	require.NotNil(t, NewTaskRetry())
}

func TestWaitTriggered(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	go func() {
		time.Sleep(5 * time.Millisecond)
		retry.Trigger()
	}()

	assert.NoError(t, retry.Wait(ctx))
}

func TestWaitContextCanceled(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	err := retry.Wait(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWaitContextDeadline(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTriggerWakesOneWaiter(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	woken := make(chan struct{}, 1)

	go func() {
		err := retry.Wait(ctx)
		assert.NoError(t, err)

		woken <- struct{}{}
	}()

	time.Sleep(5 * time.Millisecond)
	retry.Trigger()

	select {
	case <-woken:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "waiter was not woken by Trigger")
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

			assert.NoError(t, retry.Wait(ctx))
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
		require.NoError(t, err, "wait goroutine %d", i)
	}

	ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx2)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestConcurrentWaitAndTrigger(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()
	ctx := context.Background()

	var waitGroup sync.WaitGroup

	waitGroup.Go(func() {
		err := retry.Wait(ctx)
		assert.NoError(t, err)
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
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestConcurrentTriggerNoPanic(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	var waitGroup sync.WaitGroup

	for range 20 {
		waitGroup.Go(func() {
			defer func() {
				assert.Nil(t, recover())
			}()

			retry.Trigger()
		})
	}

	waitGroup.Wait()
}

func TestTriggerBeforeWaitIsConsumed(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	retry.Trigger()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTriggerBeforeWaitConsumedOnce(t *testing.T) {
	t.Parallel()

	retry := NewTaskRetry()

	retry.Trigger()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := retry.Wait(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	err = retry.Wait(ctx2)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
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

	require.NoError(t, <-waitDone)

	newWaitDone := make(chan error, 1)

	go func() {
		newWaitDone <- retry.Wait(ctx)
	}()

	time.Sleep(5 * time.Millisecond)
	retry.Trigger()

	assert.NoError(t, <-newWaitDone)
}
