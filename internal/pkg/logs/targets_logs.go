package logs

import (
	"fmt"

	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_stats"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type TargetsLogs struct {
	logs  *omap.Omap[config_attributes.Xpath, *TargetLogs]
	flags config_flags.Logging
}

func NewTargetsLogs(flags config_flags.Logging) (*TargetsLogs, error) {
	logs, err := omap.New[config_attributes.Xpath, *TargetLogs]()
	if err != nil {
		return nil, err
	}

	return &TargetsLogs{
		logs:  logs,
		flags: flags,
	}, nil
}

func (ts *TargetsLogs) Add(xpath config_attributes.Xpath) (*TargetLogs, error) {
	targetLogs := NewTargetLogs(xpath, ts.flags)

	err := ts.logs.Set(xpath, targetLogs)
	if err != nil {
		return nil, err
	}

	return targetLogs, nil
}

func (ts *TargetsLogs) Get(xpath config_attributes.Xpath) *TargetLogs {
	targetLogs, ok := ts.logs.Get(xpath)
	if !ok {
		panic("xpath key not present in TargetsLogs, this should never happen")
	}

	return targetLogs
}

func (ts *TargetsLogs) GetOrCreateLog(xpath config_attributes.Xpath, phase phases.Phase) *logs_phase.PhaseLog {
	targetLogs, ok := ts.logs.Get(xpath)
	if !ok {
		panic("xpath key not present in TargetsLogs, this should never happen")
	}

	return ts.getOrCreateLog(targetLogs, phase, nil)
}

func (ts *TargetsLogs) getOrCreateLog(targetLogs *TargetLogs, phase phases.Phase, log *logs_phase.PhaseLog) *logs_phase.PhaseLog {
	// Create it on nil log or if last child
	if log == nil || len(targetLogs.children) == 0 {
		log = targetLogs.PhaseLogs.SetIfNotExists(phase, log)
	}

	// Children should have same log pointer in phase
	for _, child := range targetLogs.children {
		ts.getOrCreateLog(child, phase, log)
	}

	return log
}

func (ts *TargetsLogs) GetLogs(xpath config_attributes.Xpath) *logs_phase.PhaseLogs {
	logs, ok := ts.logs.Get(xpath)
	if !ok {
		panic("xpath key not present in TargetsLogs, this should never happen")
	}

	return logs.PhaseLogs
}

func (ts *TargetsLogs) GetFirstLogErrorOrLastLog(xpath config_attributes.Xpath) *logs_phase.PhaseLog {
	logs, ok := ts.logs.Get(xpath)
	if !ok {
		panic("xpath key not present in TargetsLogs, this should never happen")
	}

	return logs.GetCurrentTargetLog()
}

// Emptys target logs but does not delete them
func (ts *TargetsLogs) Clear() {
	for _, pair := range ts.logs.Pairs() {
		pair.Value.Clear()
	}
}

func (ts *TargetsLogs) ComputeStatisticsPerPhase() *logs_stats.StatisticsPerPhase {
	stats := logs_stats.NewStatisticsPerPhase()

	for _, pair := range ts.logs.Pairs() {
		targetLogs := pair.Value

		if len(targetLogs.children) != 0 {
			continue
		}

		log := targetLogs.GetCurrentTargetLog()
		if log == nil {
			continue
		}

		lastCommand := log.LastNonMsgOnlyCommand()
		if lastCommand == nil {
			continue
		}

		if !lastCommand.TimeAndState.IsFinished() {
			stats.Add(log.Phase(), logs_stats.Running, pair.Key)
			continue
		}

		if lastCommand.TimeAndState.GetEndError() != nil {
			stats.Add(log.Phase(), logs_stats.Failed, pair.Key)
			continue
		}

		stats.Add(log.Phase(), logs_stats.Done, pair.Key)
	}

	return stats
}

func (ts *TargetsLogs) Debug() string {
	str := fmt.Sprintf("\nLogs: %d\n", ts.logs.Len())

	str += fmt.Sprintf("\n  Stats: %v\n\n", ts.ComputeStatisticsPerPhase())

	for _, pair := range ts.logs.Pairs() {
		var parent config_attributes.Xpath
		if pair.Value.parent != nil {
			parent = pair.Value.parent.xpath
		}

		str += fmt.Sprintf("  '%s' parent:%s children:%d, len:%d\n", pair.Key, parent, len(pair.Value.children), pair.Value.PhaseLogs.Len())

		for _, logPair := range pair.Value.PhaseLogs.All() {
			str += fmt.Sprintf("    %s len:%d\n", logPair.Key, len(logPair.Value.CommandLogs()))

			for _, log := range logPair.Value.CommandLogs() {
				str += fmt.Sprintf("      '%s' msgOnly:%v finished:%v, err:%v\n", log.Description, log.MsgOnly, log.TimeAndState.IsFinished(), log.TimeAndState.GetEndError())
			}
		}
	}

	return str
}
