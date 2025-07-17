package workflow

import (
	"context"
	"fmt"
	"slices"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/hook"
	"github.com/pkg/errors"
)

type ActivationMetadata struct {
	Error error
}

type WorkflowExecutor struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	meta   *Metadatas
	hook   *hook.Hook
	pool   pond.Pool
}

type Metadatas struct {
	StatusPhaseMeta *StatusPhaseMeta
	Error           error
}

type MetadatasBase struct {
	groupPool pond.TaskGroup
	Error     error
}

type WorkflowExecutorForConfigurationAndMachine struct {
	ctx context.Context
	cfg *config.Global
}

func NewWorkflowExecutor(ctx context.Context, cfg *config.Config) *WorkflowExecutor {
	ctx, cancel := context.WithTimeout(ctx, cfg.Global.Timeout)

	hookI := hook.NewHook()

	return &WorkflowExecutor{
		ctx:    ctx,
		cancel: cancel,
		cfg:    cfg,
		meta:   &Metadatas{},
		hook:   hookI,
		pool:   pond.NewPool(cfg.Global.Concurrency),
	}
}

func (w *WorkflowExecutor) Metadatas() *Metadatas {
	return w.meta
}

func (w *WorkflowExecutor) GetChannel() <-chan uint64 {
	return w.hook.GetChannel()
}

func (w *WorkflowExecutor) Cancel() context.CancelFunc {
	return w.cancel
}

// Helpers

func (w *WorkflowExecutor) forEachFlakeConfiguration(msBase *MetadatasBase, function func(wp *WorkflowExecutorForConfigurationAndMachine, bm *executioner.BaseMeta, configuration *config.Configuration) error) error {
	if msBase == nil {
		panic("msBase is not allowed to be nil")
	}

	if msBase.groupPool == nil {
		msBase.groupPool = w.pool.NewGroupContext(w.ctx)
	}
	groupPool := msBase.groupPool

	errCacher := make([]error, 0)

	for flakeName, flake := range w.cfg.Flakes.AllFromFront() {
		for configurationName, configuration := range flake.Configurations.AllFromFront() {
			wp := &WorkflowExecutorForConfigurationAndMachine{w.ctx, &w.cfg.Global}

			bm := &executioner.BaseMeta{
				MetadataID: executioner.MetadataID{
					FlakeName:         flakeName,
					ConfigurationName: configurationName,
				},
			}

			if w.cfg.Global.RequireAllSuccess {
				groupPool.SubmitErr(func() error {
					err := function(wp, bm, configuration)
					errCacher = append(errCacher, err)
					return err
				})
			} else {
				groupPool.Submit(func() {
					err := function(wp, bm, configuration)
					errCacher = append(errCacher, err)
				})
			}
		}
	}

	err := groupPool.Wait()

	if err != nil {
		msBase.Error = errors.Wrapf(err, "forEachFlakeConfiguration: phase failed because of 'RequireAllSuccess'")
		w.hook.OnUpdateHook()
		return msBase.Error
	}

	if !slices.Contains(errCacher, nil) {
		msBase.Error = fmt.Errorf("forEachFlakeConfiguration: phase failed because of all machines failed this phase")
		w.hook.OnUpdateHook()
		return msBase.Error
	}

	return nil
}

func (w *WorkflowExecutor) forEachConfigurationMachine(configuration *config.Configuration, msBase *MetadatasBase, bm *executioner.BaseMeta, function func(wp *WorkflowExecutorForConfigurationAndMachine, bm *executioner.BaseMeta, machine *config.Machine) error) error {
	if configuration == nil {
		panic("configuration  is not allowed to be nil")
	}

	if msBase == nil {
		panic("msBase  is not allowed to be nil")
	}

	if bm == nil {
		panic("bm is not allowed to be nil")
	}

	// Make copy, otherwise each machine will share BaseMeta that shares same flake configuration

	//if msBase.groupPool == nil {
	//	msBase.groupPool = w.pool.NewGroupContext(w.ctx)
	//}
	//groupPool := msBase.groupPool
	groupPool := w.pool.NewGroupContext(w.ctx)

	errCacher := make([]error, 0)

	for machineName, machine := range configuration.Machines.AllFromFront() {
		wp := &WorkflowExecutorForConfigurationAndMachine{w.ctx, &w.cfg.Global}

		fmt.Println("machineName", machineName)

		bm.MetadataID.MachineName = machineName

		if w.cfg.Global.RequireAllSuccess {
			groupPool.SubmitErr(func() error {
				err := function(wp, bm, machine)
				errCacher = append(errCacher, err)
				return err
			})
		} else {
			groupPool.Submit(func() {
				err := function(wp, bm, machine)
				errCacher = append(errCacher, err)
			})
		}
	}

	err := groupPool.Wait()
	if err != nil {
		msBase.Error = errors.Wrapf(err, "forEachConfigurationMachine: phase failed because of 'RequireAllSuccess'")
		w.hook.OnUpdateHook()
		return msBase.Error
	}

	if !slices.Contains(errCacher, nil) {
		msBase.Error = fmt.Errorf("forEachConfigurationMachine: phase failed because of all machines failed this phase")
		w.hook.OnUpdateHook()
		return msBase.Error
	}

	return nil
}

/*
func forAllConfigurations(conf map[string]*config.Flake, sms []StatusMetadata, function func(flakeName, configurationName string, flake *config.Flake, configuration *config.Configuration)) {
	for flakeName, flake := range conf {
		for configurationName, configuration := range flake.Configurations {

			// Checker that passes only flake configurations that have at least one machine that is reachable
			atLeastOneValid := true
			for _, sm := range sms {
				if sm.FlakeName == flakeName && sm.ConfigurationName == configurationName && sm.Error != nil {
					atLeastOneValid = false
					continue
				}
			}

			if !atLeastOneValid {
				continue
			}

			function(flakeName, configurationName, flake, configuration)
		}
	}
}
*/
