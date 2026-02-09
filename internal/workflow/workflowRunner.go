package workflow

import (
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/once_async"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// phaseRunner handles the execution of a single phase
// It automatically manages once-per-config and once-per-flake semantics
type phaseRunner struct {
	w       *Workflow
	flake   *config.Flake
	config  *config.Configuration
	machine *config.Machine
}

// onceRegistry holds the OnceAsync instances for once-per-scope semantics
// This is shared across all phaseRunners via a package-level variable
var onceRegistry sync.Map // map[string]*once_async.OnceAsync

// getOrCreateOnceAsync returns a OnceAsync for the given xpath
// This ensures that phases with ScopeConfig or ScopeFlake only run once
func getOrCreateOnceAsync(xpath string) *once_async.OnceAsync {
	if once, ok := onceRegistry.Load(xpath); ok {
		return once.(*once_async.OnceAsync)
	}

	once := once_async.NewOnceAsync()
	actual, loaded := onceRegistry.LoadOrStore(xpath, once)
	if loaded {
		return actual.(*once_async.OnceAsync)
	}
	return once
}

// run executes a phase with automatic once-per-scope semantics
func (pr *phaseRunner) run(phase phases.Phase) error {
	// Check bootstrap status - skip bootstrap phase if already bootstrapped
	if phase == phases.Bootstrap && pr.machine.MetaStatus.Bootstrapped.Load() {
		return nil
	}

	// If in Bootstrap.Only mode and machine is already bootstrapped, skip all phases
	if pr.w.state.Conf.Flags.Bootstrap.Only && pr.machine.MetaStatus.Bootstrapped.Load() {
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
		return pr.w.executePhase(phase, pr.flake, pr.config, pr.machine)
	}

	// If this phase should only run once per scope, use OnceAsync
	if phases.ShouldRunOnce(phase) {
		once := getOrCreateOnceAsync(xpath.String())
		return once.Do(func() error {
			return pr.w.NewTaskWithRetry(phase, xpath, execFn)
		})
	}

	// Otherwise, run directly
	return pr.w.NewTaskWithRetry(phase, xpath, execFn)
}

// executePhase executes a phase by dispatching to the appropriate handler
func (w *Workflow) executePhase(phase phases.Phase, flake *config.Flake, config *config.Configuration, machine *config.Machine) error {
	switch phase {
	case phases.Inspect:
		return w.executeInspectPhaseMachine(machine)
	case phases.Build:
		return w.executeBuildPhaseConfiguration(flake, config)
	case phases.Bootstrap:
		return w.executeBootstrapPhaseMachine(flake, config, machine)
	case phases.Transfer:
		return w.executeTransferPhaseMachine(machine)
	case phases.Secrets:
		return w.executeSecretsPhaseMachine(machine)
	case phases.Activate:
		return w.executeActivatePhaseMachine(machine)
	default:
		return nil
	}
}
