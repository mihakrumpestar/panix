package logs

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// Logs holds the shared log state embedded in Fleet, Flake, Configuration, Machine.
// It satisfies the LogNode interface, eliminating per-struct boilerplate.
type Logs struct {
	PhaseLogs             *phase.PhaseLogs `json:"phase_logs"`
	DurationAndErrorCache DurationAndError `json:"duration_and_error"`
}

type DurationAndError struct {
	Duration time.Duration `json:"duration"`
	Error    error         `json:"error,omitempty"`
}

func (l *Logs) MustGetOrCreateLog(p phases.Phase) *phase.PhaseLog {
	return l.PhaseLogs.GetOrCreate(p)
}

func (l *Logs) GetLog(p phases.Phase) *phase.PhaseLog {
	return l.PhaseLogs.Get(p)
}

func (l *Logs) GetLogs() *phase.PhaseLogs {
	return l.PhaseLogs
}

func (l *Logs) GetCachedDurationAndError() DurationAndError {
	return l.DurationAndErrorCache
}

func (l *Logs) CalculateDurationAndError(workflowPhases []phases.Phase) DurationAndError {
	if len(l.children) == 0 {
		l.calculateFromPhases()
	} else {
		l.calculateFromChildren(workflowPhases)
	}

	return l.DurationAndErrorCache
}

func (l *Logs) calculateFromChildren(workflowPhases []phases.Phase) {
	l.DurationAndErrorCache = DurationAndError{}

	for _, child := range l.children {
		childDae := child.CalculateDurationAndError(workflowPhases)
		if childDae.Duration > l.DurationAndErrorCache.Duration {
			l.DurationAndErrorCache = childDae
		}
	}
}

func (l *Logs) calculateFromPhases() {
	l.DurationAndErrorCache = DurationAndError{}

	for _, phaseLogPair := range l.PhaseLogs.All() {
		tas := phaseLogPair.Value.TimeAndState()

		duration, err := tas.DurationOrElapsedTime()
		if err != nil {
			l.DurationAndErrorCache.Error = err

			break
		}

		l.DurationAndErrorCache.Duration += duration

		err = tas.GetEndError()
		if err != nil {
			l.DurationAndErrorCache.Error = err

			break
		}
	}
}

func (l *Logs) propagateLog(p phases.Phase, log *phase.PhaseLog) {
	for _, child := range l.children {
		child.GetLogs().SetIfNotExists(p, log)
	}
}

func (l *Logs) ClearLogs() {
	if l.PhaseLogs != nil {
		l.PhaseLogs.Clear()
	}

	l.DurationAndErrorCache = DurationAndError{}
}

// LogNode is implemented by Fleet, Flake, Configuration, and Machine via embedded LogEntity.
type LogNode interface {
	MustGetOrCreateLog(p phases.Phase) *phase.PhaseLog
	GetLog(p phases.Phase) *phase.PhaseLog
	GetLogs() *phase.PhaseLogs
	GetCachedDurationAndError() DurationAndError
	CalculateDurationAndError(workflowPhases []phases.Phase) DurationAndError
}
