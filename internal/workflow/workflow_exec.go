package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phasehandler"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (w *Workflow) NewTaskWithRetry(phase phase.Phase, logs *logs.Logs, f func() error) error {
	for {
		err := f()
		if err == nil {
			return nil
		}

		if w.conf.Flags.RequireAllSuccess {
			return err
		}

		if w.conf.Flags.ExitOnComplete {
			return err
		}

		err = w.state.Retry.Wait(w.ctx)
		if err != nil {
			return errors.Wrap(err, "retry wait failed")
		}

		phaseLog, ok := logs.PhaseLogs.Get(phase)
		if !ok {
			continue
		}

		phaseLog.Clear()
	}
}

func (w *Workflow) runPhase(
	phaseI phase.Phase,
	fleetLeaf *fleet.FleetLeaf,
	handler phasehandler.Handler,
) error {
	logs, xpath, err := phaseLogsAndXpath(phaseI, fleetLeaf)
	if err != nil {
		return err
	}

	phaseLog := logs.PhaseLogs.GetOrCreate(phaseI)
	phaseLog.TimeAndState.StartTimer()

	sublog := log.With().
		Str("phase", string(phaseI)).
		Str("xpath", xpath.String()).
		Logger()

	sublog.Info().Str("event", "phase_start").Msgf("Started %s of %s", phaseI, xpath.String())

	ctx := w.ctx
	if w.groupCtx != nil {
		ctx = w.groupCtx
	}

	dryRun := w.conf.Flags.DryRun || (w.conf.Flags.DryRunWithInspect && phaseI != phase.Inspect)

	executionerConf := executioner.ExecutionerConf{
		Ctx:          ctx,
		Timeout:      w.conf.Flags.Timeout,
		DryRun:       dryRun,
		Xpath:        xpath,
		Machine:      fleetLeaf.Machine,
		Phase:        phaseI,
		PhaseLog:     phaseLog,
		OnUpdateHook: w.updateHook.Signal,
	}
	exc := executioner.NewExecutioner(executionerConf)
	err = handler.RunPhase(exc, fleetLeaf)

	phaseLog.TimeAndState.EndTimerWithError(err)
	w.logPhaseResult(sublog, phaseI, xpath, phaseLog, err)

	return errors.Wrap(err, "phase execution failed")
}

func (w *Workflow) logPhaseResult(sublog zerolog.Logger, phaseI phase.Phase, xpath xpath.Xpath, phaseLog *phaselogs.PhaseLog, err error) {
	duration, durationErr := phaseLog.TimeAndState.Load().Duration()
	if durationErr != nil {
		sublog.Error().Err(durationErr).Msg("failed to get phase duration")

		return
	}

	logger.ResultEvent(sublog,
		fmt.Sprintf("Finished %s of %s", phaseI, xpath.String()),
		err,
		func(event *zerolog.Event) {
			event.Str("event", "phase_end").Dur("duration", duration)
		})
}

func (w *Workflow) executePhase(phaseI phase.Phase, fleetLeaf *fleet.FleetLeaf) error {
	handler, found := w.handlers[phaseI]
	if !found {
		panic("internal error: no handler registered for phase: " + string(phaseI))
	}

	skipper, ok := handler.(phasehandler.Skipper)
	if ok && skipper.ShouldSkip(fleetLeaf) {
		return nil
	}

	return w.runPhase(phaseI, fleetLeaf, handler)
}

func phaseLogsAndXpath(phaseI phase.Phase, fleetLeaf *fleet.FleetLeaf) (*logs.Logs, xpath.Xpath, error) {
	switch phaseI.GetPhaseScope() {
	case phase.ScopeMachine:
		return fleetLeaf.Machine.Logs, fleetLeaf.Machine.Xpath, nil
	case phase.ScopeConfiguration:
		return fleetLeaf.Configuration.Logs, fleetLeaf.Configuration.Xpath, nil
	case phase.ScopeFlake, phase.ScopeFleet:
		return fleetLeaf.Flake.Logs, fleetLeaf.Flake.Xpath, nil
	default:
		return nil, xpath.Xpath{}, errors.New("invalid phase scope")
	}
}
