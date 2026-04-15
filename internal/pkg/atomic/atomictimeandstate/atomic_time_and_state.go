package atomictimeandstate

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
	"github.com/pkg/errors"
)

type AtomicTimeAndState struct {
	*atomicpointer.AtomicPointer[TimeAndState]
}

func New() *AtomicTimeAndState {
	return &AtomicTimeAndState{atomicpointer.New(&TimeAndState{live: true})}
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
		tas.AtomicPointer = atomicpointer.New(&TimeAndState{})
	}

	return tas.AtomicPointer.UnmarshalJSON(data)
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
