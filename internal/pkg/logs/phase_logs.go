package logs

import (
	"fmt"
	"strings"

	"github.com/hayageek/threadsafe"
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// PhaseLogs

type PhaseLogs struct {
	logs *omap.Omap[phases.Phase, *PhaseLog]
	// Internal
	flags *config_flags.Flags
}

func NewPhaseLogs(flags *config_flags.Flags) *PhaseLogs {
	phaseLogs, err := omap.New[phases.Phase, *PhaseLog]()
	if err != nil {
		panic(err)
	}

	return &PhaseLogs{
		logs:  phaseLogs,
		flags: flags,
	}
}

func (pl *PhaseLogs) SafeGet(phase phases.Phase) *PhaseLog {
	phaselog, ok := pl.logs.Get(phase)
	if !ok {
		phaselog = NewPhaseLog(phase, pl.flags)
		pl.logs.Set(phase, phaselog)
	}

	return phaselog
}

func (pl *PhaseLogs) All() []omap.Pair[phases.Phase, *PhaseLog] {
	return pl.logs.Pairs() // Any other ranging method locks the dict until it traverses it
}

func (pl *PhaseLogs) Len() int {
	return pl.logs.Len()
}

func (pl *PhaseLogs) Del(phase phases.Phase) {
	_, ok := pl.logs.Del(phase)
	if !ok {
		panic("key for del does not exist")
	}
}

// PhaseLog

type PhaseLog struct {
	commandLogs  *threadsafe.Slice[*CommandLog]
	TimeAndState *time_and_state.TimeAndState
	// Internal
	phase phases.Phase
	flags *config_flags.Flags
}

func NewPhaseLog(phase phases.Phase, flags *config_flags.Flags) *PhaseLog {
	return &PhaseLog{
		commandLogs:  threadsafe.NewSlice[*CommandLog](),
		TimeAndState: time_and_state.NewTimeAndState(),
		phase:        phase,
		flags:        flags,
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

	commandLog.Command.Store(strings.Join(msg, ""))

	commandLog.TimeAndState.StartTimer()
	commandLog.TimeAndState.EndTimer()

	return commandLog
}

func (pLog *PhaseLog) Verbose(format string, a ...any) *CommandLog {
	if !pLog.flags.Verbose {
		return nil
	}

	commandLog := pLog.NewCommand()
	commandLog.Command.Store(fmt.Sprintf("VERBOSE "+format, a...))

	commandLog.TimeAndState.StartTimer()
	commandLog.TimeAndState.EndTimer()

	return commandLog
}

func (pLog *PhaseLog) Debug(msg ...string) *CommandLog {
	if !pLog.flags.Debug {
		return nil
	}

	commandLog := pLog.NewCommand()
	commandLog.Command.Store("DEBUG " + strings.Join(msg, ""))

	commandLog.TimeAndState.StartTimer()
	commandLog.TimeAndState.EndTimer()

	return commandLog
}

func (pLog *PhaseLog) CommandLogs() []*CommandLog {
	return pLog.commandLogs.Values()
}

func (pLog *PhaseLog) Clear() {
	pLog.commandLogs.Clear()
	pLog.TimeAndState = time_and_state.NewTimeAndState()
}

func (pLog *PhaseLog) Phase() phases.Phase {
	return pLog.phase
}
