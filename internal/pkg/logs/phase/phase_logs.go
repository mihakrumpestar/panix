package phase

import (
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type PhaseLogs struct {
	*atomicorderedmap.AtomicOrderedMap[phases.Phase, *PhaseLog]
}

func NewPhaseLogs() *PhaseLogs {
	return &PhaseLogs{atomicorderedmap.New[phases.Phase, *PhaseLog]()}
}

func (pl *PhaseLogs) UnmarshalJSON(data []byte) error {
	if pl.AtomicOrderedMap == nil {
		pl.AtomicOrderedMap = atomicorderedmap.New[phases.Phase, *PhaseLog]()
	}

	return pl.AtomicOrderedMap.UnmarshalJSON(data)
}

// Get retrieves a PhaseLog for the given phase, or nil if not found.
func (pl *PhaseLogs) MustGet(phase phases.Phase) *PhaseLog {
	if pl == nil || pl.AtomicOrderedMap == nil {
		return nil
	}

	phaseLog, _ := pl.AtomicOrderedMap.Get(phase)

	return phaseLog
}

// GetOrCreate retrieves or creates a PhaseLog for the given phase.
func (pl *PhaseLogs) GetOrCreate(phase phases.Phase) *PhaseLog {
	phaseLog := pl.MustGet(phase)
	if phaseLog != nil {
		return phaseLog
	}

	phaseLog = NewPhaseLog()
	pl.AtomicOrderedMap.Set(phase, phaseLog)

	return phaseLog
}
