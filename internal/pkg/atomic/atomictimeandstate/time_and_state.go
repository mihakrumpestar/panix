package atomictimeandstate

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
	"github.com/pkg/errors"
)

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
