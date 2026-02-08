package logs_phase

import (
	"fmt"

	"github.com/hayageek/threadsafe"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type PhaseLog struct {
	commandLogs  *threadsafe.Slice[*logs_command.CommandLog]
	timeAndState *time_and_state.TimeAndState
	creatorXpath config_attributes.Xpath
	phase        phases.Phase
	flags        config_flags.Logging
}

func NewPhaseLog(creatorXpath config_attributes.Xpath, phase phases.Phase, flags config_flags.Logging) *PhaseLog {
	return &PhaseLog{
		commandLogs:  threadsafe.NewSlice[*logs_command.CommandLog](),
		timeAndState: time_and_state.NewTimeAndState(),
		creatorXpath: creatorXpath,
		phase:        phase,
		flags:        flags,
	}
}

func (pLog *PhaseLog) Phase() phases.Phase {
	return pLog.phase
}

func (pLog *PhaseLog) LastNonMsgOnlyCommand() *logs_command.CommandLog {
	length := pLog.commandLogs.Length()
	if length == 0 {
		return nil
	}

	for i := length - 1; i >= 0; i-- {
		commandLog, ok := pLog.commandLogs.Get(i)
		if !ok {
			panic("commandLogs does not have element on specified index")
		}

		if !commandLog.MsgOnly {
			return commandLog
		}
	}

	return nil
}

func (pLog *PhaseLog) NewCommand(description, statusIfFailed string, msgOnly bool) *logs_command.CommandLog {
	commandLog := logs_command.NewCommandLog(description, statusIfFailed, msgOnly)
	pLog.commandLogs.Append(commandLog)

	return commandLog
}

func (pLog *PhaseLog) Verbose(format string, a ...any) *logs_command.CommandLog {
	if !pLog.flags.Verbose {
		return nil
	}

	msg := fmt.Sprintf("VERBOSE "+format, a...)

	return pLog.NewCommand(msg, "", true)
}

func (pLog *PhaseLog) Debug(format string, a ...any) *logs_command.CommandLog {
	if !pLog.flags.Debug {
		return nil
	}

	msg := fmt.Sprintf("DEBUG "+format, a...)

	return pLog.NewCommand(msg, "", true)
}

func (pLog *PhaseLog) CommandLogs() []*logs_command.CommandLog {
	return pLog.commandLogs.Values()
}

func (pLog *PhaseLog) Clear() {
	pLog.commandLogs.Clear()
	pLog.timeAndState.Clear()
}

func (pLog *PhaseLog) TimeAndState() *time_and_state.TimeAndState {
	return pLog.timeAndState
}
