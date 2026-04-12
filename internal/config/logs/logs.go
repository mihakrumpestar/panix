package logs

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// Logs holds the log state embedded in Fleet, Flake, Configuration, Machine.
type Logs struct {
	PhaseLogs             *phase.PhaseLogs `json:"phase_logs"`
	DurationAndErrorCache DurationAndError `json:"duration_and_error"`
}

type DurationAndError struct {
	Duration time.Duration `json:"duration"`
	Error    error         `json:"error,omitempty"`
}

func New() *Logs {
	return &Logs{PhaseLogs: phase.NewPhaseLogs()}
}

func (l *Logs) RecalculateDurationAndError() error {
	durationAndError := DurationAndError{}

	for _, phaseLogPair := range l.PhaseLogs.Pairs() {
		tas := phaseLogPair.Value.TimeAndState.Load()

		duration, err := tas.DurationOrElapsedTime()
		if err != nil {
			return err
		}

		durationAndError.Duration += duration

		err = tas.EndError
		if err != nil {
			durationAndError.Error = err

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

// Helpers

// MergePhaseLogs merges multiple PhaseLogs.
func MergePhaseLogs(phasesInOrder []phases.Phase, input ...*phase.PhaseLogs) *Logs {
	logs := New()

	gathered := phase.NewPhaseLogs()

	// Gather all together
	for _, pl := range input {
		if pl == nil {
			continue
		}

		for _, pair := range pl.Pairs() {
			if gathered.MustGet(pair.Key) != nil {
				continue
			}

			gathered.Set(pair.Key, pair.Value)
		}
	}

	// Add phases in order (stopping after the first errored phase).
	for _, phase := range phasesInOrder {
		phaseLog, ok := gathered.Get(phase)
		if !ok {
			continue
		}

		logs.PhaseLogs.Set(phase, phaseLog)

		tas := phaseLog.TimeAndState.Load()
		phaseDOET, _ := tas.DurationOrElapsedTime()

		// Sum up all valid durations and set last error
		logs.DurationAndErrorCache = DurationAndError{
			Duration: logs.DurationAndErrorCache.Duration + phaseDOET,
			Error:    tas.EndError,
		}

		if tas.EndError != nil {
			break
		}
	}

	return logs
}
