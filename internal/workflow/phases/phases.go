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

	// Checks

	if len(phases) == 0 {
		return nil, fmt.Errorf("all phases skipped")
	}

	firstPhase := phases[0]
	validFirstPhases := []Phase{Inspect, Build, Secrets}
	if !slices.Contains(validFirstPhases, firstPhase) {
		return nil, fmt.Errorf("phase %s is can't be the first phase, allowed are %s", firstPhase, validFirstPhases)
	}

	return phases, nil
}
