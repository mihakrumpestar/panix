package logs

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
)

// Logs holds the log state embedded in Fleet, Flake, Configuration, Machine.
type Logs struct {
	PhaseLogs *phaselogs.PhaseLogs             `yaml:"-" json:"phase_logs"`
	TAS       *atomictimeandstate.TimeAndState `yaml:"-" json:"tas,omitempty"`
}

func New() *Logs {
	return &Logs{
		PhaseLogs: phaselogs.NewPhaseLogs(),
		TAS:       &atomictimeandstate.TimeAndState{},
	}
}

// Version returns the TAS state version counter. Increments on state transitions.
func (l *Logs) Version() uint64 {
	return l.TAS.StateVersion()
}

// SetDuration replaces the cached duration on TAS without bumping stateVersion.
func (l *Logs) SetDuration(d time.Duration) {
	l.TAS.SetDuration(d)
}

func (l *Logs) Clear() {
	l.PhaseLogs.Clear()
	l.TAS = &atomictimeandstate.TimeAndState{}
}

func (l *Logs) PostUnmarshalInit() {
	if l == nil {
		return
	}

	if l.PhaseLogs == nil {
		l.PhaseLogs = phaselogs.NewPhaseLogs()
	}

	if l.TAS == nil {
		l.TAS = &atomictimeandstate.TimeAndState{}
	}
}

// MergePhaseLogs merges multiple PhaseLogs into a fresh Logs object.
// The returned Logs.TAS has StartTime/EndTime set as state markers
// (for HasStarted/IsFinished) but stateVersion is not bumped — the
// caller is responsible for syncing into a persisted TAS via SyncFrom.
func MergePhaseLogs(phasesInOrder []phase.Phase, input ...*phaselogs.PhaseLogs) *Logs {
	logs := New()

	gathered := gatherPhaseLogs(input)

	anyStarted := false

	// Add phases in order (stopping after the first errored or still-running phase).
	// Phases from higher scopes (e.g. configuration-level "build") may already be
	// finished by another machine's goroutine via OnceAsync, but if the previous
	// machine-level phase hasn't completed yet the machine hasn't actually reached
	// this phase — so we must not include it in the merged view.
	for _, phase := range phasesInOrder {
		phaseLog, ok := gathered.Get(phase)
		if !ok {
			continue
		}

		logs.PhaseLogs.Set(phase, phaseLog)

		tas := phaseLog.TimeAndState
		phaseDOET, _ := tas.DurationOrElapsedTime()
		tasLoaded := tas.Load()

		logs.TAS.DurationCache += phaseDOET
		logs.TAS.EndError = tasLoaded.EndError

		if tasLoaded.HasStarted() {
			anyStarted = true
		}

		if tasLoaded.EndError != nil {
			break
		}

		if !tasLoaded.IsFinished() {
			break
		}
	}

	// Set state markers on TAS so HasStarted/IsFinished return correct values.
	// These are approximate timestamps — the real times live on per-phase TAS.
	if anyStarted {
		logs.TAS.StartTime = time.Now()

		if logs.isMergeFinished() {
			logs.TAS.EndTime = time.Now()
		}
	}

	return logs
}

// isMergeFinished returns true if the merged result should be considered finished:
// either there was an error, or all phases completed.
func (l *Logs) isMergeFinished() bool {
	return l.TAS.EndError != nil || l.allPhasesFinished()
}

// gatherPhaseLogs collects all phase logs from input into a single ordered map.
func gatherPhaseLogs(input []*phaselogs.PhaseLogs) *phaselogs.PhaseLogs {
	gathered := phaselogs.NewPhaseLogs()

	for _, pl := range input {
		if pl == nil {
			continue
		}

		pl.ForEach(func(phaseKey phase.Phase, phaseValue *phaselogs.PhaseLog) bool {
			if gathered.Exists(phaseKey) {
				panic("internal error: MergePhaseLogs found duplicate keys in inputs")
			}

			gathered.Set(phaseKey, phaseValue)

			return true
		})
	}

	return gathered
}

// allPhasesFinished returns true if every phase in PhaseLogs is finished.
// Uses ForEach (zero-allocation) instead of Pairs() since this is called
// every frame per machine in MergePhaseLogs → isMergeFinished.
func (l *Logs) allPhasesFinished() bool {
	return l.PhaseLogs.ForEach(func(_ phase.Phase, phaseLog *phaselogs.PhaseLog) bool {
		return phaseLog.TimeAndState.Load().IsFinished()
	})
}
