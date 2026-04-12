package phases

import (
	"slices"

	"github.com/pkg/errors"
)

var (
	ErrAllPhasesSkipped  = errors.New("all phases skipped")
	ErrInvalidFirstPhase = errors.New("phase can't be first")
)

type Phase string

const (
	Inspect   Phase = "inspect"
	Build     Phase = "build"
	Bootstrap Phase = "bootstrap"
	Transfer  Phase = "transfer"
	Secrets   Phase = "secrets"
	Activate  Phase = "activate"

	// Stand-alone phases.

	Rollback Phase = "rollback"
)

func (p Phase) String() string {
	return string(p)
}

// PhaseScope defines at what level a phase should execute.
type PhaseScope int

const (
	ScopeMachine       PhaseScope = iota // Once per machine
	ScopeConfiguration                   // Once per configuration
	ScopeFlake                           // Once per flake
	ScopeFleet                           // Once per fleet
)

// PhaseMetadata defines the behavior of each phase.
type PhaseMetadata struct {
	Phase Phase
	Scope PhaseScope
}

// PhaseRegistry contains metadata for all phases.
// they are defined in execution order.
var PhaseRegistry = []PhaseMetadata{
	{Phase: Inspect, Scope: ScopeMachine},
	{Phase: Build, Scope: ScopeConfiguration},
	{Phase: Bootstrap, Scope: ScopeMachine},
	{Phase: Transfer, Scope: ScopeMachine},
	{Phase: Secrets, Scope: ScopeMachine},
	{Phase: Activate, Scope: ScopeMachine},

	// Stand-alone phases
	{Phase: Rollback, Scope: ScopeMachine},
}

// GetPhaseMetadata returns metadata for a specific phase.
func GetPhaseMetadata(phase Phase) (PhaseMetadata, bool) {
	for _, pm := range PhaseRegistry {
		if pm.Phase == phase {
			return pm, true
		}
	}

	return PhaseMetadata{}, false
}

// DeployPhasesInOrder returns the deploy workflow phases in execution order.
// Stand-alone phases (like Rollback) are not included.
func DeployPhasesInOrder() []Phase {
	return []Phase{Inspect, Build, Bootstrap, Transfer, Secrets, Activate}
}

// TODO: logic in this functon seems off, e.g. DeployPhasesInOrder
func ValidatePhases(requiredPhases []Phase, skipPhases []Phase) ([]Phase, error) {
	phases := DeployPhasesInOrder()

	standalonePhases := []Phase{Rollback}

	phases = slices.DeleteFunc(phases, func(phase Phase) bool {
		return !slices.Contains(requiredPhases, phase)
	})

	for _, sp := range standalonePhases {
		if slices.Contains(requiredPhases, sp) && !slices.Contains(phases, sp) {
			phases = append(phases, sp)
		}
	}

	phases = slices.DeleteFunc(phases, func(phase Phase) bool {
		return slices.Contains(skipPhases, phase)
	})

	if len(phases) == 0 {
		return nil, ErrAllPhasesSkipped
	}

	firstPhase := phases[0]

	validFirst := []Phase{Inspect, Build, Secrets, Rollback}

	if !slices.Contains(validFirst, firstPhase) {
		return nil, errors.Wrapf(ErrInvalidFirstPhase, "%s (allowed: %v)", firstPhase, validFirst)
	}

	return phases, nil
}

// GetPhaseScope returns the scope at which a phase should execute.
func GetPhaseScope(phase Phase) PhaseScope {
	if meta, ok := GetPhaseMetadata(phase); ok {
		return meta.Scope
	}

	return ScopeMachine
}

// ShouldRunOnce returns true if this phase should only run once per scope instance.
func ShouldRunOnce(phase Phase) bool {
	scope := GetPhaseScope(phase)

	return scope == ScopeConfiguration || scope == ScopeFlake
}
