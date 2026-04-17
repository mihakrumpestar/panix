package atomictimeandstate

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
	"github.com/pkg/errors"
)

// AtomicTimeAndState is a wrapper around atomicpointer.AtomicPointer for easier access
type AtomicTimeAndState struct {
	*atomicpointer.AtomicPointer[TimeAndState]
}

func New() *AtomicTimeAndState {
	tas := atomicpointer.New[TimeAndState]()
	tas.Store(&TimeAndState{live: true})

	return &AtomicTimeAndState{tas}
}

func (tas *AtomicTimeAndState) StartTimer() {
	startTime := tas.Load().StartTime
	if !startTime.IsZero() {
		return
	}

	timeNow := time.Now()

	tas.Update(func(tas *TimeAndState) {
		tas.StartTime = timeNow
	})
}

func (tas *AtomicTimeAndState) EndTimerWithError(err error) {
	if tas.Load().IsFinished() {
		return
	}

	timeNow := time.Now()

	tas.Update(func(tas *TimeAndState) {
		tas.DurationCache = timeNow.Sub(tas.StartTime)
		tas.EndTime = timeNow
		tas.EndError = errorjson.New(err)
	})
}

func (tas *AtomicTimeAndState) UnmarshalJSON(data []byte) error {
	if tas.AtomicPointer == nil {
		tas.AtomicPointer = atomicpointer.New[TimeAndState]()
	}

	return errors.Wrap(tas.AtomicPointer.UnmarshalJSON(data), "unmarshal time and state")
}

func (tas *AtomicTimeAndState) DurationOrElapsedTime() (time.Duration, error) {
	tasI := tas.Load()

	if tasI.StartTime.IsZero() {
		return 0, errors.New("timer has not started yet")
	}

	if !tasI.EndTime.IsZero() {
		return tasI.DurationCache, nil
	}

	if !tasI.live {
		return tasI.DurationCache, nil
	}

	durationCache := time.Since(tasI.StartTime)

	tas.Update(func(tas *TimeAndState) {
		tas.DurationCache = durationCache
	})

	return durationCache, nil
}
