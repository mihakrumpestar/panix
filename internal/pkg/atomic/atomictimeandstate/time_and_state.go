package atomictimeandstate

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
	"github.com/pkg/errors"
)

type TimeAndState struct {
	StartTime     time.Time            `json:"start_time"`
	EndTime       time.Time            `json:"end_time"`
	DurationCache time.Duration        `json:"duration"`
	EndError      *errorjson.ErrorJSON `json:"end_error,omitempty"`

	live bool
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
