package phase

import (
	"github.com/hayageek/threadsafe"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/timeandstate"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type PhaseLog struct {
	commandLogs  *threadsafe.Slice[*command.CommandLog]
	timeAndState *timeandstate.TimeAndState
	creatorXpath attributes.Xpath
	phase        phases.Phase
	flags        flags.Logging
}

func NewPhaseLog(creatorXpath attributes.Xpath, phase phases.Phase, flags flags.Logging) *PhaseLog {
	return &PhaseLog{
		commandLogs:  threadsafe.NewSlice[*command.CommandLog](),
		timeAndState: timeandstate.NewTimeAndState(),
		creatorXpath: creatorXpath,
		phase:        phase,
		flags:        flags,
	}
}

func (pLog *PhaseLog) Phase() phases.Phase {
	return pLog.phase
}

func (pLog *PhaseLog) Last() *command.CommandLog {
	length := pLog.commandLogs.Length()
	if length == 0 {
		return nil
	}

	commandLog, ok := pLog.commandLogs.Get(length - 1)
	if !ok {
		panic("internal error: commandLogs does not have element on specified index")
	}

	return commandLog
}

func (pLog *PhaseLog) NewCommand(description, statusIfRunning, statusIfFailed string, commandToRun, env []string) *command.CommandLog {
	commandLog := command.NewCommandLog(description, statusIfRunning, statusIfFailed, commandToRun, env)
	pLog.commandLogs.Append(commandLog)

	return commandLog
}

func (pLog *PhaseLog) CommandLogs() []*command.CommandLog {
	return pLog.commandLogs.Values()
}

func (pLog *PhaseLog) Clear() {
	if pLog == nil {
		return
	}

	pLog.commandLogs.Clear()
	pLog.timeAndState.Clear()
}

func (pLog *PhaseLog) TimeAndState() *timeandstate.TimeAndState {
	return pLog.timeAndState
}
