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

func PhasesInOrder() []Phase {
	return []Phase{
		Inspect,
		Build,
		Bootstrap,
		Transfer,
		Secrets,
		Activate,
	}
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
