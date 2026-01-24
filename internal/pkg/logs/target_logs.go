package logs

import (
	"errors"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
)

type TargetLogs struct {
	xpath config_attributes.Xpath
	*logs_phase.PhaseLogs
	parent   *TargetLogs
	children []*TargetLogs
}

type DurationAndError struct {
	Duration time.Duration
	Err      error
}

func NewTargetLogs(xpath config_attributes.Xpath, flags config_flags.Logging) *TargetLogs {
	return &TargetLogs{
		xpath,
		logs_phase.NewPhaseLogs(xpath, flags),
		nil,
		nil,
	}
}

func (ts *TargetLogs) AddParent(parent *TargetLogs) error {
	if ts.parent != nil {
		return errors.New("can't add parent to targetLogs that already have it")
	}

	ts.parent = parent
	parent.children = append(parent.children, ts)

	return nil
}

func (ts *TargetLogs) CalculateDurationAndError() DurationAndError {
	dae := DurationAndError{
		time.Duration(0),
		nil,
	}

	// If there are children, we take the one with the longest duration,
	// else we sum all phase durations
	if len(ts.children) != 0 {
		for _, child := range ts.children {
			childDae := child.CalculateDurationAndError()
			if childDae.Duration > dae.Duration {
				dae = childDae
			}
		}
	} else {
		for _, phaseLog := range ts.PhaseLogs.All() {
			tas := phaseLog.Value.TimeAndState()

			duration, err := tas.DurationOrElapsedTime()
			if err != nil {
				break
			}

			dae.Duration += duration

			endErr := tas.GetEndError()
			if endErr != nil {
				dae.Err = endErr
				break
			}
		}
	}

	return dae
}

func (ts *TargetLogs) GetCurrentTargetLog() *logs_phase.PhaseLog {
	// Return first log that has error
	for _, phaseLogPair := range ts.PhaseLogs.All() {
		phaseLog := phaseLogPair.Value
		err := phaseLog.TimeAndState().GetEndError()
		if err != nil {
			return phaseLog
		}
	}

	// Or last log
	return ts.PhaseLogs.Last()
}

// Deletes/resets phases logs and timer
func (ts *TargetLogs) Clear() {
	ts.PhaseLogs.Clear()
}
