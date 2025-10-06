package workflow

import (
	"context"
	"slices"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/hook"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/once_async"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type Workflow struct {
	ctx        context.Context
	cancel     context.CancelFunc
	state      *WorkflowState
	updateHook *hook.Hook
}

type WorkflowState struct {
	Conf   *config.Config
	Phases *phases.PhaseStates
	Pool   pond.Pool
	Retry  *TaskRetry
}

type TaskRetry struct {
	retry chan uint64
}

func NewTaskRetry() *TaskRetry {
	return &TaskRetry{
		retry: make(chan uint64),
	}
}

func NewWorkflow(ctx context.Context, conf *config.Config, phasesI []phases.Phase) (*Workflow, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, conf.Flags.Timeout)

	phases, err := phases.NewPhaseStates(phasesI, conf.Flags.SkipPhases)
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
			Retry:  NewTaskRetry(),
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

func (w *Workflow) NewTaskWithRetry(phase phases.Phase, attr *config_attributes.Attributes, f func() error) error {
	phaseTaskStatus := w.state.Phases.Value(phase)

	for {
		phaseTaskStatus.Running.Add(attr.Xpath)
		err := f()
		phaseTaskStatus.Running.Rem(attr.Xpath)

		if err != nil {
			phaseTaskStatus.Failed.Add(attr.Xpath)

			if w.state.Conf.Flags.RequireAllSuccess {
				w.cancel()
				return err
			}

			<-w.state.Retry.retry // Pauses execution

			// Task is being retried, so it is not failed and logs are cleared
			phaseTaskStatus.Failed.Rem(attr.Xpath)
			attr.Logs.SafeGet(phase).Clear()
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

			preFlakeHook := once_async.NewOnceAsync()
			postFlakeHook := once_async.NewOnceAsync()

			for _, configuration := range flake.Configurations.SortedMap() {
				build := once_async.NewOnceAsync()

				for _, machine := range configuration.Machines.SortedMap() {

					flakePool.SubmitErr(func() error {

						// Status
						if slices.Contains(w.state.Phases.Keys(), phases.Status) {
							err := w.NewTaskWithRetry(phases.Status, &machine.Attributes, func() error {
								return w.executeStatusPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Pre flake hook
						if slices.Contains(w.state.Phases.Keys(), phases.PreFlakeHook) {
							err := preFlakeHook.Do(func() error {
								return w.NewTaskWithRetry(phases.PreFlakeHook, &flake.Attributes, func() error {
									return w.executePreFlakeHookPhaseFlake(flake)
								})
							})
							if err != nil {
								return err
							}
						}

						// Build
						if slices.Contains(w.state.Phases.Keys(), phases.Build) {
							err := build.Do(func() error {
								return w.NewTaskWithRetry(phases.Build, &configuration.Attributes, func() error {
									return w.executeBuildPhaseConfiguration(flake, configuration)
								})
							})
							if err != nil {
								return err
							}
						}

						// Transfer
						if slices.Contains(w.state.Phases.Keys(), phases.Transfer) {
							err := w.NewTaskWithRetry(phases.Transfer, &machine.Attributes, func() error {
								return w.executeTransferPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Bootstrap
						if slices.Contains(w.state.Phases.Keys(), phases.Bootstrap) {
							err := w.NewTaskWithRetry(phases.Bootstrap, &machine.Attributes, func() error {
								return w.executeBootstrapPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Secrets
						if slices.Contains(w.state.Phases.Keys(), phases.Secrets) {
							err := w.NewTaskWithRetry(phases.Secrets, &machine.Attributes, func() error {
								return w.executeSecretsPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Activate
						if slices.Contains(w.state.Phases.Keys(), phases.Activate) {
							err := w.NewTaskWithRetry(phases.Activate, &machine.Attributes, func() error {
								return w.executeActivatePhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						w.state.Phases.Value(phases.Done).Done.Add(machine.Xpath)

						return nil
					})
				}
			}

			err := flakePool.Wait()

			// Post flake hook
			if slices.Contains(w.state.Phases.Keys(), phases.PostFlakeHook) {
				errH := postFlakeHook.Do(func() error {
					return w.NewTaskWithRetry(phases.PostFlakeHook, &flake.Attributes, func() error {
						return w.executePostFlakeHookPhaseFlake(flake)
					})
				})
				if errH != nil && err != nil {
					err = errors.Wrap(err, errH.Error())
				} else if errH != nil {
					err = errH
				}
			}

			return err
		})
	}

	return subPool.Wait()
}

// Helpers

func (w *Workflow) Phase(attr *config_attributes.Attributes, phase phases.Phase, machine *config.Machine, phaseCode func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error) (err error) {
	phaseLog := attr.Logs.SafeGet(phase)

	phaseLog.TimeAndState.StartTimer()
	defer func() {
		phaseLog.TimeAndState.EndTimerWithError(err)
	}()

	phaseLog.Verbose("Started %s of %s", phaseLog.Phase(), attr.Xpath)

	exc := executioner.NewExecutioner(w.ctx, w.state.Conf.Flags, machine, phaseLog, w.updateHook.OnUpdateHook)
	err = phaseCode(exc, phaseLog)

	phaseLog.Verbose("Finished %s of %s", phaseLog.Phase(), attr.Xpath)

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
