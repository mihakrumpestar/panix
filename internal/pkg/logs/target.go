package logs

import (
	"fmt"

	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type TargetLogs struct {
	*PhaseLogs
	primary bool // Indicates that this entry is not a relation
}

func (ts *TargetLogs) GetFirstLogErrorOrLastLog() *PhaseLog {
	// Return first log that has error
	for _, phaseLogPair := range ts.PhaseLogs.All() {
		phaseLog := phaseLogPair.Value
		tas := phaseLog.TimeAndState().GetTimeAndState()
		if tas.Error != nil {
			return phaseLog
		}
	}

	return ts.PhaseLogs.Last()
}

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

func (ts *TargetsLogs) GetLog(xpath config_attributes.Xpath, phase phases.Phase) *PhaseLog {
	logs, ok := ts.logs.Get(xpath)
	if !ok {
		return nil
	}

	return logs.Get(phase)
}

func (ts *TargetsLogs) GetOrCreateLog(attr config_attributes.Attributes, phase phases.Phase) *PhaseLog {
	return ts.getOrCreateLog(attr, phase, nil)
}

func (ts *TargetsLogs) getOrCreateLog(attr config_attributes.Attributes, phase phases.Phase, log *PhaseLog) *PhaseLog {
	logs, ok := ts.logs.Get(attr.Xpath)
	if !ok {
		logs = &TargetLogs{
			NewPhaseLogs(ts.flags),
			len(attr.Related) == 0,
		}
		ts.logs.Set(attr.Xpath, logs)
	}

	log = logs.SetIfNotExists(phase, log)

	// Releted Xpaths should have same log pointer in phase
	for _, relation := range attr.Related {
		ts.getOrCreateLog(relation, phase, log)
	}

	return log
}

func (ts *TargetsLogs) GetLogs(xpath config_attributes.Xpath) *PhaseLogs {
	logs, ok := ts.logs.Get(xpath)
	if !ok {
		return nil
	}

	return logs.PhaseLogs
}

func (ts *TargetsLogs) GetFirstLogErrorOrLastLog(xpath config_attributes.Xpath) *PhaseLog {
	logs, ok := ts.logs.Get(xpath)
	if !ok {
		return nil
	}

	return logs.GetFirstLogErrorOrLastLog()
}

func (ts *TargetsLogs) ComputeStatisticsPerPhase() *StatisticsPerPhase {
	stats := NewStatisticsPerPhase()

	for _, pair := range ts.logs.Pairs() {
		targetLogs := pair.Value

		if !targetLogs.primary {
			continue
		}

		log := targetLogs.GetFirstLogErrorOrLastLog()
		if log == nil {
			continue
		}

		lastCommand := log.LastNonMsgOnlyCommand()
		if lastCommand == nil {
			continue
		}

		timeAndState := lastCommand.TimeAndState.GetTimeAndState()
		if !timeAndState.Finished {
			stats.Add(log.phase, Running, pair.Key)
			continue
		}

		if timeAndState.Error != nil {
			stats.Add(log.phase, Failed, pair.Key)
			continue
		}

		stats.Add(log.phase, Done, pair.Key)
	}

	return stats
}

func (ts *TargetsLogs) Debug() string {
	str := fmt.Sprintf("\nLogs: %d\n", ts.logs.Len())

	str += fmt.Sprintf("\n  Stats: %v\n\n", ts.ComputeStatisticsPerPhase())

	for _, pair := range ts.logs.Pairs() {
		str += fmt.Sprintf("  '%s' primary:%v, len:%d\n", pair.Key, pair.Value.primary, pair.Value.Len())

		for _, logPair := range pair.Value.All() {
			str += fmt.Sprintf("    %s len:%d\n", logPair.Key, logPair.Value.commandLogs.Length())

			for _, log := range logPair.Value.commandLogs.Values() {
				tas := log.TimeAndState.GetTimeAndState()

				str += fmt.Sprintf("      '%s' msgOnly:%v finished:%v, err:%v\n", log.Description, log.MsgOnly, tas.Finished, tas.Error)
			}
		}
	}

	return str
}
