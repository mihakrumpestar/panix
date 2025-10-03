package phases

import (
	"fmt"
	"iter"
	"slices"
	"sync"

	"github.com/elliotchance/orderedmap/v3"
)

type Phase string

const (
	Status        Phase = "status"
	PreFlakeHook  Phase = "pre-flake-hook"
	Build         Phase = "build"
	Bootstrap     Phase = "bootstrap"
	Transfer      Phase = "transfer"
	Secrets       Phase = "secrets"
	Activate      Phase = "activate"
	Rollback      Phase = "rollback"
	PostFlakeHook Phase = "post-flake-hook"
)

type PhaseStates struct {
	mutex  sync.Mutex
	states *orderedmap.OrderedMap[Phase, []string]
}

func NewPhaseStates(requiredPhases []Phase, skipPhases []Phase) (*PhaseStates, error) {
	phaseStates := &PhaseStates{
		states: orderedmap.NewOrderedMap[Phase, []string](),
	}

	phaseStates.states.Set(Status, make([]string, 0))
	phaseStates.states.Set(PreFlakeHook, make([]string, 0))
	phaseStates.states.Set(Build, make([]string, 0))
	phaseStates.states.Set(Bootstrap, make([]string, 0))
	phaseStates.states.Set(Transfer, make([]string, 0))
	phaseStates.states.Set(Secrets, make([]string, 0))
	phaseStates.states.Set(Activate, make([]string, 0))
	phaseStates.states.Set(Rollback, make([]string, 0))
	phaseStates.states.Set(PostFlakeHook, make([]string, 0))

	// Keep only required phases
	for phase := range phaseStates.states.Keys() {
		if !slices.Contains(requiredPhases, phase) {
			phaseStates.states.Delete(phase)
		}
	}

	// Remove skipped phases
	for phase := range phaseStates.states.Keys() {
		if slices.Contains(skipPhases, phase) {
			phaseStates.states.Delete(phase)
		}
	}

	// Checks

	if phaseStates.states.Len() == 0 {
		return nil, fmt.Errorf("all phases skipped")
	}

	phase := phaseStates.states.Front().Key
	validFirstPhases := []Phase{Status, Build, Secrets}
	if !slices.Contains(validFirstPhases, phase) {
		return nil, fmt.Errorf("phase %s is can't be the first phase, allowed are %s", phase, validFirstPhases)
	}

	return phaseStates, nil
}

func (ps *PhaseStates) Keys() []Phase {
	return slices.Collect(ps.states.Keys())
}

func (ps *PhaseStates) Value(phase Phase) []string {
	value, _ := ps.states.Get(phase)

	return value
}

func (ps *PhaseStates) Range() iter.Seq2[Phase, []string] {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Temporary data structure to prevent data races
	tmp := orderedmap.NewOrderedMap[Phase, []string]()
	for key, value := range ps.states.AllFromFront() {
		tmp.Set(key, value)
	}

	return tmp.AllFromFront()
}

func (ps *PhaseStates) AddKeyToValue(phase Phase, key string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	value, _ := ps.states.Get(phase)
	value = append(value, key)

	ps.states.Set(phase, value)
}

func (ps *PhaseStates) RemoveKeyFromValue(phase Phase, key string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	value, _ := ps.states.Get(phase)
	value = slices.DeleteFunc(value, func(cmp string) bool {
		return cmp == key
	})
	ps.states.Set(phase, value)
}
