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
	"github.com/pkg/errors"
)

type Workflow struct {
	ctx    context.Context
	cancel context.CancelFunc
	state  *WorkflowState
	hook   *hook.Hook
	phases []phases.Phase
}

type WorkflowState struct {
	Conf *config.Config
	Pool pond.Pool
}

func NewWorkflow(ctx context.Context, phases []phases.Phase) (*Workflow, error) {
	conf := ctx.Value(config.ContextConfigKey).(*config.Config)

	if conf == nil {
		return nil, fmt.Errorf("%s key is nil/empty in workflow context", config.ContextConfigKey)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, conf.Global.Timeout)

	pool := pond.NewPool(conf.Global.Concurrency, pond.WithContext(ctxWithTimeout))

	hookI := hook.NewHook()

	// Remove skipped phases
	phases, err := validatePhaseConstraints(phases, conf.Global.SkipPhases)
	if err != nil {
		cancel()
		return nil, err
	}

	return &Workflow{
		ctx:    ctxWithTimeout,
		cancel: cancel,
		state: &WorkflowState{
			Conf: conf,
			Pool: pool,
		},
		hook:   hookI,
		phases: phases,
	}, nil
}

func (w *Workflow) Ctx() context.Context {
	return w.ctx
}

func (w *Workflow) State() *WorkflowState {
	return w.state
}

func (w *Workflow) GetChannel() <-chan uint64 {
	return w.hook.GetChannel()
}

func (w *Workflow) Cancel() context.CancelFunc {
	return w.cancel
}

func (w *Workflow) Phases() []phases.Phase {
	return w.phases
}

func (w *Workflow) Start() error {
	if slices.Contains(w.phases, phases.Status) {
		err := w.ExecuteStatusPhase()
		if err != nil {
			return err
		}
	}

	if slices.Contains(w.phases, phases.Build) {
		err := w.ExecuteBuildPhase()
		if err != nil {
			return err
		}

		return nil
	}

	if slices.Contains(w.phases, phases.Secrets) {
		err := w.ExecuteSecretsPhase()
		if err != nil {
			return err
		}

		return nil
	}

	return nil
}

func (w *Workflow) Phase(phaseLog *config.PhaseLog, startDebugMsg, endDebugMsg string, machine *config.Machine, phaseCode func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error) (err error) {
	phaseLog.TimeAndState.StartTimer()
	defer func() {
		phaseLog.TimeAndState.EndTimerWithError(err)
	}()

	if w.state.Conf.Global.Verbose {
		phaseLog.AddMessageOnly(startDebugMsg)
	}

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, machine, phaseLog, w.hook.OnUpdateHook)
	err = phaseCode(exc, phaseLog)

	if w.state.Conf.Global.Verbose {
		phaseLog.AddMessageOnly(endDebugMsg)
	}

	return err
}

// Helpers

func validatePhaseConstraints(phasesI, skipPhases []phases.Phase) ([]phases.Phase, error) {
	phasesI = slices.DeleteFunc(phasesI, func(phase phases.Phase) bool {
		return slices.Contains(skipPhases, phase)
	})

	if len(phasesI) == 0 {
		return nil, fmt.Errorf("all phases skipped")
	}

	phase := phasesI[0]
	validFirstPhases := []phases.Phase{phases.Status, phases.Build, phases.Secrets}
	if !slices.Contains(validFirstPhases, phase) {
		return nil, fmt.Errorf("phase %s is can't be the first phase, allowed are %s", phase, validFirstPhases)
	}

	return phasesI, nil
}

// poolChildren is a generic helper function to iterate over children in parallel
func poolChildren[V config.FCM](w *Workflow, parent config.FCM, skipDisabled bool, function func(value V) error) (err error) {
	groupPool := w.state.Pool.NewGroup()

	errCacher := make([]error, 0)

	defer func() {
		if err != nil {
			parent.Disable(config.DefaultColorScheme().Error.Render(fmt.Sprintf("(disabled) %v", err)))
		}
	}()

	for _, value := range parent.Children(skipDisabled) {
		// Type assertion to ensure the value is of type V
		typedValue, ok := value.(V)
		if !ok {
			panic(fmt.Sprintf("type assertion failed: expected %T, got %T", typedValue, value))
		}

		groupPool.SubmitErr(func() error {
			err := function(typedValue)

			if err != nil {
				typedValue.Disable(config.DefaultColorScheme().Error.Render(fmt.Sprintf("(disabled) %v", err)))
			}

			errCacher = append(errCacher, err)
			if !w.state.Conf.Global.RequireAllSuccess {
				return nil
			}

			return err
		})
	}

	err = groupPool.Wait()
	if err != nil {
		err = errors.Wrapf(err, "'RequireAllSuccess' condition not met")
		return
	}

	if !slices.Contains(errCacher, nil) {
		err = fmt.Errorf("all sub-tasks failed")
		return
	}

	return
}

func (w *WorkflowState) ExpandFlakeConfigurationMachine(skipDisabled bool, function func(i int,
	flake *config.Flake, configuration *config.Configuration, machine *config.Machine)) {
	i := 0

	for _, flake := range w.Conf.Root.Flakes.SortedMap(false, skipDisabled) {
		for _, configuration := range flake.Configurations.SortedMap(false, skipDisabled) {
			for _, machine := range configuration.Machines.SortedMap(false, skipDisabled) {
				function(i, flake, configuration, machine)
				i++
			}
		}
	}
}
