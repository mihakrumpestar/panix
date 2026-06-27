package phaselogs

import (
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicslice"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

type PhaseLog struct {
	CommandLogs  *atomicslice.AtomicSlice[*command.CommandLog] `yaml:"-" json:"command_logs"`
	TimeAndState *atomictimeandstate.AtomicTimeAndState        `yaml:"-" json:"time_and_state"`
}

func NewPhaseLog() *PhaseLog {
	return &PhaseLog{
		CommandLogs:  atomicslice.New[*command.CommandLog](),
		TimeAndState: atomictimeandstate.New(),
	}
}

func (pLog *PhaseLog) NewCommand(
	phaseXpath xpath.Xpath,
	description, statusIfRunning, statusIfFailed string,
	commandToRun, env []string,
	maxOutputLines uint64,
) *command.CommandLog {
	commandLog := command.NewCommandLog(phaseXpath, description, statusIfRunning, statusIfFailed, commandToRun, env)
	commandLog.Output.SetMaxLines(maxOutputLines)
	pLog.CommandLogs.Append(commandLog)

	return commandLog
}

func (pLog *PhaseLog) Clear() {
	if pLog == nil {
		return
	}

	pLog.CommandLogs.Clear()
	pLog.TimeAndState.Clear()
}
