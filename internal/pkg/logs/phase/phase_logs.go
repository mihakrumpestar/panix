package phase

import (
	"fmt"

	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var ErrPhaseNotFound = errors.New("key for del does not exist")

// PhaseLogs manages a collection of PhaseLog instances indexed by phase.
type PhaseLogs struct {
	logs  *omap.Omap[phases.Phase, *PhaseLog]
	xpath attributes.Xpath
	flags flags.Logging
}

// NewPhaseLogs creates a new PhaseLogs instance.
func NewPhaseLogs(xpath attributes.Xpath, flags flags.Logging) (*PhaseLogs, error) {
	phaseLogs, err := omap.New[phases.Phase, *PhaseLog]()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create phase logs map for %s", xpath.String())
	}

	return &PhaseLogs{
		logs:  phaseLogs,
		xpath: xpath,
		flags: flags,
	}, nil
}

// Get retrieves a PhaseLog for the given phase, or nil if not found.
func (pl *PhaseLogs) Get(phase phases.Phase) *PhaseLog {
	if pl == nil || pl.logs == nil {
		return nil
	}

	phaseLog, _ := pl.logs.Get(phase)

	return phaseLog
}

// GetOrCreate retrieves or creates a PhaseLog for the given phase.
func (pl *PhaseLogs) GetOrCreate(phase phases.Phase) *PhaseLog {
	phaseLog := pl.Get(phase)
	if phaseLog != nil {
		return phaseLog
	}

	phaseLog = NewPhaseLog(pl.xpath, phase, pl.flags)

	err := pl.logs.Set(phase, phaseLog)
	if err != nil {
		return nil
	}

	return phaseLog
}

// SetIfNotExists sets provided phaseLog to phase,
// or creates a new one and sets it if phaseLog is nil,
// both only if phase does not already exist.
func (pl *PhaseLogs) SetIfNotExists(phase phases.Phase, phaseLog *PhaseLog) *PhaseLog {
	pLog, ok := pl.logs.Get(phase)
	if ok {
		return pLog
	}

	if phaseLog == nil {
		phaseLog = NewPhaseLog(pl.xpath, phase, pl.flags)
	}

	err := pl.logs.Set(phase, phaseLog)
	if err != nil {
		panic(fmt.Sprintf("failed to set phase log: %v", err))
	}

	return phaseLog
}

// All returns all phase-log pairs as a slice.
func (pl *PhaseLogs) All() []omap.Pair[phases.Phase, *PhaseLog] {
	if pl == nil || pl.logs == nil {
		return nil
	}

	return pl.logs.Pairs()
}

// First returns the first inserted PhaseLog, or nil if empty.
func (pl *PhaseLogs) First() *PhaseLog {
	if pl == nil || pl.logs == nil {
		return nil
	}

	pairs := pl.logs.Pairs()
	if len(pairs) == 0 {
		return nil
	}

	return pairs[0].Value
}

// Last returns the last inserted PhaseLog, or nil if empty.
func (pl *PhaseLogs) Last() *PhaseLog {
	if pl == nil || pl.logs == nil {
		return nil
	}

	pairs := pl.logs.Pairs()
	if len(pairs) == 0 {
		return nil
	}

	return pairs[len(pairs)-1].Value
}

// Len returns the number of phase logs.
func (pl *PhaseLogs) Len() int {
	if pl == nil || pl.logs == nil {
		return 0
	}

	return pl.logs.Len()
}

// Del removes a phase log by phase.
func (pl *PhaseLogs) Del(phase phases.Phase) error {
	_, ok := pl.logs.Del(phase)
	if !ok {
		return errors.Wrapf(ErrPhaseNotFound, "%v", phase)
	}

	return nil
}

// Clear removes all phase logs.
func (pl *PhaseLogs) Clear() {
	pl.logs.Clear()
}
