package config

import (
	"iter"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

// Logs

type Logs struct {
	logs *orderedmap.OrderedMap[workflow_definition.WorkflowPhase, *Log]
}

func NewLogs() *Logs {
	return &Logs{
		logs: orderedmap.NewOrderedMap[workflow_definition.WorkflowPhase, *Log](),
	}
}

func (l *Logs) SafeGet(wp workflow_definition.WorkflowPhase) *Log {
	log, ok := l.logs.Get(wp)

	if !ok {
		log = &Log{
			Commands:     make([]*CommandLog, 0),
			TimeAndState: &TimeAndState{},
		}

		l.logs.Set(wp, log)
	}

	return log
}

func (l *Logs) All() iter.Seq2[workflow_definition.WorkflowPhase, *Log] {
	return l.logs.AllFromFront()
}

func (l *Logs) Len() int {
	return l.logs.Len()
}

// Log

type Log struct {
	Commands     []*CommandLog
	TimeAndState *TimeAndState
}

func (log *Log) LastCommand() *CommandLog {
	// Safety
	if len(log.Commands) == 0 {
		return &CommandLog{TimeAndState: TimeAndState{}}
	}

	return log.Commands[len(log.Commands)-1]
}

func (log *Log) NewCommand() *CommandLog {
	cmd := &CommandLog{TimeAndState: TimeAndState{}}

	log.Commands = append(log.Commands, cmd)

	return cmd
}

func (log *Log) AddMessageOnly(msg ...string) {
	comLog := &CommandLog{
		Command: "-> ",
	}

	for _, msgInstance := range msg {
		comLog.Command += msgInstance
	}

	comLog.TimeAndState.StartTimer()
	comLog.TimeAndState.EndTimer()

	if log.Commands == nil {
		log.Commands = make([]*CommandLog, 0)
	}

	log.Commands = append(log.Commands, comLog)
}
