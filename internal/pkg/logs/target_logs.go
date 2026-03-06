package logs

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/pkg/errors"
)

type TargetLogs struct {
	xpath config_attributes.Xpath
	*logs_phase.PhaseLogs
	parent   *TargetLogs
	children []*TargetLogs
	cache    DurationAndError
}

type DurationAndError struct {
	Duration time.Duration
	Err      error
}

func NewTargetLogs(xpath config_attributes.Xpath, flags config_flags.Logging) (*TargetLogs, error) {
	phaseLogs, err := logs_phase.NewPhaseLogs(xpath, flags)
	if err != nil {
		return nil, err
	}

	return &TargetLogs{
		xpath:     xpath,
		PhaseLogs: phaseLogs,
		parent:    nil,
		children:  nil,
	}, nil
}

func (ts *TargetLogs) AddParent(parent *TargetLogs) error {
	if ts.parent != nil {
		return errors.New("can't add parent to targetLogs that already have it")
	}

	ts.parent = parent
	parent.children = append(parent.children, ts)

	return nil
}

func (ts *TargetLogs) GetCachedDurationAndError() DurationAndError {
	return ts.cache
}

func (ts *TargetLogs) calculateDurationAndError() DurationAndError {
	if len(ts.children) != 0 {
		ts.cache = ts.calculateFromChildren()
	} else {
		ts.cache = ts.calculateFromPhases()
	}
	return ts.cache
}

func (ts *TargetLogs) calculateFromChildren() DurationAndError {
	var dae DurationAndError

	for _, child := range ts.children {
		childDae := child.calculateDurationAndError()
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
			dae.Err = err
			break
		}

		dae.Duration += duration

		err = tas.GetEndError()
		if err != nil {
			dae.Err = err
			break
		}
	}

	return dae
}

func (ts *TargetLogs) GetCurrentTargetLog() *logs_phase.PhaseLog {
	for _, phaseLogPair := range ts.PhaseLogs.All() {
		phaseLog := phaseLogPair.Value
		err := phaseLog.TimeAndState().GetEndError()

		if err != nil {
			return phaseLog
		}
	}

	return ts.PhaseLogs.Last()
}

// Clear deletes/resets phases logs and timer.
func (ts *TargetLogs) Clear() {
	ts.PhaseLogs.Clear()
}
