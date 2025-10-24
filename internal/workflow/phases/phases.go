package phases

import (
	"fmt"
	"iter"
	"slices"
	"strconv"

	"github.com/hayageek/threadsafe"
	"github.com/kirill-scherba/omap"
)

type Phase string

const (
	Status            Phase = "status"
	PreFlakeHook      Phase = "pre-flake-hook"
	Build             Phase = "build"
	Bootstrap         Phase = "bootstrap"
	PostBootstrapHook Phase = "post-bootstrap-hook"
	Transfer          Phase = "transfer"
	Secrets           Phase = "secrets"
	Activate          Phase = "activate"
	Done              Phase = "done"
	PostFlakeHook     Phase = "post-flake-hook"
)

func PhasesInOrder() []Phase {
	return []Phase{
		Status,
		PreFlakeHook,
		Build,
		Bootstrap,
		PostBootstrapHook,
		Transfer,
		Secrets,
		Activate,
		Done,
		PostFlakeHook,
	}
}

type PhaseState struct {
	Running *Tasks
	Failed  *Tasks
	Done    *Tasks // Only for the "done" phase
}

type Tasks struct {
	tasks *threadsafe.Slice[string]
}

func NewTasks() *Tasks {
	return &Tasks{
		threadsafe.NewSlice[string](),
	}
}

func (t *Tasks) List() []string {
	return t.tasks.Values()
}

func (t *Tasks) Len() int {
	return t.tasks.Length()
}

func (t *Tasks) Add(task string) {
	t.tasks.Append(task)
}

func (t *Tasks) Rem(task string) {
	index := -1
	for i, value := range t.tasks.Values() {
		if value == task {
			index = i
			break
		}
	}

	if index == -1 {
		panic("key was not found in valueKeys")
	}

	t.tasks.Remove(index)
}

type PhaseStates struct {
	states *omap.Omap[Phase, *PhaseState]
}

func NewPhaseStates(requiredPhases []Phase, skipPhases []Phase) (*PhaseStates, error) {
	states, err := omap.New[Phase, *PhaseState]()
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
		err := states.Set(phase, &PhaseState{
			Running: NewTasks(),
			Failed:  NewTasks(),
			Done:    NewTasks(),
		})
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

	ps.states.ForEach(func(key Phase, data *PhaseState) {
		phases = append(phases, key)
	})

	return phases
}

func (ps *PhaseStates) Value(phase Phase) *PhaseState {
	value, ok := ps.states.Get(phase)
	if !ok {
		panic("phaseState with given key does not exist")
	}

	return value
}

func (ps *PhaseStates) Range() iter.Seq2[Phase, *PhaseState] {
	return ps.states.Records()
}
