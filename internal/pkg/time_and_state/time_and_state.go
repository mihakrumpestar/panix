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

	target TimeAndStateNode
}

// TimeAndStateNode is an interface for objects that contain a TimeAndState
// in a tree-like structure
type TimeAndStateNode interface {
	// Note: target.GetTimeAndState() and tas.(TimeAndState) are the same,
	// except for phaseLogs (tas is there for phase, target is there for machine)
	GetTimeAndState() *TimeAndState
	GetParent() TimeAndStateNode
	GetChildren() []TimeAndStateNode
}

func NewTimeAndState(target TimeAndStateNode) *TimeAndState {
	return &TimeAndState{
		&atomic.Time{},
		&atomic.Time{},
		&atomic.Error{},
		target,
	}
}

func (tas *TimeAndState) StartTimer() {
	if tas.HasStarted() {
		return
	}

	timeNow := time.Now()

	// Here we set the phase TimeAndState, while the recursive works on targets
	tas.startTime.Store(timeNow)

	if tas.target == nil {
		return
	}

	tas.target.GetTimeAndState().startTimerRecursively(timeNow)
}

func (tas *TimeAndState) startTimerRecursively(timeNow time.Time) {
	if tas.HasStarted() {
		return
	}

	tas.startTime.Store(timeNow)

	if tas.IsFinished() {
		// If timeNow is newer than target endTime, we need to reset it,
		// since a new phase was started
		if timeNow.After(tas.GetEndTime()) {
			tas.endTime.Store(time.Time{})
			tas.endError.Store(nil)
		}
	}

	if tas.target == nil {
		return
	}

	// Propagate timer start to parent recursively
	parent := tas.target.GetParent()
	if parent != nil {
		parent.GetTimeAndState().startTimerRecursively(timeNow)
	}
}

func (tas *TimeAndState) EndTimerWithError(err error) {
	if tas.IsFinished() {
		return
	}

	timeNow := time.Now()

	tas.endTime.Store(timeNow)
	tas.endError.Store(err)

	if tas.target == nil {
		return
	}

	tas.target.GetTimeAndState().endTimerWithErrorRecursively(timeNow, err)
}

func (tas *TimeAndState) endTimerWithErrorRecursively(timeNow time.Time, err error) {
	if tas.IsFinished() {
		return
	}

	// At least one of the children did not finish, which means
	// we should not stop this one
	if !tas.DidChildrenFinish() {
		return
	}

	tas.endTime.Store(timeNow)
	tas.endError.Store(err)

	// Propagate timer end to parent recursively
	parent := tas.target.GetParent()
	if parent != nil {
		parent.GetTimeAndState().endTimerWithErrorRecursively(timeNow, err)
	}
}

func (tas *TimeAndState) DidChildrenFinish() bool {
	for _, child := range tas.target.GetChildren() {
		if !child.GetTimeAndState().IsFinished() {
			return false
		}
	}

	return true
}

func (tas *TimeAndState) Clear() {
	tas.startTime.Store(time.Time{})
	tas.endTime.Store(time.Time{})
	tas.endError.Store(nil)

	if tas.target == nil {
		return
	}

	tas.target.GetTimeAndState().clearRecursive()
}

// clearRecursive should clear endTimer and endError, but not startTime,
// since that one does not reset when re restart the phase
func (tas *TimeAndState) clearRecursive() {
	tas.endTime.Store(time.Time{})
	tas.endError.Store(nil)

	tas.target.GetParent().GetTimeAndState().clearRecursive()
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
