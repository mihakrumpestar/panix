package tasks

import (
	"github.com/hayageek/threadsafe"
)

type State string

const (
	Running State = "running"
	Failed  State = "failed"
	Done    State = "done" // Only for the "done" phase
)

type PhaseTasksStates struct {
	tasks *threadsafe.Map[string, State]
}

func NewPhaseTasksStates() *PhaseTasksStates {
	return &PhaseTasksStates{
		threadsafe.NewMap[string, State](),
	}
}

func (t *PhaseTasksStates) Len() int {
	return t.tasks.Length()
}

func (t *PhaseTasksStates) LenByState(state State) int {
	count := 0

	for _, value := range t.tasks.Values() {
		if value == state {
			count++
		}
	}

	return count
}

func (t *PhaseTasksStates) Set(task string, state State) {
	t.tasks.Set(task, state)
}

func (t *PhaseTasksStates) Remove(task string) {
	t.tasks.Delete(task)
}
