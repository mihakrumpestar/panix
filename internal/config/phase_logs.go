package config

import (
	"strings"

	"github.com/hayageek/threadsafe"
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// PhaseLogs

type PhaseLogs struct {
	logs *omap.Omap[phases.Phase, *PhaseLog]
}

func NewPhaseLogs() *PhaseLogs {
	phaseLogs, err := omap.New[phases.Phase, *PhaseLog]()
	if err != nil {
		panic(err)
	}

	return &PhaseLogs{
		logs: phaseLogs,
	}
}

func (l *PhaseLogs) SafeGet(phase phases.Phase) *PhaseLog {
	phaselog, ok := l.logs.Get(phase)
	if !ok {
		phaselog = NewPhaseLog()
		l.logs.Set(phase, phaselog)
	}

	return phaselog
}

func (l *PhaseLogs) All() []omap.Pair[phases.Phase, *PhaseLog] {
	return l.logs.Pairs() // Any other ranging method locks the dict until it traverses it
}

func (l *PhaseLogs) Len() int {
	return l.logs.Len()
}

// PhaseLog

type PhaseLog struct {
	commandLogs  *threadsafe.Slice[*CommandLog]
	TimeAndState *TimeAndState
}

func NewPhaseLog() *PhaseLog {
	return &PhaseLog{
		commandLogs:  threadsafe.NewSlice[*CommandLog](),
		TimeAndState: NewTimeAndState(),
	}
}

func (pLog *PhaseLog) LastCommand() *CommandLog {
	commandLog, ok := pLog.commandLogs.Get(pLog.commandLogs.Length() - 1)
	if !ok {
		panic("commandLogs does not have element on specified index")
	}
	return commandLog
}

func (pLog *PhaseLog) NewCommand() *CommandLog {
	commandLog := NewCommandLog()
	pLog.commandLogs.Append(commandLog)

	return commandLog
}

func (pLog *PhaseLog) AddMessageOnly(msg ...string) *CommandLog {
	commandLog := pLog.NewCommand()

	commandLog.Command.Store("-> " + strings.Join(msg, ""))

	commandLog.TimeAndState.StartTimer()
	commandLog.TimeAndState.EndTimer()

	return commandLog
}

func (pLog *PhaseLog) CommandLogs() []*CommandLog {
	return pLog.commandLogs.Values()
}
