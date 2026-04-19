package phaselogs

import (
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

type PhaseLogs struct {
	*atomicorderedmap.AtomicOrderedMap[phase.Phase, *PhaseLog]
}

func NewPhaseLogs() *PhaseLogs {
	return &PhaseLogs{atomicorderedmap.New[phase.Phase, *PhaseLog]()}
}

func (pl *PhaseLogs) UnmarshalJSON(data []byte) error {
	if pl.AtomicOrderedMap == nil {
		pl.AtomicOrderedMap = atomicorderedmap.New[phase.Phase, *PhaseLog]()
	}

	return errors.Wrap(pl.AtomicOrderedMap.UnmarshalJSON(data), "unmarshal phase logs")
}

// GetOrCreate retrieves or creates a PhaseLog for the given phase.
func (pl *PhaseLogs) GetOrCreate(phase phase.Phase) *PhaseLog {
	phaseLog, ok := pl.AtomicOrderedMap.Get(phase)
	if ok {
		return phaseLog
	}

	phaseLog = NewPhaseLog()
	pl.AtomicOrderedMap.Set(phase, phaseLog)

	return phaseLog
}
