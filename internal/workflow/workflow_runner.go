package workflow

import (
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/onceasync"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
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
	r       *runner
	flake   *config.Flake
	config  *config.Configuration
	machine *config.Machine
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
func (r *runner) getOrCreateOnceAsync(xpath string) *onceasync.OnceAsync {
	once, ok := r.onceRegistry.Get(xpath)
	if ok {
		return once
	}

	newOnce := onceasync.NewOnceAsync()

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

// run executes a phase with automatic once-per-scope semantics.
func (pr *phaseRunner) run(phase phases.Phase) error {
	if shouldSkipPhase(phase, pr.machine) {
		return nil
	}

	workflow := pr.r.workflow

	logNode := getLogNodeForScope(phase, pr.flake, pr.config, pr.machine)

	execFn := func() error {
		return workflow.executePhase(phase, pr.flake, pr.config, pr.machine)
	}

	// If this phase should only run once per scope, use OnceAsync
	if phases.ShouldRunOnce(phase) {
		once := pr.r.getOrCreateOnceAsync(logNodeMustGetXpath(logNode))

		err := once.Do(func() error {
			return workflow.NewTaskWithRetry(phase, logNode, execFn)
		})
		if err != nil {
			return errors.Wrap(err, "failed to run once-per-scope phase")
		}

		return nil
	}

	// Otherwise, run directly
	return workflow.NewTaskWithRetry(phase, logNode, execFn)
}

func shouldSkipPhase(phase phases.Phase, machine *config.Machine) bool {
	if phase != phases.Bootstrap {
		return false
	}

	mi := machine.MetaInspect.Load()

	return mi != nil && mi.Bootstrapped && !machine.Bootstrap.ForceBootstrap
}

func getLogNodeForScope(phase phases.Phase, flake *config.Flake, cfg *config.Configuration, machine *config.Machine) config.LogNode {
	switch phases.GetPhaseScope(phase) {
	case phases.ScopeFlake:
		return flake
	case phases.ScopeConfig:
		return cfg
	default:
		return machine
	}
}

func logNodeMustGetXpath(node config.LogNode) string {
	switch n := node.(type) {
	case *config.Fleet:
		return n.Xpath.String()
	case *config.Flake:
		return n.Xpath.String()
	case *config.Configuration:
		return n.Xpath.String()
	case *config.Machine:
		return n.Xpath.String()
	default:
		return ""
	}
}
