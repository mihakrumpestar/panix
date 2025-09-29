package config

import (
	"errors"
	"iter"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// PhaseLogs

type PhaseLogs struct {
	logs *orderedmap.OrderedMap[phases.Phase, *PhaseLog]
}

func NewPhaseLogs() *PhaseLogs {
	return &PhaseLogs{
		logs: orderedmap.NewOrderedMap[phases.Phase, *PhaseLog](),
	}
}

func (l *PhaseLogs) SafeGet(phase phases.Phase) *PhaseLog {
	log, ok := l.logs.Get(phase)

	if !ok {
		log = &PhaseLog{
			Commands:     make([]*CommandLog, 0),
			TimeAndState: &TimeAndState{},
		}

		l.logs.Set(phase, log)
	}

	return log
}

func (l *PhaseLogs) All() iter.Seq2[phases.Phase, *PhaseLog] {
	return l.logs.AllFromFront()
}

func (l *PhaseLogs) Len() int {
	return l.logs.Len()
}

// PhaseLog

type PhaseLog struct {
	Commands     []*CommandLog
	TimeAndState *TimeAndState
}

func (log *PhaseLog) LastCommand() (*CommandLog, error) {
	if len(log.Commands) == 0 {
		return nil, errors.New("commands log empty")
	}

	return log.Commands[len(log.Commands)-1], nil
}

func (log *PhaseLog) NewCommand() *CommandLog {
	cmd := &CommandLog{}
	log.Commands = append(log.Commands, cmd)

	return cmd
}

func (log *PhaseLog) AddMessageOnly(msg ...string) *CommandLog {
	commandLog := &CommandLog{
		Command: "-> ",
	}

	for _, msgInstance := range msg {
		commandLog.Command += msgInstance
	}

	commandLog.TimeAndState.StartTimer()
	commandLog.TimeAndState.EndTimer()

	if log.Commands == nil {
		log.Commands = make([]*CommandLog, 0)
	}

	log.Commands = append(log.Commands, commandLog)

	return commandLog
}
