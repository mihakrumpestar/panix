package phase

import (
	"slices"

	"github.com/pkg/errors"
)

var (
	ErrAllPhasesSkipped  = errors.New("all phases skipped")
	ErrInspectRequired   = errors.New("inspect phase required")
	ErrInvalidFirstPhase = errors.New("phase can't be first")
	ErrUnknownPhase      = errors.New("unknown phase")
)

type Phase string

const (
	Inspect   Phase = "inspect"
	Build     Phase = "build"
	Bootstrap Phase = "bootstrap"
	Transfer  Phase = "transfer"
	Secrets   Phase = "secrets"
	Activate  Phase = "activate"
	Rollback  Phase = "rollback"
)

func (p Phase) String() string {
	return string(p)
}

// GetPhaseScope returns the scope at which a phase should execute.
func (p Phase) GetPhaseScope() PhaseScope {
	meta, ok := GetPhaseMetadata(p)
	if ok {
		return meta.Scope
	}

	return ScopeMachine
}

// ShouldRunOnce returns true if this phase should only run once per scope instance.
func (p Phase) ShouldRunOnce() bool {
	scope := p.GetPhaseScope()

	return scope != ScopeMachine
}

// PhaseScope defines at what level a phase should execute.
type PhaseScope int

const (
	ScopeMachine     PhaseScope = iota // Once per machine
	ScopeInstallable                   // Once per installable
	ScopeFlake                         // Once per flake
	ScopeFleet                         // Once per fleet
)

// PhaseMetadata defines the behavior of each phase.
type PhaseMetadata struct {
	Phase      Phase
	Scope      PhaseScope
	ValidFirst bool
}

// PhaseRegistry contains metadata for all phases, they are defined in execution order (but not all phases are all orders).
var PhaseRegistry = []PhaseMetadata{
	{Phase: Inspect, Scope: ScopeMachine, ValidFirst: true},
	{Phase: Bootstrap, Scope: ScopeMachine},
	{Phase: Build, Scope: ScopeInstallable, ValidFirst: true},
	{Phase: Transfer, Scope: ScopeMachine},
	{Phase: Secrets, Scope: ScopeMachine},
	{Phase: Activate, Scope: ScopeMachine},
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

// PhasesInOrder returns the standard deploy workflow phases in execution order.
func PhasesInOrder() []Phase {
	result := make([]Phase, len(PhaseRegistry))

	for idx, pm := range PhaseRegistry {
		result[idx] = pm.Phase
	}

	return result
}

// ValidatePhases filters and validates the requested phases against the registry.
// It preserves the execution order defined in PhaseRegistry, removes skipped phases,
// and validates that the first phase is allowed to start a workflow.
func ValidatePhases(requestedPhases []Phase, skipPhases []Phase) ([]Phase, error) {
	for _, p := range requestedPhases {
		_, ok := GetPhaseMetadata(p)
		if !ok {
			return nil, errors.Wrapf(ErrUnknownPhase, "%s", p)
		}
	}

	var result []Phase

	for _, pm := range PhaseRegistry {
		if slices.Contains(requestedPhases, pm.Phase) {
			result = append(result, pm.Phase)
		}
	}

	result = slices.DeleteFunc(result, func(phase Phase) bool {
		return slices.Contains(skipPhases, phase)
	})

	if len(result) == 0 {
		return nil, ErrAllPhasesSkipped
	}

	inspectErr := validateInspectSkip(skipPhases, result)
	if inspectErr != nil {
		return nil, inspectErr
	}

	firstPhase := result[0]

	meta, ok := GetPhaseMetadata(firstPhase)
	if !ok || !meta.ValidFirst {
		validFirst := validFirstPhases()

		return nil, errors.Wrapf(ErrInvalidFirstPhase, "%s (allowed: %v)", firstPhase, validFirst)
	}

	inspectErr = validateInspectRequirement(result)
	if inspectErr != nil {
		return nil, inspectErr
	}

	return result, nil
}

// validateInspectSkip fails closed when inspect is explicitly skipped: the
// remaining phases cannot tell a fresh machine from an already-installed one,
// and Activate would install to /mnt. It runs before the first-phase check, so
// an explicit skip reports the missing inspect requirement.
func validateInspectSkip(skipPhases, result []Phase) error {
	if !slices.Contains(skipPhases, Inspect) {
		return nil
	}

	return requireBuildOnly(result)
}

// validateInspectRequirement enforces the same invariant when inspect was never
// requested. It runs after the first-phase check, so invalid first phases keep
// their specific error. Build is the only phase that can run offline; every
// other phase depends on inspect.
func validateInspectRequirement(result []Phase) error {
	if slices.Contains(result, Inspect) {
		return nil
	}

	return requireBuildOnly(result)
}

func requireBuildOnly(result []Phase) error {
	for _, phase := range result {
		if phase != Build {
			return errors.Wrapf(ErrInspectRequired, "cannot run %s without inspect", phase)
		}
	}

	return nil
}

func validFirstPhases() []Phase {
	var result []Phase

	for _, pm := range PhaseRegistry {
		if pm.ValidFirst {
			result = append(result, pm.Phase)
		}
	}

	return result
}
