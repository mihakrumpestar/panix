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

// MergePhaseLogsInto merges phase logs from multiple inputs into dst in-place.
// dst.PhaseLogs is cleared and repopulated (reuses map capacity, zero allocation
// when entry count is stable). The merge result is written to dst.TAS as a
// temporary snapshot; the caller should sync it into a persisted TAS via SyncFrom.
func MergePhaseLogsInto(dst *Logs, phasesInOrder []phase.Phase, input ...*phaselogs.PhaseLogs) {
	dst.PhaseLogs.Clear()

	// Reset TAS to a fresh state for this merge snapshot.
	dst.TAS.Reset()

	anyStarted := false
	acc := newIntervalAccumulator()

	// Add phases in order (stopping after the first errored or still-running phase).
	// Phases from higher scopes (e.g. configuration-level "build") may already be
	// finished by another machine's goroutine via OnceAsync, but if the previous
	// machine-level phase hasn't completed yet the machine hasn't actually reached
	// this phase, so we must not include it in the merged view.
	for _, phase := range phasesInOrder {
		// Look up phase directly in inputs; avoids building an intermediate map.
		phaseLog := findPhaseLog(phase, input...)
		if phaseLog == nil {
			continue
		}

		dst.PhaseLogs.Set(phase, phaseLog)

		tas := phaseLog.TimeAndState
		// Call DurationOrElapsedTime so running phases get their DurationCache updated.
		_, _ = tas.DurationOrElapsedTime()
		tasLoaded := tas.Load()

		dst.TAS.EndError = tasLoaded.EndError

		if tasLoaded.HasStarted() {
			anyStarted = true

			acc.add(tasLoaded.StartTime, tasLoaded.EndTime)
		}

		if tasLoaded.EndError != nil {
			break
		}

		if !tasLoaded.IsFinished() {
			break
		}
	}

	dst.TAS.DurationCache = acc.total()

	// Set state markers on TAS so HasStarted/IsFinished return correct values.
	// These are approximate timestamps; the real times live on per-phase TAS.
	if anyStarted {
		dst.TAS.StartTime = time.Now()

		if dst.TAS.EndError != nil || dst.allPhasesFinished() {
			dst.TAS.EndTime = time.Now()
		}
	}
}

// findPhaseLog looks up a phase in the given PhaseLogs inputs, returning the
// first match. Returns nil if not found in any input.
func findPhaseLog(phase phase.Phase, inputs ...*phaselogs.PhaseLogs) *phaselogs.PhaseLog {
	for _, pl := range inputs {
		if pl == nil {
			continue
		}

		log, ok := pl.Get(phase)
		if ok {
			return log
		}
	}

	return nil
}

// allPhasesFinished returns true if every phase in PhaseLogs is finished.
// Uses ForEach (zero-allocation) since this is called every frame per machine.
func (l *Logs) allPhasesFinished() bool {
	return l.PhaseLogs.ForEach(func(_ phase.Phase, phaseLog *phaselogs.PhaseLog) bool {
		return phaseLog.TimeAndState.Load().IsFinished()
	})
}

// intervalAccumulator merges overlapping time intervals on the fly, producing
// the union duration. Phases must be added in chronological order.
//
// Phases from different scopes can run concurrently (e.g. configuration-level
// "build" runs via OnceAsync while a machine-level "bootstrap" is still in
// progress on another goroutine). Simply summing each phase's duration would
// double-count the overlapping time window. The accumulator merges overlapping
// intervals, counting each moment exactly once. This also correctly excludes
// gaps between retry attempts, since a retried phase's StartTime is reset.
type intervalAccumulator struct {
	duration time.Duration
	curStart time.Time
	curEnd   time.Time
}

func newIntervalAccumulator() *intervalAccumulator {
	return &intervalAccumulator{}
}

func (a *intervalAccumulator) add(start, end time.Time) {
	if end.IsZero() {
		end = time.Now()
	}

	switch {
	case a.curStart.IsZero():
		// First interval.
		a.curStart = start
		a.curEnd = end
	case start.Before(a.curEnd):
		// Overlapping; extend current interval.
		if end.After(a.curEnd) {
			a.curEnd = end
		}
	default:
		// Non-overlapping; finalize current interval and start a new one.
		a.duration += a.curEnd.Sub(a.curStart)
		a.curStart = start
		a.curEnd = end
	}
}

func (a *intervalAccumulator) total() time.Duration {
	if !a.curStart.IsZero() {
		a.duration += a.curEnd.Sub(a.curStart)
	}

	return a.duration
}
