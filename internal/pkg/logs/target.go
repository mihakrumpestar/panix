package logs

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type TargetLogs struct {
	*phase.PhaseLogs

	xpath    attributes.Xpath
	parent   *TargetLogs
	children []*TargetLogs
	cache    DurationAndError
}

type DurationAndError struct {
	Duration time.Duration
	Err      error
}

func NewTargetLogs(xpath attributes.Xpath, flags flags.Logging) (*TargetLogs, error) {
	phaseLogs, err := phase.NewPhaseLogs(xpath, flags)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create phase logs")
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

type MachineState struct {
	Status   stats.StatsState
	Phase    phases.Phase
	Duration time.Duration
	Error    error
}

func (ts *TargetLogs) ComputeMachineState(orderedPhases []phases.Phase) MachineState {
	if len(ts.children) != 0 {
		return MachineState{}
	}

	machineState := MachineState{}

	for _, phase := range orderedPhases {
		pl := ts.PhaseLogs.Get(phase)
		if pl == nil {
			continue
		}

		tas := pl.TimeAndState()
		machineState.Phase = phase

		if !tas.IsFinished() {
			machineState.Status = stats.Running
			machineState.Duration, _ = tas.DurationOrElapsedTime()

			return machineState
		}

		duration, _ := tas.DurationOrElapsedTime()
		machineState.Duration += duration

		endErr := tas.GetEndError()
		if endErr != nil {
			machineState.Status = stats.Failed
			machineState.Error = endErr

			return machineState
		}
	}

	machineState.Status = stats.Done

	return machineState
}

func (ts *TargetLogs) GetCachedDurationAndError() DurationAndError {
	return ts.cache
}

func (ts *TargetLogs) GetCurrentTargetLog() *phase.PhaseLog {
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
