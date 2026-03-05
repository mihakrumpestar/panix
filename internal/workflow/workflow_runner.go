package workflow

import (
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/once_async"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// runner handles phase execution with once-per-scope semantics
// It is bound to a Workflow instance to avoid global state issues
type runner struct {
	w            *Workflow
	onceRegistry *omap.Omap[string, *once_async.OnceAsync]
}

// phaseRunner handles the execution of a single phase for a specific machine
// It is created per-machine to hold the context for that machine's execution
type phaseRunner struct {
	r       *runner
	flake   *config.Flake
	config  *config.Configuration
	machine *config.Machine
}

func newRunner(w *Workflow) (*runner, error) {
	onceRegistry, err := omap.New[string, *once_async.OnceAsync]()
	if err != nil {
		return nil, err
	}

	return &runner{
		w:            w,
		onceRegistry: onceRegistry,
	}, nil
}

// getOrCreateOnceAsync returns a OnceAsync for the given xpath
// This ensures that phases with ScopeConfig or ScopeFlake only run once
func (r *runner) getOrCreateOnceAsync(xpath string) *once_async.OnceAsync {
	once, ok := r.onceRegistry.Get(xpath)
	if ok {
		return once
	}

	newOnce := once_async.NewOnceAsync()
	existing, ok := r.onceRegistry.Get(xpath)
	if ok {
		return existing
	}

	err := r.onceRegistry.Set(xpath, newOnce)
	if err != nil {
		return newOnce
	}

	return newOnce
}

// run executes a phase with automatic once-per-scope semantics
func (pr *phaseRunner) run(phase phases.Phase) error {
	w := pr.r.w

	// Check bootstrap status - skip bootstrap phase if already bootstrapped
	if phase == phases.Bootstrap && pr.machine.MetaInspect.Bootstrapped.Load() {
		return nil
	}

	// If in Bootstrap.Only mode and machine is already bootstrapped, skip all phases
	if w.conf.Flags.Bootstrap.Only && pr.machine.MetaInspect.Bootstrapped.Load() {
		return nil
	}

	scope := phases.GetPhaseScope(phase)

	// Determine the xpath and execution function based on scope
	var xpath config_attributes.Xpath
	var execFn func() error

	switch scope {
	case phases.ScopeFlake:
		xpath = pr.flake.Xpath
	case phases.ScopeConfig:
		xpath = pr.config.Xpath
	default: // ScopeMachine
		xpath = pr.machine.Xpath
	}

	execFn = func() error {
		return w.executePhase(phase, pr.flake, pr.config, pr.machine)
	}

	// If this phase should only run once per scope, use OnceAsync
	if phases.ShouldRunOnce(phase) {
		once := pr.r.getOrCreateOnceAsync(xpath.String())
		return once.Do(func() error {
			return w.NewTaskWithRetry(phase, xpath, execFn)
		})
	}

	// Otherwise, run directly
	return w.NewTaskWithRetry(phase, xpath, execFn)
}
