package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/pkg/hook"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/pkg/retry"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	WorkerPoolMaxConcurrency    = 1000
	SSHReachabilityCheckTimeout = 5 * time.Second
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

	runner, err := newRunner(workflow)
	if err != nil {
		cancel()

		return nil, err
	}

	workflow.runner = runner

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

func (w *Workflow) NewTaskWithRetry(p phase.Phase, logs *logs.Logs, f func() error) error {
	for {
		err := f()
		if err != nil {
			if w.conf.Flags.RequireAllSuccess {
				w.cancel()

				return err
			}

			if w.conf.Flags.ExitOnComplete {
				return err
			}

			err = w.state.Retry.Wait(w.ctx)
			if err != nil {
				return errors.Wrap(err, "retry wait failed")
			}

			phaseLog, ok := logs.PhaseLogs.Get(p)
			if !ok {
				continue
			}

			phaseLog.Clear()
		} else {
			return nil
		}
	}
}

func (w *Workflow) Phase(p phase.Phase, fleetLeaf *fleet.FleetLeaf, phaseCode func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error) error {
	var (
		logs  *logs.Logs
		xpath xpath.Xpath
		err   error
	)

	switch p.GetPhaseScope() {
	case phase.ScopeMachine:
		logs = fleetLeaf.Machine.Logs
		xpath = fleetLeaf.Machine.Xpath
	case phase.ScopeConfiguration:
		logs = fleetLeaf.Configuration.Logs
		xpath = fleetLeaf.Configuration.Xpath
	case phase.ScopeFlake:
		logs = fleetLeaf.Flake.Logs
		xpath = fleetLeaf.Flake.Xpath
	case phase.ScopeFleet:
		logs = fleetLeaf.Flake.Logs
		xpath = fleetLeaf.Flake.Xpath
	default:
		return errors.New("invalid phase scope")
	}

	phaseLog := logs.PhaseLogs.GetOrCreate(p)

	phaseLog.TimeAndState.StartTimer()

	sublog := log.With().
		Str("phase", string(p)).
		Str("xpath", xpath.String()).
		Logger()

	sublog.Info().Str("event", "phase_start").Msgf("Started %s of %s", p, xpath.String())

	dryRun := w.conf.Flags.DryRun || (w.conf.Flags.DryRunWithInspect && p != phase.Inspect)
	exc := executioner.NewExecutioner(w.ctx, w.conf.Flags.Timeout, dryRun, xpath, fleetLeaf.Machine, p, phaseLog, w.updateHook.Signal)
	err = phaseCode(exc, phaseLog)

	phaseLog.TimeAndState.EndTimerWithError(err)

	duration, durationErr := phaseLog.TimeAndState.Load().Duration()
	if durationErr != nil {
		return durationErr
	}

	logger.ResultEvent(sublog,
		fmt.Sprintf("Finished %s of %s", p, xpath.String()),
		err,
		func(event *zerolog.Event) {
			event.Str("event", "phase_end").Dur("duration", duration)
		})

	return err
}

// StartWorkflow orchestrates the execution of all phases.
func (w *Workflow) StartWorkflow() error {
	subPool := w.state.Pool.NewGroup()

	for _, fleetLeaf := range w.conf.Fleet.AllMachines() {
		subPool.SubmitErr(func() error {
			// Create a shared phaseRunner for this machine
			phaseRunnerInstance := &phaseRunner{
				r:         w.runner,
				fleetLeaf: fleetLeaf,
			}

			// Execute each phase in order
			for _, phase := range w.conf.Phases {
				err := phaseRunnerInstance.run(phase)
				if err != nil {
					return err
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

	return nil
}

func (w *Workflow) MachineCount() int {
	count := 0

	for range w.conf.Fleet.AllMachines() {
		count++
	}

	return count
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
