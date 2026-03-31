package logs

import (
	"fmt"
	"strings"

	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type TargetsLogs struct {
	logs  *omap.Omap[attributes.Xpath, *TargetLogs]
	flags flags.Logging
}

func NewTargetsLogs(flags flags.Logging) (*TargetsLogs, error) {
	logs, err := omap.New[attributes.Xpath, *TargetLogs]()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create targets logs map")
	}

	return &TargetsLogs{
		logs:  logs,
		flags: flags,
	}, nil
}

func (ts *TargetsLogs) AddWithParent(xpath attributes.Xpath, parent *TargetLogs) (*TargetLogs, error) {
	targetLogs, err := ts.Add(xpath)
	if err != nil {
		return nil, err
	}

	return targetLogs, targetLogs.AddParent(parent)
}

func (ts *TargetsLogs) Add(xpath attributes.Xpath) (*TargetLogs, error) {
	targetLogs, err := NewTargetLogs(xpath, ts.flags)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create target logs for %s", xpath.String())
	}

	err = ts.logs.Set(xpath, targetLogs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set target logs for %s", xpath.String())
	}

	return targetLogs, nil
}

func (ts *TargetsLogs) get(xpath attributes.Xpath) (*TargetLogs, bool) {
	return ts.logs.Get(xpath)
}

func (ts *TargetsLogs) MustGet(xpath attributes.Xpath) *TargetLogs {
	targetLogs, ok := ts.get(xpath)
	if !ok {
		panic(fmt.Sprintf("internal error: xpath %q not present in TargetsLogs", xpath.String()))
	}

	return targetLogs
}

func (ts *TargetsLogs) CalculateDurationAndError() {
	for _, pair := range ts.logs.Pairs() {
		if pair.Value.parent == nil { // Calculate only root nodes - flakes
			pair.Value.calculateDurationAndError()
		}
	}
}

func (ts *TargetsLogs) MustGetOrCreateLog(xpath attributes.Xpath, phase phases.Phase) *phase.PhaseLog {
	targetLogs, ok := ts.get(xpath)
	if !ok {
		panic(fmt.Sprintf("internal error: xpath %q not present in TargetsLogs for phase %s", xpath.String(), phase))
	}

	return ts.getOrCreateLog(targetLogs, phase, nil)
}

func (ts *TargetsLogs) getOrCreateLog(targetLogs *TargetLogs, phase phases.Phase, log *phase.PhaseLog) *phase.PhaseLog {
	if log == nil || len(targetLogs.children) == 0 {
		log = targetLogs.PhaseLogs.SetIfNotExists(phase, log)
	}

	for _, child := range targetLogs.children {
		ts.getOrCreateLog(child, phase, log)
	}

	return log
}

func (ts *TargetsLogs) MustGetLogs(xpath attributes.Xpath) *phase.PhaseLogs {
	targetLogs, ok := ts.get(xpath)
	if !ok {
		panic(fmt.Sprintf("internal error: xpath %q not present in TargetsLogs", xpath.String()))
	}

	return targetLogs.PhaseLogs
}

func (ts *TargetsLogs) MustGetFirstLogErrorOrLastLog(xpath attributes.Xpath) *phase.PhaseLog {
	targetLogs, ok := ts.get(xpath)
	if !ok {
		panic(fmt.Sprintf("internal error: xpath %q not present in TargetsLogs", xpath.String()))
	}

	return targetLogs.GetCurrentTargetLog()
}

// Clear empties target logs but does not delete them.
func (ts *TargetsLogs) Clear() {
	for _, pair := range ts.logs.Pairs() {
		pair.Value.Clear()
	}
}

func (ts *TargetsLogs) ComputeStatisticsPerPhase() *stats.StatisticsPerPhase {
	statistics := stats.NewStatisticsPerPhase()

	for _, pair := range ts.logs.Pairs() {
		targetLogs := pair.Value

		if len(targetLogs.children) != 0 {
			continue
		}

		log := targetLogs.GetCurrentTargetLog()
		if log == nil {
			continue
		}

		timeAndState := log.TimeAndState()

		if !timeAndState.IsFinished() {
			statistics.Add(log.Phase(), stats.Running, pair.Key)

			continue
		}

		if timeAndState.GetEndError() != nil {
			statistics.Add(log.Phase(), stats.Failed, pair.Key)

			continue
		}

		statistics.Add(log.Phase(), stats.Done, pair.Key)
	}

	return statistics
}

func (ts *TargetsLogs) Debug() string {
	str := fmt.Sprintf("\nLogs: %d\n", ts.logs.Len())

	str += fmt.Sprintf("\n  Stats: %v\n\n", ts.ComputeStatisticsPerPhase())

	for _, pair := range ts.logs.Pairs() {
		var parent attributes.Xpath
		if pair.Value.parent != nil {
			parent = pair.Value.parent.xpath
		}

		var builder strings.Builder
		for _, child := range pair.Value.children {
			builder.WriteString(child.xpath.String())
			builder.WriteByte(',')
		}

		children := builder.String()

		str += fmt.Sprintf("  '%s' parent:%s children:%v, len:%d\n", pair.Key, parent, children, pair.Value.PhaseLogs.Len())

		for _, logPair := range pair.Value.PhaseLogs.All() {
			finished := logPair.Value.TimeAndState().IsFinished()
			err := logPair.Value.TimeAndState().GetEndError()
			cmdLen := len(logPair.Value.CommandLogs())
			str += fmt.Sprintf("    %s finished:%v err:%v len:%d\n", logPair.Key, finished, err, cmdLen)

			for _, log := range logPair.Value.CommandLogs() {
				str += fmt.Sprintf("      '%s' finished:%v, err:%v\n", log.Description, log.TimeAndState.IsFinished(), log.TimeAndState.GetEndError())
			}
		}
	}

	return str
}
