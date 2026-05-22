package hook

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHook(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	require.NotNil(t, hook)
	require.NotNil(t, hook.WaitForUpdate())
}

func TestSignal(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	select {
	case <-hook.WaitForUpdate():
		require.FailNow(t, "channel should not receive before Signal()")
	default:
	}

	hook.Signal()

	select {
	case <-hook.WaitForUpdate():
	case <-time.After(100 * time.Millisecond):
		require.FailNow(t, "channel should receive after Signal()")
	}
}

func TestBufferedSignal(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	hook.Signal()
	hook.Signal()

	select {
	case <-hook.WaitForUpdate():
	case <-time.After(100 * time.Millisecond):
		require.FailNow(t, "channel should receive after Signal()")
	}
}

func TestClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	hook.Close()

	select {
	case <-hook.WaitForUpdate():
	case <-time.After(100 * time.Millisecond):
		require.FailNow(t, "channel should receive after Close()")
	}
}

func TestDoubleClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	hook.Close()
	hook.Close()
}

func TestSignalAfterClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	hook.Close()

	hook.Signal()
	hook.Signal()
}

func TestMultipleWaiters(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			select {
			case <-hook.WaitForUpdate():
			case <-time.After(time.Second):
			}
		})
	}

	time.Sleep(10 * time.Millisecond)
	hook.Signal()
	wg.Wait()
}

func TestWaitForUpdateAfterClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	hook.Close()

	_ = hook.WaitForUpdate()
}

func TestConcurrentSignalAndClose(t *testing.T) {
	t.Parallel()

	hook := NewHook()

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for range 100 {
			hook.Signal()
		}
	}()

	go func() {
		defer wg.Done()

		for range 100 {
			hook.Close()
		}
	}()

	wg.Wait()
}

func TestHookNotifiesAfterSignal(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	ch := hook.WaitForUpdate()
	hook.Signal()

	select {
	case <-ch:
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for notification")
	}
}

func TestSignalOnlyNotifiesOnce(t *testing.T) {
	t.Parallel()

	hook := NewHook()
	ch := hook.WaitForUpdate()
	hook.Signal()

	select {
	case _, ok := <-ch:
		assert.True(t, ok)
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for notification")
	}

	select {
	case <-ch:
		require.FailNow(t, "should not receive again on same channel")
	default:
	}
}

func TestConcurrentWaitAndSignal(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		hook := NewHook()
		hook.WaitForUpdate()
	}()

	go func() {
		defer wg.Done()

		hook := NewHook()
		hook.Signal()
	}()

	wg.Wait()
}
