package workflow

import (
	"context"
	"slices"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/hook"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/once_async"
	"github.com/mihakrumpestar/panix/internal/pkg/retry"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type Workflow struct {
	ctx        context.Context
	cancel     context.CancelFunc
	state      *WorkflowState
	updateHook *hook.Hook
}

type WorkflowState struct {
	Conf   *config.Config
	Phases []phases.Phase
	Pool   pond.Pool
	Retry  *retry.Retry
}

func NewWorkflow(ctx context.Context, conf *config.Config, phasesI []phases.Phase) (*Workflow, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, conf.Flags.Timeout)

	phases, err := phases.ValidatePhases(phasesI, conf.Flags.SkipPhases)
	if err != nil {
		cancel()
		return nil, err
	}

	return &Workflow{
		ctx:    ctxWithTimeout,
		cancel: cancel,
		state: &WorkflowState{
			Conf:   conf,
			Phases: phases,
			Pool:   pond.NewPool(0, pond.WithContext(ctxWithTimeout)),
			Retry:  retry.NewTaskRetry(),
		},
		updateHook: hook.NewHook(),
	}, nil
}

func (w *Workflow) Ctx() context.Context {
	return w.ctx
}

func (w *Workflow) State() *WorkflowState {
	return w.state
}

func (w *Workflow) WaitForUpdate() <-chan uint64 {
	return w.updateHook.WaitForUpdate()
}

func (w *Workflow) Cancel() context.CancelFunc {
	return w.cancel
}

func (w *Workflow) NewTaskWithRetry(phase phases.Phase, xpath config_attributes.Xpath, f func() error) error {

	for {
		err := f()
		if err != nil {
			if w.state.Conf.Flags.RequireAllSuccess {
				w.cancel()
				return err
			}

			w.state.Retry.Wait()

			// Task is being retried, so logs are cleared
			w.state.Conf.TargetsLogs.Get(xpath).PhaseLogs.Get(phase).Clear()
		} else {
			return nil
		}
	}
}

func (w *Workflow) CreateWorkflow() error {
	subPool := w.state.Pool.NewGroup()

	for _, flake := range w.state.Conf.Root.Flakes.SortedMap() {

		subPool.SubmitErr(func() error {

			flakePool := w.state.Pool.NewGroup()

			for _, configuration := range flake.Configurations.SortedMap() {

				build := once_async.NewOnceAsync()

				for _, machine := range configuration.Machines.SortedMap() {

					flakePool.SubmitErr(func() error {

						// Status
						if slices.Contains(w.state.Phases, phases.Inspect) {
							err := w.NewTaskWithRetry(phases.Inspect, machine.Xpath, func() error {
								return w.executeInspectPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						if w.state.Conf.Flags.Bootstrap.Only && machine.MetaStatus.Bootstrapped.Load() {
							return nil
						}

						// Bootstrap
						if !machine.MetaStatus.Bootstrapped.Load() {

							if !w.state.Conf.Flags.Bootstrap.DisableDisko &&
								slices.Contains(w.state.Phases, phases.Bootstrap) {

								err := w.NewTaskWithRetry(phases.Bootstrap, machine.Xpath, func() error {
									return w.executeBootstrapPhaseMachine(flake, configuration, machine)
								})
								if err != nil {
									return err
								}
							}
						}

						// Build
						if slices.Contains(w.state.Phases, phases.Build) {
							err := build.Do(func() error {
								return w.NewTaskWithRetry(phases.Build, configuration.Xpath, func() error {
									return w.executeBuildPhaseConfiguration(flake, configuration)
								})
							})
							if err != nil {
								return err
							}
						}

						// Transfer
						if slices.Contains(w.state.Phases, phases.Transfer) {
							err := w.NewTaskWithRetry(phases.Transfer, machine.Xpath, func() error {
								return w.executeTransferPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Secrets
						if slices.Contains(w.state.Phases, phases.Secrets) {
							err := w.NewTaskWithRetry(phases.Secrets, machine.Xpath, func() error {
								return w.executeSecretsPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Activate
						if slices.Contains(w.state.Phases, phases.Activate) {
							err := w.NewTaskWithRetry(phases.Activate, machine.Xpath, func() error {
								return w.executeActivatePhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						return nil
					})
				}
			}

			err := flakePool.Wait()

			return err
		})
	}

	return subPool.Wait()
}

// Helpers

func (w *Workflow) Phase(xpath config_attributes.Xpath, phase phases.Phase, machine *config.Machine, phaseCode func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error) (err error) {
	phaseLog := w.state.Conf.TargetsLogs.GetOrCreateLog(xpath, phase)

	phaseLog.TimeAndState().StartTimer()
	defer func() {
		phaseLog.TimeAndState().EndTimerWithError(err)
	}()

	phaseLog.Verbose("Started %s of %s", phaseLog.Phase(), xpath)

	exc := executioner.NewExecutioner(w.ctx, w.state.Conf.Flags, machine, phaseLog, w.updateHook.OnUpdateHook)
	err = phaseCode(exc, phaseLog)

	phaseLog.Verbose("Finished %s of %s", phaseLog.Phase(), xpath)

	return err
}

func (w *WorkflowState) RootTree(function func(i int, machine *config.Machine)) {
	i := 0

	for _, flake := range w.Conf.Root.Flakes.SortedMap() {
		for _, configuration := range flake.Configurations.SortedMap() {
			for _, machine := range configuration.Machines.SortedMap() {
				function(i, machine)
				i++
			}
		}
	}
}
