package logs

import (
	"fmt"
	"sync"

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
	flags config_flags.Logging
}

func NewPhaseLogs(flags config_flags.Logging) *PhaseLogs {
	phaseLogs, err := omap.New[phases.Phase, *PhaseLog]()
	if err != nil {
		panic(err)
	}

	return &PhaseLogs{
		logs:  phaseLogs,
		flags: flags,
	}
}

func (pl *PhaseLogs) Get(phase phases.Phase) *PhaseLog {
	phaselog, ok := pl.logs.Get(phase)
	if !ok {
		return nil
	}

	return phaselog
}

func (pl *PhaseLogs) GetOrCreate(phase phases.Phase) *PhaseLog {
	phaselog := pl.Get(phase)
	if phaselog == nil {
		phaselog = NewPhaseLog(phase, pl.flags)
		pl.logs.Set(phase, phaselog)
	}

	return phaselog
}

func (pl *PhaseLogs) SetIfNotExists(phase phases.Phase, phaseLog *PhaseLog) *PhaseLog {
	pLog, ok := pl.logs.Get(phase)
	if ok {
		return pLog
	}

	if phaseLog == nil {
		phaseLog = NewPhaseLog(phase, pl.flags)
	}

	pl.logs.Set(phase, phaseLog)

	return phaseLog
}

func (pl *PhaseLogs) All() []omap.Pair[phases.Phase, *PhaseLog] {
	return pl.logs.Pairs() // Any other ranging method locks the dict until it traverses it
}

// Gets the last inserted PhaseLog from PhaseLogs
func (pl *PhaseLogs) Last() *PhaseLog {
	if pl.logs.Len() == 0 {
		return nil
	}

	lastIndex := pl.logs.Len() - 1

	return pl.logs.Pairs()[lastIndex].Value
}

func (pl *PhaseLogs) Len() int {
	if pl == nil {
		return 0
	}

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
	mutex        sync.Mutex
	commandLogs  *threadsafe.Slice[*CommandLog]
	timeAndState *time_and_state.TimeAndState
	// Internal
	phase phases.Phase
	flags config_flags.Logging
}

func NewPhaseLog(phase phases.Phase, flags config_flags.Logging) *PhaseLog {
	return &PhaseLog{
		commandLogs:  threadsafe.NewSlice[*CommandLog](),
		timeAndState: time_and_state.NewTimeAndState(),
		phase:        phase,
		flags:        flags,
	}
}

func (pLog *PhaseLog) LastNonMsgOnlyCommand() *CommandLog {
	var commandLog *CommandLog

	// Iterate backwards to find the last command that is not just a message
	for i := pLog.commandLogs.Length() - 1; i >= 0; i-- {
		var ok bool
		commandLog, ok = pLog.commandLogs.Get(i)
		if !ok {
			panic("commandLogs does not have element on specified index")
		}
		if !commandLog.MsgOnly {
			return commandLog
		}
	}

	return nil
}

func (pLog *PhaseLog) NewCommand(description, statusIfFailed string, msgOnly bool) *CommandLog {
	commandLog := NewCommandLog(description, statusIfFailed, msgOnly)
	pLog.commandLogs.Append(commandLog)

	return commandLog
}

func (pLog *PhaseLog) Verbose(format string, a ...any) *CommandLog {
	if !pLog.flags.Verbose {
		return nil
	}

	msg := fmt.Sprintf("VERBOSE "+format, a...)

	return pLog.NewCommand(msg, "", true)
}

func (pLog *PhaseLog) Debug(format string, a ...any) *CommandLog {
	if !pLog.flags.Debug {
		return nil
	}

	msg := fmt.Sprintf("DEBUG "+format, a...)

	return pLog.NewCommand(msg, "", true)
}

func (pLog *PhaseLog) CommandLogs() []*CommandLog {
	return pLog.commandLogs.Values()
}

func (pLog *PhaseLog) Clear() {
	if pLog == nil {
		return
	}

	pLog.mutex.Lock()
	defer pLog.mutex.Unlock()

	pLog.commandLogs.Clear()
	pLog.timeAndState = time_and_state.NewTimeAndState()
}

func (pLog *PhaseLog) TimeAndState() *time_and_state.TimeAndState {
	pLog.mutex.Lock()
	defer pLog.mutex.Unlock()

	return pLog.timeAndState
}

func (pLog *PhaseLog) Phase() phases.Phase {
	return pLog.phase
}
