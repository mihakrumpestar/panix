package hook

import (
	"sync"
	"testing"
	"time"
)

func TestNewHook(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	if hook == nil {
		t.Fatal("NewHook() returned nil")
	}

	ch := hook.WaitForUpdate()
	if ch == nil {
		t.Fatal("WaitForUpdate() returned nil")
	}
}

func TestSignal(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	select {
	case <-hook.WaitForUpdate():
		t.Fatal("channel should not receive before Signal()")
	default:
	}

	hook.Signal()

	select {
	case <-hook.WaitForUpdate():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel should receive after Signal()")
	}
}

func TestSignalDropsOnFullBuffer(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	hook.Signal()
	hook.Signal()

	count := 0

	select {
	case <-hook.WaitForUpdate():
		count++
	default:
	}

	select {
	case <-hook.WaitForUpdate():
		count++
	default:
	}

	if count != 1 {
		t.Errorf("received %d signals, want 1 (buffer size 1 drops extras)", count)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	hook.Close()

	notifyCh := hook.WaitForUpdate()
	select {
	case _, ok := <-notifyCh:
		if ok {
			t.Fatal("channel should be closed, not open")
		}
	default:
		t.Fatal("channel should be closed immediately after Close()")
	}
}

func TestCloseIdempotent(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	hook.Close()
	hook.Close()
	hook.Close()

	notifyCh := hook.WaitForUpdate()
	for range 3 {
		_, ok := <-notifyCh
		if ok {
			t.Fatal("channel should remain closed after multiple Close() calls")
		}
	}
}

func TestSignalAfterClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	hook.Close()

	hook.Signal()
}

func TestSignalAndClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	notifyCh := hook.WaitForUpdate()

	hook.Signal()
	hook.Close()

	_, ok := <-notifyCh
	if !ok {
		t.Fatal("should receive the signaled value on the pre-close channel")
	}

	_, ok = <-hook.WaitForUpdate()
	if ok {
		t.Fatal("WaitForUpdate() after Close should return closed channel")
	}
}

func TestMultipleListeners(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	received := make(chan struct{}, 3)

	var waitGroup sync.WaitGroup

	for range 3 {
		waitGroup.Go(func() {
			notifyCh := hook.WaitForUpdate()

			select {
			case <-notifyCh:
				received <- struct{}{}
			case <-time.After(200 * time.Millisecond):
			}
		})
	}

	time.Sleep(10 * time.Millisecond)

	hook.Signal()

	waitGroup.Wait()
	close(received)

	count := len(received)
	if count == 0 {
		t.Error("no listeners received the signal")
	}

	t.Logf("listeners received: %d/3 (buffer size 1 means only 1 gets the value)", count)
}

func TestConcurrentSignal(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	var waitGroup sync.WaitGroup

	for range 20 {
		waitGroup.Go(func() {
			hook.Signal()
		})
	}

	waitGroup.Wait()

	select {
	case <-hook.WaitForUpdate():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("should receive at least one signal from concurrent Signal() calls")
	}
}

func TestConcurrentSignalAndClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	var waitGroup sync.WaitGroup

	for idx := range 20 {
		waitGroup.Add(1)

		go func(index int) {
			defer waitGroup.Done()

			if index%2 == 0 {
				hook.Signal()
			} else {
				hook.Close()
			}
		}(idx)
	}

	waitGroup.Wait()
}
