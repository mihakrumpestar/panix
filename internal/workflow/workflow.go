package workflow

import (
	"context"
	"fmt"
	"slices"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/hook"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/mihakrumpestar/panix/internal/workflow/shared_deps"
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

func NewWorkflow(ctx context.Context, phasesI []phases.Phase) (*Workflow, error) {
	conf := ctx.Value(config.ContextConfigKey).(*config.Config)

	if conf == nil {
		return nil, fmt.Errorf("%s key is nil/empty in workflow context", config.ContextConfigKey)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, conf.Global.Timeout)

	phases, err := phases.NewPhaseStates(phasesI, conf.Global.SkipPhases)
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
			Pool:   pond.NewPool(conf.Global.Concurrency, pond.WithContext(ctxWithTimeout)),
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

func (w *Workflow) NewTaskWithRetry(phase phases.Phase, identifier string, f func() error) error {
	w.state.Phases.AddKeyToValue(phase, identifier)
	defer w.state.Phases.RemoveKeyFromValue(phase, identifier)

	for {
		err := f()
		if err != nil {
			if w.state.Conf.Global.RequireAllSuccess {
				w.cancel()
				return err
			}

			<-w.state.Retry.retry // Pauses execution
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

			flakeIdentifier := flake.Name
			preFlakeHook := shared_deps.NewOnceAsync()
			postFlakeHook := shared_deps.NewOnceAsync()

			for _, configuration := range flake.Configurations.SortedMap() {
				configurationIdentifier := fmt.Sprintf("%s/%s", flake.Name, configuration.Name)
				build := shared_deps.NewOnceAsync()

				for _, machine := range configuration.Machines.SortedMap() {
					machineIdentifier := fmt.Sprintf("%s/%s/%s", flake.Name, configuration.Name, machine.Name)

					flakePool.SubmitErr(func() error {

						// Status
						if slices.Contains(w.state.Phases.Keys(), phases.Status) {
							err := w.NewTaskWithRetry(phases.Status, machineIdentifier, func() error {
								return w.executeStatusPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Pre flake hook
						if slices.Contains(w.state.Phases.Keys(), phases.PreFlakeHook) {
							err := preFlakeHook.Do(func() error {
								return w.NewTaskWithRetry(phases.PreFlakeHook, flakeIdentifier, func() error {
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
								return w.NewTaskWithRetry(phases.Build, configurationIdentifier, func() error {
									return w.executeBuildPhaseConfiguration(flake, configuration)
								})
							})
							if err != nil {
								return err
							}
						}

						// Transfer
						if slices.Contains(w.state.Phases.Keys(), phases.Transfer) {
							err := w.NewTaskWithRetry(phases.Transfer, machineIdentifier, func() error {
								return w.executeTransferPhaseMachine(configuration, machine)
							})
							if err != nil {
								return err
							}
						}

						// Bootstrap
						if slices.Contains(w.state.Phases.Keys(), phases.Bootstrap) {
							err := w.NewTaskWithRetry(phases.Bootstrap, machineIdentifier, func() error {
								return w.executeBootstrapPhaseMachine(configuration, machine)
							})
							if err != nil {
								return err
							}
						}

						// Secrets
						if slices.Contains(w.state.Phases.Keys(), phases.Secrets) {
							err := w.NewTaskWithRetry(phases.Secrets, machineIdentifier, func() error {
								return w.executeSecretsPhaseMachine(machine)
							})
							if err != nil {
								return err
							}
						}

						// Activate
						if slices.Contains(w.state.Phases.Keys(), phases.Activate) {
							err := w.NewTaskWithRetry(phases.Activate, machineIdentifier, func() error {
								return w.executeActivatePhaseMachine(configuration, machine)
							})
							if err != nil {
								return err
							}
						}

						w.state.Phases.AddKeyToValue(phases.Done, machineIdentifier)

						return nil
					})
				}
			}

			err := flakePool.Wait()

			// Post flake hook
			if slices.Contains(w.state.Phases.Keys(), phases.PostFlakeHook) {
				errH := postFlakeHook.Do(func() error {
					return w.NewTaskWithRetry(phases.PostFlakeHook, flakeIdentifier, func() error {
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

func (w *Workflow) Phase(phaseLog *config.PhaseLog, startDebugMsg, endDebugMsg string, machine *config.Machine, phaseCode func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error) (err error) {
	phaseLog.TimeAndState.StartTimer()
	defer func() {
		phaseLog.TimeAndState.EndTimerWithError(err)
	}()

	if w.state.Conf.Global.Verbose {
		phaseLog.AddMessageOnly("VERBOSE " + startDebugMsg)
	}

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, machine, phaseLog, w.updateHook.OnUpdateHook)
	err = phaseCode(exc, phaseLog)

	if w.state.Conf.Global.Verbose {
		phaseLog.AddMessageOnly("VERBOSE " + endDebugMsg)
	}

	return err
}

// Helpers

func (w *WorkflowState) ExpandFlakeConfigurationMachine(function func(i int, flake *config.Flake, configuration *config.Configuration, machine *config.Machine)) {
	i := 0

	for _, flake := range w.Conf.Root.Flakes.SortedMap() {
		for _, configuration := range flake.Configurations.SortedMap() {
			for _, machine := range configuration.Machines.SortedMap() {
				function(i, flake, configuration, machine)
				i++
			}
		}
	}
}
