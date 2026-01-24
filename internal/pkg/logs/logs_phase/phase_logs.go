package logs_phase

import (
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// PhaseLogs

type PhaseLogs struct {
	logs  *omap.Omap[phases.Phase, *PhaseLog]
	xpath config_attributes.Xpath
	flags config_flags.Logging
}

func NewPhaseLogs(xpath config_attributes.Xpath, flags config_flags.Logging) *PhaseLogs {
	phaseLogs, err := omap.New[phases.Phase, *PhaseLog]()
	if err != nil {
		panic(err)
	}

	return &PhaseLogs{
		phaseLogs,
		xpath,
		flags,
	}
}

func (pl *PhaseLogs) Get(phase phases.Phase) *PhaseLog {
	phaselog, ok := pl.logs.Get(phase)
	if !ok {
		return nil
	}

	return phaselog
}

func (pl *PhaseLogs) GetOrCreate(phase phases.Phase) *PhaseLog {
	phaselog := pl.Get(phase)
	if phaselog == nil {
		phaselog = NewPhaseLog(pl.xpath, phase, pl.flags)
		pl.logs.Set(phase, phaselog)
	}

	return phaselog
}

// SetIfNotExists sets provided phaseLog to phase,
// or creates a new one and sets it if phaseLog is nil,
// both only if phase does not already exist
func (pl *PhaseLogs) SetIfNotExists(phase phases.Phase, phaseLog *PhaseLog) *PhaseLog {
	pLog, ok := pl.logs.Get(phase)
	if ok {
		return pLog
	}

	if phaseLog == nil {
		phaseLog = NewPhaseLog(pl.xpath, phase, pl.flags)
	}

	pl.logs.Set(phase, phaseLog)

	return phaseLog
}

func (pl *PhaseLogs) All() []omap.Pair[phases.Phase, *PhaseLog] {
	return pl.logs.Pairs() // Any other ranging method locks the dict until it traverses it
}

// Gets the first inserted PhaseLog from PhaseLogs
func (pl *PhaseLogs) First() *PhaseLog {
	if pl.logs.Len() == 0 {
		return nil
	}

	return pl.logs.Pairs()[0].Value
}

// Gets the last inserted PhaseLog from PhaseLogs
func (pl *PhaseLogs) Last() *PhaseLog {
	if pl.logs.Len() == 0 {
		return nil
	}

	lastIndex := pl.logs.Len() - 1

	return pl.logs.Pairs()[lastIndex].Value
}

func (pl *PhaseLogs) Len() int {
	if pl == nil {
		return 0
	}

	return pl.logs.Len()
}

func (pl *PhaseLogs) Del(phase phases.Phase) {
	_, ok := pl.logs.Del(phase)
	if !ok {
		panic("key for del does not exist")
	}
}

func (pl *PhaseLogs) Clear() {
	pl.logs.Clear()
}
