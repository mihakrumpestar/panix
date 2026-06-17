package atomictimeandstate

import (
	"time"

	"github.com/mihakrumpestar/panix/pkg/jsonerror"
	"github.com/pkg/errors"
)

type TimeAndState struct {
	StartTime     time.Time            `json:"start_time"`
	EndTime       time.Time            `json:"end_time"`
	DurationCache time.Duration        `json:"duration"`
	EndError      *jsonerror.JSONError `json:"end_error,omitempty"`

	live        bool
	stateVersion uint64
}

func (tas *TimeAndState) HasStarted() bool {
	return !tas.StartTime.IsZero()
}

func (tas *TimeAndState) IsFinished() bool {
	return !tas.EndTime.IsZero()
}

func (tas *TimeAndState) Duration() (time.Duration, error) {
	if tas.StartTime.IsZero() {
		return 0, errors.New("timer has not started yet")
	}

	if tas.EndTime.IsZero() {
		return 0, errors.New("timer has not ended yet")
	}

	return tas.DurationCache, nil // Already calculated by EndTimerWithError
}

// SetStarted marks the timer as started and increments stateVersion.
func (tas *TimeAndState) SetStarted(startTime time.Time) {
	tas.StartTime = startTime
	tas.live = true
	tas.stateVersion++
}

// SetFinished marks the timer as finished, caches the duration, and increments stateVersion.
func (tas *TimeAndState) SetFinished(endTime time.Time, err error) {
	tas.DurationCache = endTime.Sub(tas.StartTime)
	tas.EndTime = endTime
	tas.EndError = jsonerror.New(err)
	tas.stateVersion++
}

// MarkFinished sets EndTime and bumps stateVersion, but only if not already finished.
// Unlike SetFinished, this does NOT overwrite DurationCache — the caller is
// responsible for setting the duration separately (e.g. via SetDuration).
func (tas *TimeAndState) MarkFinished() {
	if !tas.IsFinished() {
		tas.EndTime = time.Now()
		tas.stateVersion++
	}
}

// MarkRunning resets EndTime and bumps stateVersion to transition a finished
// entity back to running (e.g. after retry). No-op if already running.
func (tas *TimeAndState) MarkRunning() {
	if tas.IsFinished() {
		tas.EndTime = time.Time{}
		tas.stateVersion++
	}
}

// SetDuration replaces the cached duration without bumping stateVersion.
func (tas *TimeAndState) SetDuration(d time.Duration) {
	tas.DurationCache = d
}

// StateVersion returns a counter that increments on state transitions.
func (tas *TimeAndState) StateVersion() uint64 {
	return tas.stateVersion
}

// BumpVersion increments stateVersion unconditionally.
func (tas *TimeAndState) BumpVersion() {
	tas.stateVersion++
}

// SyncFrom copies duration and error from other, and applies any state
// transitions (started/finished/running) into tas, bumping stateVersion on change.
func (tas *TimeAndState) SyncFrom(other *TimeAndState) {
	tas.DurationCache = other.DurationCache
	tas.EndError = other.EndError

	if other.HasStarted() && !tas.HasStarted() {
		tas.SetStarted(time.Now())
	}

	// Backward transition: other is running but tas is finished (e.g. after retry).
	if other.HasStarted() && !other.IsFinished() && tas.IsFinished() {
		tas.MarkRunning()
	}

	if other.IsFinished() && !tas.IsFinished() {
		tas.MarkFinished()
	}
}
