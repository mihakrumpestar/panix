package workflow

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/hook"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/pkg/errors"
)

type Workflow struct {
	ctx    context.Context
	cancel context.CancelFunc
	state  *WorkflowState
	hook   *hook.Hook
	phases []workflow_definition.WorkflowPhase
}

type WorkflowState struct {
	Conf *config.Config
	Pool pond.Pool
}

func NewWorkflow(ctx context.Context, phases []workflow_definition.WorkflowPhase) (*Workflow, error) {
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

func (w *Workflow) Phases() []workflow_definition.WorkflowPhase {
	return w.phases
}

func (w *Workflow) Start() error {
	if slices.Contains(w.phases, workflow_definition.PhaseStatus) {
		err := w.ExecuteStatusPhase()
		if err != nil {
			return err
		}
	}

	if slices.Contains(w.phases, workflow_definition.PhaseBuild) {
		err := w.ExecuteBuildPhase()
		if err != nil {
			return err
		}
	}

	return nil
}

// Helpers

func validatePhaseConstraints(phases, slipPhases []workflow_definition.WorkflowPhase) ([]workflow_definition.WorkflowPhase, error) {
	phases = slices.DeleteFunc(phases, func(phase workflow_definition.WorkflowPhase) bool {
		return slices.Contains(slipPhases, phase)
	})

	if len(phases) == 0 {
		return nil, fmt.Errorf("all phases skipped")
	}

	phase := phases[0]
	validFirstPhases := []workflow_definition.WorkflowPhase{workflow_definition.PhaseStatus, workflow_definition.PhaseBuild, workflow_definition.PhaseSecrets}
	if !slices.Contains(validFirstPhases, phase) {
		return nil, fmt.Errorf("phase %s is can't be the first phase, allowed are %s", phase, validFirstPhases)
	}

	return phases, nil
}

// poolForEach is a generic helper function to iterate over an ordered map in parallel
func poolForEach[K cmp.Ordered, V config.FlakeOrConfigurationOrMachine](w *Workflow, iterable config.SortableMap[K, V], skipDisabled bool, function func(value V) error) error {
	groupPool := w.state.Pool.NewGroup()

	errCacher := make([]error, 0)

	for _, value := range iterable.Range(skipDisabled) {
		groupPool.SubmitErr(func() error {
			err := function(value)
			errCacher = append(errCacher, err)

			if !w.state.Conf.Global.RequireAllSuccess {
				return nil
			}

			return err
		})
	}

	err := groupPool.Wait()
	if err != nil {
		return errors.Wrapf(err, "poolForEach: failed because of 'RequireAllSuccess'")
	}

	if !slices.Contains(errCacher, nil) {
		return fmt.Errorf("poolForEach: failed because of all tasks errored")
	}

	return nil
}

func (w *WorkflowState) ExpandFlakeConfigurationMachine(skipDisabled bool, function func(i int,
	flake *config.Flake, configuration *config.Configuration, machine *config.Machine)) {
	i := 0

	for _, flake := range w.Conf.Flakes.Range(skipDisabled) {
		for _, configuration := range flake.Configurations.Range(skipDisabled) {
			for _, machine := range configuration.Machines.Range(skipDisabled) {
				function(i, flake, configuration, machine)
				i++
			}
		}
	}
}
