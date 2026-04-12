package phase

import (
	"github.com/mihakrumpestar/panix/internal/pkg/orderedmap"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var ErrPhaseNotFound = errors.New("key for del does not exist")

// PhaseLogs manages a collection of PhaseLog instances indexed by phase.
type PhaseLogs struct {
	*orderedmap.OrderedMap[phases.Phase, *PhaseLog]
}

// NewPhaseLogs creates a new PhaseLogs instance.
func NewPhaseLogs() *PhaseLogs {
	return &PhaseLogs{orderedmap.New[phases.Phase, *PhaseLog]()}
}

// Get retrieves a PhaseLog for the given phase, or nil if not found.
func (pl *PhaseLogs) MustGet(phase phases.Phase) *PhaseLog {
	if pl == nil || pl.OrderedMap == nil {
		return nil
	}

	phaseLog, _ := pl.OrderedMap.Get(phase)

	return phaseLog
}

// GetOrCreate retrieves or creates a PhaseLog for the given phase.
func (pl *PhaseLogs) GetOrCreate(phase phases.Phase) *PhaseLog {
	phaseLog := pl.MustGet(phase)
	if phaseLog != nil {
		return phaseLog
	}

	phaseLog = NewPhaseLog()
	pl.OrderedMap.Set(phase, phaseLog)

	return phaseLog
}

// All returns all phase-log pairs as a slice.
func (pl *PhaseLogs) All() []orderedmap.Pair[phases.Phase, *PhaseLog] {
	if pl == nil || pl.OrderedMap == nil {
		return nil
	}

	return pl.OrderedMap.Pairs()
}
