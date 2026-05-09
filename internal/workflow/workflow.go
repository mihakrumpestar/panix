package workflow

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/hook"
	"github.com/mihakrumpestar/panix/pkg/retry"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	WorkerPoolMaxConcurrency    = 1000
	SSHReachabilityCheckTimeout = 2 * time.Second
)

type Workflow struct {
	ctx        context.Context
	cancel     context.CancelFunc
	conf       *config.Config
	state      *WorkflowState
	updateHook *hook.Hook
	runner     *runner
}

type WorkflowState struct {
	Pool  pond.Pool
	Retry *retry.Retry
}

func NewWorkflow(ctx context.Context, conf *config.Config) (*Workflow, error) {
	ctxWithCancel, cancel := context.WithCancel(ctx)

	workflow := &Workflow{
		ctx:    ctxWithCancel,
		cancel: cancel,
		conf:   conf,
		state: &WorkflowState{
			Pool:  pond.NewPool(WorkerPoolMaxConcurrency, pond.WithContext(ctxWithCancel)),
			Retry: retry.NewTaskRetry(),
		},
		updateHook: hook.NewHook(),
	}

	workflow.runner = newRunner(workflow)

	return workflow, nil
}

func (w *Workflow) State() *WorkflowState {
	return w.state
}

func (w *Workflow) WaitForUpdate() <-chan struct{} {
	return w.updateHook.WaitForUpdate()
}

// Cancel cancels the context and waits for it's completion.
func (w *Workflow) Cancel() error {
	w.cancel()
	<-w.ctx.Done() // Wait to fully finish context (this also stops and cancels pool)

	w.updateHook.Close() // If we don't close it, WaitForUpdate will wait beyond the restart

	return errors.Wrap(w.ctx.Err(), "context error")
}

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

func (w *Workflow) Phase(
	phaseI phase.Phase,
	fleetLeaf *fleet.FleetLeaf,
	phaseCode func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error,
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
	if w.runner.groupCtx != nil {
		ctx = w.runner.groupCtx
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
	err = phaseCode(exc, phaseLog)

	phaseLog.TimeAndState.EndTimerWithError(err)
	w.logPhaseResult(sublog, phaseI, xpath, phaseLog, err)

	return err
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
		return nil, xpath.Xpath(""), errors.New("invalid phase scope")
	}
}

// StartWorkflow orchestrates the execution of all phases.
func (w *Workflow) StartWorkflow() error {
	subPool := w.state.Pool.NewGroup()

	w.runner.groupCtx = subPool.Context()

	var failedCount atomic.Int32

	for _, fleetLeaf := range w.conf.Fleet.AllMachines() {
		subPool.SubmitErr(func() error {
			defer func() {
				fleetLeaf.Machine.Bootstrap.SSH.KnownHostsFile.RemoveIfAuto()
			}()

			// Create a shared phaseRunner for this machine
			phaseRunnerInstance := &phaseRunner{
				r:         w.runner,
				fleetLeaf: fleetLeaf,
			}

			// Execute each phase in order
			for _, phase := range w.conf.Phases {
				err := phaseRunnerInstance.run(phase)
				if err != nil {
					if w.conf.Flags.RequireAllSuccess {
						return err
					}

					failedCount.Add(1)

					return nil
				}
			}

			return nil
		})
	}

	err := subPool.Wait()

	w.updateHook.Close()

	if err != nil && !errors.Is(err, context.Canceled) {
		return errors.Wrap(err, "workflow execution failed")
	}

	// Don't report failed machines when the workflow was cancelled (e.g. for restart)
	if w.ctx.Err() == nil {
		n := failedCount.Load()
		if n > 0 {
			return errors.Errorf("workflow completed with %d machine(s) failed", n)
		}
	}

	return nil
}

func (w *Workflow) MachineCount() int {
	count := 0

	for range w.conf.Fleet.AllMachines() {
		count++
	}

	return count
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

// executePhase executes a phase by dispatching to the appropriate handler.
func (w *Workflow) executePhase(p phase.Phase, fleetLeaf *fleet.FleetLeaf) error {
	switch p {
	case phase.Inspect:
		return w.executeInspectPhaseMachine(fleetLeaf)
	case phase.Build:
		return w.executeBuildPhaseConfiguration(fleetLeaf)
	case phase.Bootstrap:
		return w.executeBootstrapPhaseMachine(fleetLeaf)
	case phase.Transfer:
		return w.executeTransferPhaseMachine(fleetLeaf)
	case phase.Secrets:
		return w.executeSecretsPhaseMachine(fleetLeaf)
	case phase.Activate:
		return w.executeActivatePhaseMachine(fleetLeaf)
	case phase.Rollback:
		return w.executeRollbackPhaseMachine(fleetLeaf)
	default:
		return nil
	}
}
