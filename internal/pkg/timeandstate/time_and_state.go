package timeandstate

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
	return &AtomicTimeAndState{atomicpointer.New(&TimeAndState{})}
}

func (tas *AtomicTimeAndState) StartTimer() time.Time {
	startTime := tas.Load().StartTime
	if !startTime.IsZero() {
		return startTime
	}

	timeNow := time.Now()
	tas.Update(func(tas *TimeAndState) {
		tas.StartTime = timeNow
	})

	return timeNow
}

func (tas *AtomicTimeAndState) EndTimerWithError(err error) {
	if tas.Load().IsFinished() {
		return
	}

	timeNow := time.Now()
	tas.Update(func(tas *TimeAndState) {
		tas.EndTime = timeNow
		tas.EndError = errorjson.New(err)
	})
}

func (t *AtomicTimeAndState) UnmarshalJSON(data []byte) error {
	if t.AtomicPointer == nil {
		t.AtomicPointer = atomicpointer.New(&TimeAndState{})
	}

	return t.AtomicPointer.UnmarshalJSON(data)
}

// Getters

type TimeAndState struct {
	StartTime time.Time            `json:"start_time"`
	EndTime   time.Time            `json:"end_time"`
	EndError  *errorjson.ErrorJSON `json:"end_error,omitempty"`
}

func (tas *TimeAndState) HasStarted() bool {
	return !tas.StartTime.IsZero()
}

func (tas *TimeAndState) IsFinished() bool {
	return !tas.EndTime.IsZero()
}

func (tas *TimeAndState) DurationOrElapsedTime() (time.Duration, error) {
	startTime := tas.StartTime
	if startTime.IsZero() {
		return time.Duration(0), errors.New("timer has not started yet")
	}

	endTime := tas.EndTime
	if !endTime.IsZero() {
		return endTime.Sub(startTime), nil
	}

	return time.Since(startTime), nil
}

func (tas *TimeAndState) Duration() (time.Duration, error) {
	startTime := tas.StartTime
	if startTime.IsZero() {
		return time.Duration(0), errors.New("timer has not started yet")
	}

	endTime := tas.EndTime
	if endTime.IsZero() {
		return time.Duration(0), errors.New("timer has not ended yet")
	}

	return endTime.Sub(startTime), nil
}
