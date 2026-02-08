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
		xpath:     xpath,
		PhaseLogs: logs_phase.NewPhaseLogs(xpath, flags),
		parent:    nil,
		children:  nil,
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
		Duration: 0,
		Err:      nil,
	}

	if len(ts.children) != 0 {
		dae = ts.calculateFromChildren()
	} else {
		dae = ts.calculateFromPhases()
	}

	return dae
}

func (ts *TargetLogs) calculateFromChildren() DurationAndError {
	var dae DurationAndError

	for _, child := range ts.children {
		childDae := child.CalculateDurationAndError()
		if childDae.Duration > dae.Duration {
			dae = childDae
		}
	}

	return dae
}

func (ts *TargetLogs) calculateFromPhases() DurationAndError {
	var dae DurationAndError

	for _, phaseLog := range ts.PhaseLogs.All() {
		tas := phaseLog.Value.TimeAndState()

		duration, err := tas.DurationOrElapsedTime()
		if err != nil {
			break
		}

		dae.Duration += duration

		if endErr := tas.GetEndError(); endErr != nil {
			dae.Err = endErr
			break
		}
	}

	return dae
}

func (ts *TargetLogs) GetCurrentTargetLog() *logs_phase.PhaseLog {
	for _, phaseLogPair := range ts.PhaseLogs.All() {
		phaseLog := phaseLogPair.Value
		if err := phaseLog.TimeAndState().GetEndError(); err != nil {
			return phaseLog
		}
	}

	return ts.PhaseLogs.Last()
}

// Clear deletes/resets phases logs and timer.
func (ts *TargetLogs) Clear() {
	ts.PhaseLogs.Clear()
}
