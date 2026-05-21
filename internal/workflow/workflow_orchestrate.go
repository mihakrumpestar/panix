package workflow

import (
	"context"
	"sync/atomic"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/onceasync"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
)

// StartWorkflow orchestrates the execution of all phases.
func (w *Workflow) StartWorkflow() error {
	defer func() {
		w.updateHook.Close()
		close(w.done)
	}()

	subPool := w.state.Pool.NewGroup()

	w.groupCtx = subPool.Context()

	var failedCount atomic.Int32

	for _, fleetLeaf := range w.conf.Fleet.AllMachines() {
		subPool.SubmitErr(func() error {
			defer func() {
				fleetLeaf.Machine.Bootstrap.SSH.KnownHostsFile.RemoveIfAuto()
			}()

			for _, phase := range w.conf.Phases {
				err := w.runPhaseForMachine(phase, fleetLeaf)
				if err != nil {
					if w.conf.Flags.RequireAllSuccess {
						return err
					}

					failedCount.Add(1)

					return nil
				}
			}

			return nil
		})
	}

	err := subPool.Wait()

	if err != nil && !errors.Is(err, context.Canceled) {
		return errors.Wrap(err, "workflow execution failed")
	}

	if w.ctx.Err() == nil {
		n := failedCount.Load()
		if n > 0 {
			return errors.Errorf("workflow completed with %d machine(s) failed", n)
		}
	}

	return nil
}

func (w *Workflow) runPhaseForMachine(phase phase.Phase, fleetLeaf *fleet.FleetLeaf) error {
	logs, xpath, err := phaseLogsAndXpath(phase, fleetLeaf)
	if err != nil {
		return err
	}

	execFn := func() error {
		return w.executePhase(phase, fleetLeaf)
	}

	if phase.ShouldRunOnce() {
		once := w.getOrCreateOnceAsync(xpath)

		err = once.Do(func() error {
			return w.NewTaskWithRetry(phase, logs, execFn)
		})
		if err != nil {
			return errors.Wrap(err, "failed to run once-per-scope phase")
		}

		return nil
	}

	return w.NewTaskWithRetry(phase, logs, execFn)
}

func (w *Workflow) getOrCreateOnceAsync(xpath xpath.Xpath) *onceasync.OnceAsync {
	xpathS := xpath.String()

	once, ok := w.onceRegistry.Get(xpathS)
	if ok {
		return once
	}

	newOnce := onceasync.NewOnceAsync()

	existing, ok := w.onceRegistry.Get(xpathS)
	if ok {
		return existing
	}

	w.onceRegistry.Set(xpathS, newOnce)

	return newOnce
}
