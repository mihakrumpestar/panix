package phaselogs

import (
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicslice"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
)

type PhaseLog struct {
	CommandLogs  *atomicslice.AtomicSlice[*command.CommandLog]
	TimeAndState *atomictimeandstate.AtomicTimeAndState `json:"time_and_state"`
}

func NewPhaseLog() *PhaseLog {
	return &PhaseLog{
		CommandLogs:  atomicslice.New[*command.CommandLog](),
		TimeAndState: atomictimeandstate.New(),
	}
}

func (pLog *PhaseLog) NewCommand(description, statusIfRunning, statusIfFailed string, commandToRun, env []string) *command.CommandLog {
	commandLog := command.NewCommandLog(description, statusIfRunning, statusIfFailed, commandToRun, env)
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
