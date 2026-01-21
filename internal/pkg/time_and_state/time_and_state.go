package time_and_state

import (
	"errors"
	"time"

	"go.uber.org/atomic"
)

type TimeAndState struct {
	startTime *atomic.Time
	endTime   *atomic.Time
	endError  *atomic.Error
}

func NewTimeAndState() *TimeAndState {
	return &TimeAndState{
		startTime: &atomic.Time{},
		endTime:   &atomic.Time{},
		endError:  &atomic.Error{},
	}
}

func (tas *TimeAndState) StartTimer() {
	tas.startTime.Store(time.Now())
}

func (tas *TimeAndState) EndTimer() {
	tas.endTime.Store(time.Now())
}

func (tas *TimeAndState) EndTimerWithError(err error) {
	tas.EndTimer()
	tas.endError.Store(err)
}

// WithEnd returns current start with endTas end
func (tas *TimeAndState) WithEnd(endTas *TimeAndState) *TimeAndState {
	if endTas == nil {
		return tas
	}

	return &TimeAndState{
		startTime: tas.startTime,
		endTime:   endTas.endTime,
		endError:  endTas.endError,
	}
}

func (tas *TimeAndState) Clear() {
	tas.startTime.Store(time.Time{})
	tas.endTime.Store(time.Time{})
	tas.endError.Store(nil)
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
	return tas.endError.Load()
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
