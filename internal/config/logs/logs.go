package logs

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/jsonerror"
	"github.com/pkg/errors"
)

// LogState represents the aggregate state of a set of phase logs.
type LogState uint8

const (
	LogStateIdle     LogState = iota // no phase has started
	LogStateRunning                  // at least one phase is in progress
	LogStateFinished                 // all phases finished
)

// Logs holds the log state embedded in Fleet, Flake, Configuration, Machine.
type Logs struct {
	PhaseLogs             *phaselogs.PhaseLogs `yaml:"-" json:"phase_logs"`
	DurationAndErrorCache DurationAndError     `yaml:"-" json:"duration_and_error"`
	version               uint64
}

type DurationAndError struct {
	Duration time.Duration        `yaml:"-" json:"duration"`
	Error    *jsonerror.JSONError `yaml:"-" json:"error,omitempty"`
	State    LogState             `yaml:"-" json:"state"`
}

func New() *Logs {
	return &Logs{
		PhaseLogs: phaselogs.NewPhaseLogs(),
	}
}

// Version returns the current version counter. Increments on state transitions.
func (l *Logs) Version() uint64 {
	return l.version
}

// SetDurationAndError replaces the cached duration/error and bumps version on
// state or error change. Duration-only changes are throttled by the spinner
// generation and do not bump the version.
func (l *Logs) SetDurationAndError(dae DurationAndError) {
	if l.DurationAndErrorCache.State != dae.State || l.DurationAndErrorCache.Error != dae.Error {
		l.version++
	}

	l.DurationAndErrorCache = dae
}

// SetDuration replaces the cached duration. No version bump — duration
// display is throttled by the spinner generation.
func (l *Logs) SetDuration(d time.Duration) {
	l.DurationAndErrorCache.Duration = d
}

func (l *Logs) RecalculateDurationAndError() error {
	old := l.DurationAndErrorCache
	durationAndError := DurationAndError{}

	for _, phaseLogPair := range l.PhaseLogs.Pairs() {
		tas := phaseLogPair.Value.TimeAndState

		duration, err := tas.DurationOrElapsedTime()
		if err != nil {
			return errors.Wrap(err, "failed to get duration or elapsed time")
		}

		durationAndError.Duration += duration

		tasLoaded := tas.Load()

		endError := tasLoaded.EndError
		if endError != nil {
			durationAndError.Error = endError

			break
		}

		if !tasLoaded.IsFinished() {
			break
		}
	}

	// Derive aggregate state from phase logs.
	durationAndError.State = l.computeState()

	l.DurationAndErrorCache = durationAndError

	if durationAndError.State != old.State || durationAndError.Error != old.Error {
		l.version++
	}

	return nil
}

func (l *Logs) Clear() {
	l.PhaseLogs.Clear()
	l.DurationAndErrorCache = DurationAndError{}
	l.version++
}

func (l *Logs) PostUnmarshalInit() {
	if l == nil {
		return
	}

	if l.PhaseLogs == nil {
		l.PhaseLogs = phaselogs.NewPhaseLogs()
	}
}

// computeState returns the aggregate state of all phase logs.
func (l *Logs) computeState() LogState {
	state := LogStateIdle

	for _, phaseLogPair := range l.PhaseLogs.Pairs() {
		tasLoaded := phaseLogPair.Value.TimeAndState.Load()

		if tasLoaded.IsFinished() {
			state = LogStateFinished
		} else if tasLoaded.HasStarted() {
			return LogStateRunning
		}
	}

	return state
}

// Helpers

// MergePhaseLogs merges multiple PhaseLogs.
func MergePhaseLogs(phasesInOrder []phase.Phase, input ...*phaselogs.PhaseLogs) *Logs {
	logs := New()

	gathered := phaselogs.NewPhaseLogs()

	// Gather all together
	for _, pl := range input {
		if pl == nil {
			continue
		}

		pl.ForEach(func(phaseKey phase.Phase, phaseValue *phaselogs.PhaseLog) bool {
			ok := gathered.Exists(phaseKey)
			if ok {
				panic("internal error: MergePhaseLogs found duplicate keys in inputs")
			}

			gathered.Set(phaseKey, phaseValue)

			return true
		})
	}

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

		// Sum up all valid durations and set last error
		logs.DurationAndErrorCache = DurationAndError{
			Duration: logs.DurationAndErrorCache.Duration + phaseDOET,
			Error:    tasLoaded.EndError,
		}

		if tasLoaded.EndError != nil {
			logs.DurationAndErrorCache.State = LogStateFinished

			break
		}

		if !tasLoaded.IsFinished() {
			if tasLoaded.HasStarted() {
				logs.DurationAndErrorCache.State = LogStateRunning
			}

			break
		}

		logs.DurationAndErrorCache.State = LogStateFinished
	}

	return logs
}
