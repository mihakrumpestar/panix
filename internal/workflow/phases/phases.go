package phases

import (
	"fmt"
	"iter"
	"slices"

	"github.com/hayageek/threadsafe"
	"github.com/kirill-scherba/omap"
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

func PhasesInOrder() []Phase {
	return []Phase{
		Status,
		PreFlakeHook,
		Build,
		Bootstrap,
		Transfer,
		Secrets,
		Activate,
		Rollback,
		PostFlakeHook,
	}
}

type PhaseStates struct {
	states *omap.Omap[Phase, *threadsafe.Slice[string]]
}

func NewPhaseStates(requiredPhases []Phase, skipPhases []Phase) (*PhaseStates, error) {
	states, err := omap.New[Phase, *threadsafe.Slice[string]]()
	if err != nil {
		panic(err)
	}

	phasesInOrder := PhasesInOrder()

	// Keep only required phases
	phasesInOrder = slices.DeleteFunc(phasesInOrder, func(phase Phase) bool {
		return !slices.Contains(requiredPhases, phase)
	})

	// Remove skipped phases
	phasesInOrder = slices.DeleteFunc(phasesInOrder, func(phase Phase) bool {
		return slices.Contains(skipPhases, phase)
	})

	// Checks

	if len(phasesInOrder) == 0 {
		return nil, fmt.Errorf("all phases skipped")
	}

	firstPhase := phasesInOrder[0]
	validFirstPhases := []Phase{Status, Build, Secrets}
	if !slices.Contains(validFirstPhases, firstPhase) {
		return nil, fmt.Errorf("phase %s is can't be the first phase, allowed are %s", firstPhase, validFirstPhases)
	}

	for _, phase := range phasesInOrder {
		err := states.Set(phase, threadsafe.NewSlice[string]())
		if err != nil {
			panic(err)
		}
	}

	phaseStates := &PhaseStates{
		states: states,
	}

	return phaseStates, nil
}

func (ps *PhaseStates) Keys() []Phase {
	phases := make([]Phase, 0)

	ps.states.ForEach(func(key Phase, data *threadsafe.Slice[string]) {
		phases = append(phases, key)
	})

	return phases
}

func (ps *PhaseStates) Value(phase Phase) *threadsafe.Slice[string] {
	value, ok := ps.states.Get(phase)
	if !ok {
		panic("phaseState with given key does not exist")
	}

	return value
}

func (ps *PhaseStates) Range() iter.Seq2[Phase, *threadsafe.Slice[string]] {
	return ps.states.Records()
}

func (ps *PhaseStates) AddKeyToValue(phase Phase, key string) {
	value, ok := ps.states.Get(phase)
	if !ok {
		panic("phaseState with given key does not exist")
	}

	value.Append(key)
}

func (ps *PhaseStates) RemoveKeyFromValue(phase Phase, key string) {
	value, ok := ps.states.Get(phase)
	if !ok {
		panic("phaseState with given key does not exist")
	}

	index := -1
	for i, valueKey := range value.Values() {
		if valueKey == key {
			index = i
			break
		}
	}

	if index == -1 {
		panic("key was not found in valueKeys")
	}

	value.Remove(index)
}
