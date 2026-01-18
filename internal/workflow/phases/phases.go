package phases

import (
	"fmt"
	"iter"
	"slices"
	"strconv"

	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/workflow/tasks"
)

type Phase string

const (
	Status    Phase = "status"
	Build     Phase = "build"
	Bootstrap Phase = "bootstrap"
	Transfer  Phase = "transfer"
	Secrets   Phase = "secrets"
	Activate  Phase = "activate"
	Done      Phase = "done"
)

func PhasesInOrder() []Phase {
	return []Phase{
		Status,
		Build,
		Bootstrap,
		Transfer,
		Secrets,
		Activate,
		Done,
	}
}

type PhaseStates struct {
	states *omap.Omap[Phase, *tasks.PhaseTasksStates]
}

func NewPhaseStates(requiredPhases []Phase, skipPhases []Phase) (*PhaseStates, error) {
	states, err := omap.New[Phase, *tasks.PhaseTasksStates]()
	if err != nil {
		panic(err)
	}

	phasesInOrder := PhasesInOrder()

	// Keep only required phases
	phasesInOrder = slices.DeleteFunc(phasesInOrder, func(phase Phase) bool {
		return !slices.Contains(requiredPhases, phase) && phase != Done
	})

	// Remove skipped phases
	if slices.Contains(skipPhases, Done) {
		return nil, fmt.Errorf("%s phase can't be skipped", strconv.Quote(string(Done)))
	}

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

	// Initialize task status for each phase
	for _, phase := range phasesInOrder {
		err := states.Set(phase, tasks.NewPhaseTasksStates())
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

	ps.states.ForEach(func(key Phase, data *tasks.PhaseTasksStates) {
		phases = append(phases, key)
	})

	return phases
}

func (ps *PhaseStates) Value(phase Phase) *tasks.PhaseTasksStates {
	value, ok := ps.states.Get(phase)
	if !ok {
		panic("phaseState with given key does not exist")
	}

	return value
}

func (ps *PhaseStates) Range() iter.Seq2[Phase, *tasks.PhaseTasksStates] {
	return ps.states.Records()
}
