package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/pkg/hook"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/retry"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
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
	Pool        pond.Pool
	Retry       *retry.Retry
	TargetsLogs *logs.TargetsLogs
}

func NewWorkflow(ctx context.Context, conf *config.Config) (*Workflow, error) {
	targetsLogs, err := logs.InitBuildLogs(conf.Fleet, conf.Flags.Logging)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize build logs")
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)

	workflow := &Workflow{
		ctx:    ctxWithCancel,
		cancel: cancel,
		conf:   conf,
		state: &WorkflowState{
			Pool:        pond.NewPool(WorkerPoolMaxConcurrency, pond.WithContext(ctxWithCancel)),
			Retry:       retry.NewTaskRetry(),
			TargetsLogs: targetsLogs,
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

func (w *Workflow) WorkflowPhases() []phases.Phase {
	return w.conf.Phases
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

func (w *Workflow) NewTaskWithRetry(phase phases.Phase, xpath attributes.Xpath, f func() error) error {
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

			w.state.TargetsLogs.MustGet(xpath).PhaseLogs.Get(phase).Clear()
		} else {
			return nil
		}
	}
}

func (w *Workflow) Phase(
	xpath attributes.Xpath,
	phase phases.Phase,
	machine *config.Machine,
	phaseCode func(exc *executioner.Executioner, phaseLog *phase.PhaseLog) error,
) error {
	var err error

	phaseLog := w.state.TargetsLogs.MustGetOrCreateLog(xpath, phase)

	phaseLog.TimeAndState().StartTimer()

	sublog := log.With().
		Str("phase", string(phase)).
		Str("xpath", xpath.String()).
		Logger()

	sublog.Info().Str("event", "phase_start").Msgf("Started %s of %s", phase, xpath)

	dryRun := w.conf.Flags.DryRun || (w.conf.Flags.DryRunWithInspect && phase != phases.Inspect)
	exc := executioner.NewExecutioner(w.ctx, w.conf.Flags.Timeout, dryRun, machine, phaseLog, w.updateHook.Signal)
	err = phaseCode(exc, phaseLog)

	phaseLog.TimeAndState().EndTimerWithError(err)
	duration, _ := phaseLog.TimeAndState().Duration()

	logger.ResultEvent(sublog,
		fmt.Sprintf("Finished %s of %s", phase, xpath),
		err,
		func(event *zerolog.Event) {
			event.Str("event", "phase_end").Dur("duration", duration)
		})

	return err
}

// StartWorkflow orchestrates the execution of all phases.
func (w *Workflow) StartWorkflow() error {
	subPool := w.state.Pool.NewGroup()

	w.FleetTree(func(idx int, machine *config.Machine) {
		subPool.SubmitErr(func() error {
			// Create a shared phaseRunner for this machine
			phaseRunnerInstance := &phaseRunner{
				r:       w.runner,
				flake:   machine.ParentConfiguration.ParentFlake,
				config:  machine.ParentConfiguration,
				machine: machine,
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
	})

	err := subPool.Wait()

	w.updateHook.Close()

	if err != nil && !errors.Is(err, context.Canceled) {
		return errors.Wrap(err, "workflow execution failed")
	}

	return nil
}

func (w *Workflow) MachineCount() int {
	count := 0

	w.FleetTree(func(i int, machine *config.Machine) {
		count++
	})

	return count
}

func (w *Workflow) FleetTree(function func(idx int, machine *config.Machine)) {
	idx := 0

	for _, flakePair := range w.conf.Fleet.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		for _, configPair := range flake.Configurations.Omap.Pairs() {
			configuration := configPair.Value
			for _, machinePair := range configuration.Machines.Omap.Pairs() {
				machine := machinePair.Value
				function(idx, machine)

				idx++
			}
		}
	}
}

// executePhase executes a phase by dispatching to the appropriate handler.
func (w *Workflow) executePhase(phase phases.Phase, flake *config.Flake, config *config.Configuration, machine *config.Machine) error {
	switch phase {
	case phases.Inspect:
		return w.executeInspectPhaseMachine(machine)
	case phases.Build:
		return w.executeBuildPhaseConfiguration(flake, config)
	case phases.Bootstrap:
		return w.executeBootstrapPhaseMachine(flake, config, machine)
	case phases.Transfer:
		return w.executeTransferPhaseMachine(machine)
	case phases.Secrets:
		return w.executeSecretsPhaseMachine(machine)
	case phases.Activate:
		return w.executeActivatePhaseMachine(machine)
	case phases.Rollback:
		return w.executeRollbackPhaseMachine(machine)
	default:
		return nil
	}
}
