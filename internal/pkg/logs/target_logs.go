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

func (ts *TargetLogs) GetTimeAndState() *time_and_state.TimeAndState {
	return ts.timeAndState
}

func (ts *TargetLogs) GetParent() time_and_state.TimeAndStateNode {
	if ts.parent == nil {
		return nil
	}
	return ts.parent
}

func (ts *TargetLogs) GetChildren() []time_and_state.TimeAndStateNode {
	children := make([]time_and_state.TimeAndStateNode, len(ts.children))
	for i, child := range ts.children {
		children[i] = child
	}
	return children
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
	ts.timeAndState.Clear()
}
