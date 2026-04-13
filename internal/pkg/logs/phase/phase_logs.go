package phase

import (
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type PhaseLogs struct {
	*atomicorderedmap.OrderedMap[phases.Phase, *PhaseLog]
}

func NewPhaseLogs() *PhaseLogs {
	return &PhaseLogs{atomicorderedmap.New[phases.Phase, *PhaseLog]()}
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
