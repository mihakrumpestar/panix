package config

import (
	"errors"
	"iter"
	"slices"
	"strings"
	"sync"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// PhaseLogs

type PhaseLogs struct {
	mutex sync.Mutex
	logs  *orderedmap.OrderedMap[phases.Phase, *PhaseLog]
}

func NewPhaseLogs() *PhaseLogs {
	return &PhaseLogs{
		logs: orderedmap.NewOrderedMap[phases.Phase, *PhaseLog](),
	}
}

func (l *PhaseLogs) SafeGet(phase phases.Phase) *PhaseLog {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	phaselog, ok := l.logs.Get(phase)
	if !ok {
		phaselog = NewPhaseLog()
		l.logs.Set(phase, phaselog)
	}

	return phaselog
}

func (l *PhaseLogs) All() iter.Seq2[phases.Phase, *PhaseLog] {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	tmp := orderedmap.NewOrderedMap[phases.Phase, *PhaseLog]()
	for phase, phaseLog := range l.logs.AllFromFront() {
		tmp.Set(phase, phaseLog)
	}

	return l.logs.AllFromFront()
}

func (l *PhaseLogs) Len() int {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	return l.logs.Len()
}

// PhaseLog

type PhaseLog struct {
	mutex        sync.Mutex
	commandLogs  []*CommandLog
	TimeAndState *TimeAndState
}

func NewPhaseLog() *PhaseLog {
	return &PhaseLog{
		commandLogs:  make([]*CommandLog, 0),
		TimeAndState: NewTimeAndState(),
	}
}

func (pLog *PhaseLog) LastCommand() (*CommandLog, error) {
	pLog.mutex.Lock()
	defer pLog.mutex.Unlock()

	if len(pLog.commandLogs) == 0 {
		return nil, errors.New("commands log empty")
	}

	return pLog.commandLogs[len(pLog.commandLogs)-1], nil
}

func (pLog *PhaseLog) NewCommand() *CommandLog {
	pLog.mutex.Lock()
	defer pLog.mutex.Unlock()

	commandLog := NewCommandLog()
	pLog.commandLogs = append(pLog.commandLogs, commandLog)

	return commandLog
}

func (pLog *PhaseLog) AddMessageOnly(msg ...string) *CommandLog {
	commandLog := pLog.NewCommand()

	pLog.mutex.Lock()
	defer pLog.mutex.Unlock()

	commandLog.Command.Store("-> " + strings.Join(msg, ""))

	commandLog.TimeAndState.StartTimer()
	commandLog.TimeAndState.EndTimer()

	return commandLog
}

func (pLog *PhaseLog) CommandLogs() []*CommandLog {
	pLog.mutex.Lock()
	defer pLog.mutex.Unlock()

	tmp := slices.Clone(pLog.commandLogs)

	return tmp
}
