package time_and_state

import (
	"errors"
	"time"

	"go.uber.org/atomic"
)

type TimeAndState struct {
	startTime *atomic.Time
	endTime   *atomic.Time
	endErr    *atomic.Error
}

func NewTimeAndState() *TimeAndState {
	return &TimeAndState{
		&atomic.Time{},
		&atomic.Time{},
		&atomic.Error{},
	}
}

func NewTimeAndStateCustom(startTime, endTime time.Time, endErr error) *TimeAndState {
	return &TimeAndState{
		atomic.NewTime(startTime),
		atomic.NewTime(endTime),
		atomic.NewError(endErr),
	}
}

func (tas *TimeAndState) StartTimer() time.Time {
	if tas.HasStarted() {
		return tas.startTime.Load()
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

func (tas *TimeAndState) GetEndTime() time.Time {
	return tas.endTime.Load()
}

func (tas *TimeAndState) IsFinished() bool {
	return !tas.endTime.Load().IsZero()
}

func (tas *TimeAndState) GetEndError() error {
	return tas.endErr.Load()
}

func (tas *TimeAndState) DurationOrElapsedTime() (time.Duration, error) {
	duration, err := tas.Duration()
	if err == nil {
		return duration, nil
	}

	duration, err = tas.ElapsedTime()
	if err == nil {
		return duration, nil
	}

	return time.Duration(0), err
}

func (tas *TimeAndState) Duration() (time.Duration, error) {
	if !tas.HasStarted() {
		return time.Duration(0), errors.New("timer has not started yet")
	}

	if !tas.IsFinished() {
		return time.Duration(0), errors.New("timer has ended yet")
	}

	return tas.GetEndTime().Sub(tas.GetStartTime()), nil
}

func (tas *TimeAndState) ElapsedTime() (time.Duration, error) {
	if !tas.HasStarted() {
		return time.Duration(0), errors.New("timer has not started yet")
	}

	return time.Since(tas.GetStartTime()), nil
}
