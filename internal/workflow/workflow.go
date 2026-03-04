package workflow

import (
	"context"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/hook"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/retry"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/rs/zerolog/log"
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
	targetsLogs, err := logs.InitBuildLogs(conf.Root, conf.Flags.Logging)
	if err != nil {
		return nil, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, conf.Flags.Timeout)

	wf := &Workflow{
		ctx:    ctxWithTimeout,
		cancel: cancel,
		conf:   conf,
		state: &WorkflowState{
			Pool:        pond.NewPool(0, pond.WithContext(ctxWithTimeout)),
			Retry:       retry.NewTaskRetry(),
			TargetsLogs: targetsLogs,
		},
		updateHook: hook.NewHook(),
	}

	// Initialize the runner as an internal attribute of the workflow
	// This ensures onceRegistry is bound to this workflow instance
	wf.runner = &runner{
		w: wf,
	}

	return wf, nil
}

func (w *Workflow) State() *WorkflowState {
	return w.state
}

func (w *Workflow) WaitForUpdate() <-chan struct{} {
	return w.updateHook.WaitForUpdate()
}

// Cancel cancels the context and waits for it's completion
func (w *Workflow) Cancel() error {
	w.cancel()
	<-w.ctx.Done() // Wait to fully finish context

	w.updateHook.Close() // If we don't close it, WaitForUpdate will wait beyond the restart

	return w.ctx.Err()
}

func (w *Workflow) NewTaskWithRetry(phase phases.Phase, xpath config_attributes.Xpath, f func() error) error {
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

			w.state.Retry.Wait()

			w.state.TargetsLogs.Get(xpath).PhaseLogs.Get(phase).Clear()
		} else {
			return nil
		}
	}
}

func (w *Workflow) Phase(xpath config_attributes.Xpath, phase phases.Phase, machine *config.Machine, phaseCode func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error) (err error) {
	phaseLog := w.state.TargetsLogs.GetOrCreateLog(xpath, phase)

	phaseLog.TimeAndState().StartTimer()
	defer func() {
		phaseLog.TimeAndState().EndTimerWithError(err)
	}()

	log.Info().
		Str("phase", string(phaseLog.Phase())).
		Str("xpath", xpath.String()).
		Msgf("Started %s of %s", phaseLog.Phase(), xpath)

	dryRun := w.conf.Flags.DryRun || (w.conf.Flags.DryRunWithInspect && phase != phases.Inspect)
	exc := executioner.NewExecutioner(w.ctx, dryRun, machine, phaseLog, w.updateHook.Signal)
	err = phaseCode(exc, phaseLog)

	log.Info().
		Str("phase", string(phaseLog.Phase())).
		Str("xpath", xpath.String()).
		Msgf("Finished %s of %s", phaseLog.Phase(), xpath)

	return err
}

// CreateWorkflow orchestrates the execution of all phases
func (w *Workflow) CreateWorkflow() error {
	subPool := w.state.Pool.NewGroup()

	w.RootTree(func(i int, machine *config.Machine) {
		subPool.SubmitErr(func() error {
			// Create a shared phaseRunner for this machine
			pr := &phaseRunner{
				r:       w.runner,
				flake:   machine.ParentConfiguration.ParentFlake,
				config:  machine.ParentConfiguration,
				machine: machine,
			}

			// Execute each phase in order
			for _, phase := range w.conf.Phases {
				if err := pr.run(phase); err != nil {
					return err
				}
			}

			return nil
		})
	})

	err := subPool.Wait()
	w.updateHook.Close()
	return err
}

func (w *Workflow) MachineCount() int {
	count := 0
	w.RootTree(func(i int, machine *config.Machine) {
		count = i
	})
	return count
}

func (w *Workflow) RootTree(function func(i int, machine *config.Machine)) {
	i := 0

	for _, flakePair := range w.conf.Root.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		for _, configPair := range flake.Configurations.Omap.Pairs() {
			configuration := configPair.Value
			for _, machinePair := range configuration.Machines.Omap.Pairs() {
				machine := machinePair.Value
				function(i, machine)
				i++
			}
		}
	}
}

// executePhase executes a phase by dispatching to the appropriate handler
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
	default:
		return nil
	}
}
