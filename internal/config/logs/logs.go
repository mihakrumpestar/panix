package logs

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/jsonerror"
	"github.com/pkg/errors"
)

// Logs holds the log state embedded in Fleet, Flake, Configuration, Machine.
type Logs struct {
	PhaseLogs             *phaselogs.PhaseLogs `yaml:"-" json:"phase_logs"`
	DurationAndErrorCache DurationAndError     `yaml:"-" json:"duration_and_error"`
}

type DurationAndError struct {
	Duration time.Duration        `yaml:"-" json:"duration"`
	Error    *jsonerror.JSONError `yaml:"-" json:"error,omitempty"`
}

func New() *Logs {
	return &Logs{
		PhaseLogs: phaselogs.NewPhaseLogs(),
	}
}

func (l *Logs) RecalculateDurationAndError() error {
	durationAndError := DurationAndError{}

	for _, phaseLogPair := range l.PhaseLogs.Pairs() {
		tas := phaseLogPair.Value.TimeAndState

		duration, err := tas.DurationOrElapsedTime()
		if err != nil {
			return errors.Wrap(err, "failed to get duration or elapsed time")
		}

		durationAndError.Duration += duration

		endError := tas.Load().EndError
		if endError != nil {
			durationAndError.Error = endError

			break
		}
	}

	l.DurationAndErrorCache = durationAndError

	return nil
}

func (l *Logs) Clear() {
	l.PhaseLogs.Clear()
	l.DurationAndErrorCache = DurationAndError{}
}

func (l *Logs) PostUnmarshalInit() {
	if l == nil {
		return
	}

	if l.PhaseLogs == nil {
		l.PhaseLogs = phaselogs.NewPhaseLogs()
	}
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

		for _, pair := range pl.Pairs() {
			ok := gathered.Exists(pair.Key)
			if ok {
				panic("internal error: MergePhaseLogs found duplicate keys in inputs")
			}

			gathered.Set(pair.Key, pair.Value)
		}
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
			break
		}

		if !tasLoaded.IsFinished() {
			break
		}
	}

	return logs
}
