package workflow

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/hook"
	"github.com/pkg/errors"
)

type Workflow struct {
	ctx    context.Context
	cancel context.CancelFunc
	state  *WorkflowState
	hook   *hook.Hook
}

type WorkflowState struct {
	Conf  *config.Config
	Pool  pond.Pool
	Error error
}

func NewWorkflow(ctx context.Context) (*Workflow, error) {
	conf := ctx.Value(config.ContextConfigKey).(*config.Config)

	if conf == nil {
		return nil, fmt.Errorf("%s key is nil/empty in workflow context", config.ContextConfigKey)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, conf.Global.Timeout)

	pool := pond.NewPool(conf.Global.Concurrency, pond.WithContext(ctxWithTimeout))

	hookI := hook.NewHook()

	return &Workflow{
		ctx:    ctxWithTimeout,
		cancel: cancel,
		state: &WorkflowState{
			Conf: conf,
			Pool: pool,
		},
		hook: hookI,
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

// Helpers

func (w *Workflow) forEachFlakeConfiguration(function func(groupPool pond.TaskGroup, flakeName, configurationName string, configuration *config.Configuration) error) error {
	groupPool := w.state.Pool.NewGroup()

	errCacher := make([]error, 0)

	for flakeName, flake := range w.state.Conf.Flakes.AllFromFront() {
		for configurationName, configuration := range flake.Configurations.AllFromFront() {
			if w.state.Conf.Global.RequireAllSuccess { // This will make groupPool.Wait() exit on first error
				groupPool.SubmitErr(func() error {
					err := function(groupPool, flakeName, configurationName, configuration)
					errCacher = append(errCacher, err)
					return err
				})
			} else {
				groupPool.Submit(func() {
					err := function(groupPool, flakeName, configurationName, configuration)
					errCacher = append(errCacher, err)
				})
			}
		}
	}

	err := groupPool.Wait()

	if err != nil {
		return errors.Wrapf(err, "forEachFlakeConfiguration: failed because of 'RequireAllSuccess'")
	}

	if !slices.Contains(errCacher, nil) {
		return fmt.Errorf("forEachFlakeConfiguration: failed because of all machines failed this phase")
	}

	return nil
}

func (w *Workflow) forEachConfigurationMachine(groupPool pond.TaskGroup, flakeName, configurationName string, configuration *config.Configuration, function func(machineName url.URL, machine *config.Machine) error) error {
	// If groupPool != nil means that we are using parent groupPool and we do not block
	if groupPool == nil {
		groupPool = w.state.Pool.NewGroup()
	}

	// Here we don't care if all tasks fail since we have multiple configurations
	for machineName, machine := range configuration.Machines.AllFromFront() {

		if w.state.Conf.Global.RequireAllSuccess {
			groupPool.SubmitErr(func() error {
				return function(machineName, machine)
			})
		} else {
			groupPool.Submit(func() {
				function(machineName, machine)
			})
		}
	}

	err := groupPool.Wait()
	if err != nil {
		return errors.Wrapf(err, "forEachConfigurationMachine: failed because of 'RequireAllSuccess'")
	}

	return nil
}

func (w *WorkflowState) ExpandFlakeConfigurationMachine(function func(i int, flakeName, configurationName string, configuration *config.Configuration, machineName url.URL, machine *config.Machine)) {
	i := 0

	for flakeName, flake := range w.Conf.Flakes.AllFromFront() {
		for configurationName, configuration := range flake.Configurations.AllFromFront() {
			for machineName, machine := range configuration.Machines.AllFromFront() {
				function(i, flakeName, configurationName, configuration, machineName, machine)
				i++
			}
		}
	}
}
