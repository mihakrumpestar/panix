package pool

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

var errSentinel = errors.New("sentinel error")

func TestWaitNilWhenAllSucceed(t *testing.T) {
	t.Parallel()

	p := New(2, context.Background())
	group := p.NewGroup()

	for range 4 {
		group.SubmitErr(func() error { return nil })
	}

	err := group.Wait()
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}
}

func TestWaitReturnsFirstError(t *testing.T) {
	t.Parallel()

	p := New(2, context.Background())
	group := p.NewGroup()

	for i := range 3 {
		group.SubmitErr(func() error {
			if i == 1 {
				return errSentinel
			}

			return nil
		})
	}

	err := group.Wait()
	if !errors.Is(err, errSentinel) {
		t.Fatalf("Wait() = %v, want %v", err, errSentinel)
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	p := New(2, parent)
	group := p.NewGroup()
	groupCtx := group.Context()

	group.SubmitErr(func() error {
		<-groupCtx.Done()

		return groupCtx.Err()
	})

	cancel()

	err := group.Wait()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() = %v, want context.Canceled", err)
	}
}

func TestSubmitErrSkipsTaskWhenAlreadyCancelled(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	p := New(0, parent) // unlimited pool: tasks would run immediately if not skipped
	group := p.NewGroup()

	cancel()

	ran := false

	group.SubmitErr(func() error {
		ran = true

		return nil
	})

	err := group.Wait()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() = %v, want context.Canceled", err)
	}

	if ran {
		t.Fatal("task ran despite the group context being already cancelled")
	}
}

func TestPanicRecoveredAsError(t *testing.T) {
	t.Parallel()

	p := New(2, context.Background())
	group := p.NewGroup()

	group.SubmitErr(func() error {
		panic("boom")
	})

	err := group.Wait()
	if err == nil {
		t.Fatal("Wait() = nil, want recovered panic error")
	}
}

func TestConcurrencyLimit(t *testing.T) {
	t.Parallel()

	p := New(1, context.Background())
	group := p.NewGroup()

	const tasks = 3

	entered := make(chan int, tasks)
	release := make([]chan struct{}, 0, tasks)

	var mu sync.Mutex

	active := 0
	maxActive := 0

	for i := range tasks {
		idx := i
		rel := make(chan struct{})
		release = append(release, rel)

		group.SubmitErr(func() error {
			mu.Lock()

			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()

			entered <- idx

			<-rel

			mu.Lock()
			active--
			mu.Unlock()

			return nil
		})
	}

	// Only one task can be inside the critical section at a time, so entered
	// signals arrive one per released gate, never in a burst. Release the gate
	// of whichever task just entered, since entry order is not guaranteed.
	for i := range tasks {
		var idx int
		select {
		case idx = <-entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for task %d to enter", i)
		}

		close(release[idx])
	}

	err := group.Wait()
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}

	if maxActive != 1 {
		t.Fatalf("max concurrent tasks = %d, want 1", maxActive)
	}
}
