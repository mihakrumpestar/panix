package timeandstate

import (
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"
)

type TimeAndState struct {
	startTime atomic.Time
	endTime   atomic.Time
	endErr    atomic.Error
}

func NewTimeAndState() *TimeAndState {
	return &TimeAndState{}
}

func NewTimeAndStateCustom(startTime, endTime time.Time, endErr error) *TimeAndState {
	tas := &TimeAndState{}
	tas.startTime.Store(startTime)
	tas.endTime.Store(endTime)
	tas.endErr.Store(endErr)
	return tas
}

func (tas *TimeAndState) StartTimer() time.Time {
	startTime := tas.startTime.Load()
	if !startTime.IsZero() {
		return startTime
	}

	timeNow := time.Now()
	tas.startTime.Store(timeNow)

	return timeNow
}

func (tas *TimeAndState) EndTimerWithError(err error) {
	if tas.IsFinished() {
		return
	}

	timeNow := time.Now()
	tas.endTime.Store(timeNow)
	tas.endErr.Store(err)
}

func (tas *TimeAndState) Clear() {
	tas.startTime.Store(time.Time{})
	tas.endTime.Store(time.Time{})
	tas.endErr.Store(nil)
}

// Getters

func (tas *TimeAndState) GetStartTime() time.Time {
	return tas.startTime.Load()
}

func (tas *TimeAndState) HasStarted() bool {
	return !tas.startTime.Load().IsZero()
}

func (tas *TimeAndState) IsFinished() bool {
	return !tas.endTime.Load().IsZero()
}

func (tas *TimeAndState) GetEndError() error {
	err := tas.endErr.Load()
	if err != nil {
		return errors.Wrap(err, "end error")
	}
	return nil
}

func (tas *TimeAndState) DurationOrElapsedTime() (time.Duration, error) {
	startTime := tas.startTime.Load()
	if startTime.IsZero() {
		return time.Duration(0), errors.New("timer has not started yet")
	}

	endTime := tas.endTime.Load()
	if !endTime.IsZero() {
		return endTime.Sub(startTime), nil
	}

	return time.Since(startTime), nil
}

func (tas *TimeAndState) Duration() (time.Duration, error) {
	startTime := tas.startTime.Load()
	if startTime.IsZero() {
		return time.Duration(0), errors.New("timer has not started yet")
	}

	endTime := tas.endTime.Load()
	if endTime.IsZero() {
		return time.Duration(0), errors.New("timer has not ended yet")
	}

	return endTime.Sub(startTime), nil
}

func (tas *TimeAndState) ElapsedTime() (time.Duration, error) {
	startTime := tas.startTime.Load()
	if startTime.IsZero() {
		return time.Duration(0), errors.New("timer has not started yet")
	}

	return time.Since(startTime), nil
}
