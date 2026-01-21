package logs

import (
	"errors"

	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
)

type TargetLogs struct {
	xpath config_attributes.Xpath
	*logs_phase.PhaseLogs
	timeAndState *time_and_state.TimeAndState // This never finishes, we take end times from children
	parent       *TargetLogs
	children     []*TargetLogs
}

func NewTargetLogs(xpath config_attributes.Xpath, flags config_flags.Logging) *TargetLogs {
	targetLogs := &TargetLogs{
		xpath,
		nil,
		nil,
		nil,
		[]*TargetLogs{},
	}

	targetLogs.timeAndState = time_and_state.NewTimeAndState(targetLogs)
	targetLogs.PhaseLogs = logs_phase.NewPhaseLogs(xpath, flags, targetLogs)

	return targetLogs
}

func (ts *TargetLogs) AddParent(parent *TargetLogs) error {
	if ts.parent != nil {
		return errors.New("can't add parent to targetLogs that already have it")
	}

	ts.parent = parent
	parent.children = append(parent.children, ts)

	return nil
}

func (ts *TargetLogs) Parent(parent *TargetLogs) *TargetLogs {
	return ts.parent
}

func (ts *TargetLogs) Children(parent *TargetLogs) []*TargetLogs {
	return ts.children
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

func (ts *TargetLogs) GetNestedTimeAndState() *time_and_state.TimeAndState {
	var lastTas *time_and_state.TimeAndState

	for _, child := range ts.children {
		var tas *time_and_state.TimeAndState

		// If child errored, the child children definitely did not progress more,
		// so we don't need to go deeper
		childTas := child.GetCurrentTargetLog().TimeAndState()
		if childTas.GetEndError() != nil {
			tas = childTas
		} else {
			tas = child.GetNestedTimeAndState()
		}

		if lastTas == nil {
			lastTas = tas
			continue
		}

		// tas > lastTas
		if tas.GetEndTime().After(lastTas.GetEndTime()) {
			lastTas = tas
		}
	}

	return ts.timeAndState.WithEnd(lastTas)
}

// Deletes/resets phases logs and timer
func (ts *TargetLogs) Clear() {
	ts.PhaseLogs.Clear()
	ts.timeAndState.Clear()
}
