package workflow

import (
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/onceasync"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

// runner handles phase execution with once-per-scope semantics.
// It is bound to a Workflow instance to avoid global state issues.
type runner struct {
	workflow     *Workflow
	onceRegistry *omap.Omap[string, *onceasync.OnceAsync]
}

// phaseRunner handles the execution of a single phase for a specific machine.
// It is created per-machine to hold the context for that machine's execution.
type phaseRunner struct {
	r         *runner
	fleetLeaf *fleet.FleetLeaf
}

func newRunner(workflow *Workflow) (*runner, error) {
	onceRegistry, err := omap.New[string, *onceasync.OnceAsync]()
	if err != nil {
		return nil, err
	}

	return &runner{
		workflow:     workflow,
		onceRegistry: onceRegistry,
	}, nil
}

// getOrCreateOnceAsync returns a OnceAsync for the given xpath.
// This ensures that phases with ScopeConfig or ScopeFlake only run once.
func (r *runner) getOrCreateOnceAsync(xpath xpath.Xpath) *onceasync.OnceAsync {
	xpathS := xpath.String()

	once, ok := r.onceRegistry.Get(xpathS)
	if ok {
		return once
	}

	newOnce := onceasync.NewOnceAsync()

	existing, ok := r.onceRegistry.Get(xpathS)
	if ok {
		return existing
	}

	err := r.onceRegistry.Set(xpathS, newOnce)
	if err != nil {
		return newOnce
	}

	return newOnce
}

// run executes a phase with automatic once-per-scope semantics.
func (pr *phaseRunner) run(phase phase.Phase) error {
	if shouldSkipPhase(phase, pr.fleetLeaf.Machine) {
		return nil
	}

	workflow := pr.r.workflow

	xpath, logs, err := getXpathAndLogsForScope(phase, pr.fleetLeaf)
	if err != nil {
		return err
	}

	execFn := func() error {
		return workflow.executePhase(phase, pr.fleetLeaf)
	}

	// If this phase should only run once per scope, use OnceAsync
	if phase.ShouldRunOnce() {
		once := pr.r.getOrCreateOnceAsync(xpath)

		err := once.Do(func() error {
			return workflow.NewTaskWithRetry(phase, logs, execFn)
		})
		if err != nil {
			return errors.Wrap(err, "failed to run once-per-scope phase")
		}

		return nil
	}

	// Otherwise, run directly
	return workflow.NewTaskWithRetry(phase, logs, execFn)
}

func shouldSkipPhase(p phase.Phase, machine *machine.Machine) bool {
	if p != phase.Bootstrap {
		return false
	}

	mi := machine.MetaInspect.Load()

	return mi != nil && mi.Bootstrapped && !machine.Bootstrap.ForceBootstrap
}

func getXpathAndLogsForScope(p phase.Phase, fleetLeaf *fleet.FleetLeaf) (xpath.Xpath, *logs.Logs, error) {
	switch p.GetPhaseScope() {
	case phase.ScopeFlake:
		return fleetLeaf.Flake.Xpath, fleetLeaf.Flake.Logs, nil
	case phase.ScopeConfiguration:
		return fleetLeaf.Configuration.Xpath, fleetLeaf.Configuration.Logs, nil
	case phase.ScopeMachine:
		return fleetLeaf.Machine.Xpath, fleetLeaf.Machine.Logs, nil
	default:
		return xpath.New(), nil, errors.New("getLogsForScope invalid scope")
	}
}
