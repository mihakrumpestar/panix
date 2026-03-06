package phases

import (
	"fmt"
	"slices"
)

type Phase string

const (
	Inspect   Phase = "inspect"
	Build     Phase = "build"
	Bootstrap Phase = "bootstrap"
	Transfer  Phase = "transfer"
	Secrets   Phase = "secrets"
	Activate  Phase = "activate"
)

// PhaseScope defines at what level a phase should execute
type PhaseScope int

const (
	ScopeMachine PhaseScope = iota // Once per machine
	ScopeConfig                    // Once per configuration
	ScopeFlake                     // Once per flake
)

// PhaseMetadata defines the behavior of each phase
type PhaseMetadata struct {
	Phase Phase
	Scope PhaseScope
}

// PhaseRegistry contains metadata for all phases
// they are defined in execution order
var PhaseRegistry = []PhaseMetadata{
	{Phase: Inspect, Scope: ScopeMachine},
	{Phase: Build, Scope: ScopeConfig}, // Once per config
	{Phase: Bootstrap, Scope: ScopeMachine},
	{Phase: Transfer, Scope: ScopeMachine},
	{Phase: Secrets, Scope: ScopeMachine},
	{Phase: Activate, Scope: ScopeMachine},
}

// GetPhaseMetadata returns metadata for a specific phase
func GetPhaseMetadata(phase Phase) (PhaseMetadata, bool) {
	for _, pm := range PhaseRegistry {
		if pm.Phase == phase {
			return pm, true
		}
	}

	return PhaseMetadata{}, false
}

// PhasesInOrder returns phases in their defined order
// PhaseRegistry is already defined in execution order
func PhasesInOrder() []Phase {
	result := make([]Phase, len(PhaseRegistry))
	for i, pm := range PhaseRegistry {
		result[i] = pm.Phase
	}

	return result
}

func ValidatePhases(requiredPhases []Phase, skipPhases []Phase) ([]Phase, error) {
	phases := PhasesInOrder()

	// Keep only required phases
	phases = slices.DeleteFunc(phases, func(phase Phase) bool {
		return !slices.Contains(requiredPhases, phase)
	})

	// Remove skipped phases
	phases = slices.DeleteFunc(phases, func(phase Phase) bool {
		return slices.Contains(skipPhases, phase)
	})

	// Validation
	if len(phases) == 0 {
		return nil, fmt.Errorf("all phases skipped")
	}

	firstPhase := phases[0]

	validFirst := []Phase{Inspect, Build, Secrets}

	if !slices.Contains(validFirst, firstPhase) {
		return nil, fmt.Errorf("phase %s can't be first, allowed: %v", firstPhase, validFirst)
	}

	return phases, nil
}

// GetPhaseScope returns the scope at which a phase should execute
func GetPhaseScope(phase Phase) PhaseScope {
	if meta, ok := GetPhaseMetadata(phase); ok {
		return meta.Scope
	}

	return ScopeMachine
}

// ShouldRunOnce returns true if this phase should only run once per scope instance
func ShouldRunOnce(phase Phase) bool {
	scope := GetPhaseScope(phase)

	return scope == ScopeConfig || scope == ScopeFlake
}
