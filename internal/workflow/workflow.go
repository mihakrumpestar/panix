package workflow

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

type WorkflowExecutor struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	phases []workflow_definition.WorkflowPhase
}

func NewWorkflowExecutor(ctx context.Context, cfg *config.Config, phases []workflow_definition.WorkflowPhase) (*WorkflowExecutor, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.Global.Timeout)

	filteredPhases := make([]workflow_definition.WorkflowPhase, 0)
	for _, phase := range phases {
		if !slices.Contains(cfg.Global.SkipPhases, phase) {
			filteredPhases = append(filteredPhases, phase)
		}
	}

	if len(filteredPhases) == 0 {
		cancel()
		return nil, fmt.Errorf("No phases left after filtering")
	}

	return &WorkflowExecutor{
		ctx:    ctx,
		cancel: cancel,
		phases: phases,
		cfg:    cfg,
	}, nil
}

// Execute runs the workflow phases
func (w *WorkflowExecutor) Execute() error {
	defer w.cancel()
	return w.executePhase(w.phases)
}

func (w *WorkflowExecutor) executePhase(currentPhases []workflow_definition.WorkflowPhase) error {
	if len(currentPhases) == 0 {
		return nil
	}

	currentPhase, nextPhases := currentPhases[0], currentPhases[1:]
	if w.cfg.Global.Verbose {
		fmt.Printf("starting phase: %s\n", currentPhase)
	}

	switch currentPhase {
	case workflow_definition.PhaseBuild:
		return w.executeBuild(nextPhases)
	case workflow_definition.PhaseBootstrap:
		return w.executeBootstrap(nextPhases)
	case workflow_definition.PhaseTransfer:
		return w.executeTransfer(nextPhases)
	case workflow_definition.PhaseSecrets:
		return w.executeSecrets(nextPhases)
	case workflow_definition.PhaseActivate:
		return w.executeActivatePhase(nextPhases)
	case workflow_definition.PhaseStatus:
		return w.executeStatus(nextPhases)
	default:
		return fmt.Errorf("unknown phase: %s", currentPhase)
	}
}

// Parallel execution helpers

type parallelExecution struct {
	wg      *sync.WaitGroup
	mu      *sync.Mutex
	errors  []error
	verbose bool
	ctx     context.Context
}

func newParallelExecution(ctx context.Context, verbose bool) *parallelExecution {
	return &parallelExecution{
		wg:      &sync.WaitGroup{},
		mu:      &sync.Mutex{},
		errors:  []error{},
		verbose: verbose,
		ctx:     ctx,
	}
}

func (pe *parallelExecution) addTask(task func() error) {
	pe.wg.Add(1)
	go func() {
		defer pe.wg.Done()

		// Check if context is already cancelled
		select {
		case <-pe.ctx.Done():
			return
		default:
		}

		if err := task(); err != nil {
			pe.mu.Lock()
			pe.errors = append(pe.errors, err)
			pe.mu.Unlock()
		}
	}()
}

func (pe *parallelExecution) waitAndHandleErrors(phaseName string, requireAllSuccess bool, cancel context.CancelFunc) error {
	if requireAllSuccess {
		// In strict mode, wait for first error or all completion
		done := make(chan struct{})
		go func() {
			pe.wg.Wait()
			close(done)
		}()

		for {
			select {
			case <-done:
				// All tasks completed
				if len(pe.errors) > 0 {
					return fmt.Errorf("%s phase failed: %v", phaseName, pe.errors)
				}
				return nil
			case <-pe.ctx.Done():
				// Context cancelled, return immediately
				return fmt.Errorf("%s phase cancelled", phaseName)
			default:
				// Check for errors
				pe.mu.Lock()
				hasErrors := len(pe.errors) > 0
				pe.mu.Unlock()

				if hasErrors {
					cancel()     // Cancel the entire workflow
					pe.wg.Wait() // Wait for cancellation to propagate
					pe.mu.Lock()
					err := fmt.Errorf("%s phase failed: %v", phaseName, pe.errors)
					pe.mu.Unlock()
					return err
				}
			}
		}
	} else {
		// In permissive mode, wait for all tasks to complete
		pe.wg.Wait()

		if len(pe.errors) > 0 {
			for _, err := range pe.errors {
				fmt.Printf("Warning: %v\n", err)
			}
		}
	}
	return nil
}

// executeParallelMachines executes a function across all machines in parallel
func (w *WorkflowExecutor) executeParallelMachines(
	phaseName string,
	executeFunc func(flakeName, configName, machineName string, machine *config.Machine) error,
	nextPhases []workflow_definition.WorkflowPhase,
) error {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing %s phase\n", phaseName)
	}

	pe := newParallelExecution(w.ctx, w.cfg.Global.Verbose)

	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				f, c, m, mach := flakeName, configName, machineName, machine
				pe.addTask(func() error {
					return executeFunc(f, c, m, mach)
				})
			}
		}
	}

	if err := pe.waitAndHandleErrors(phaseName, w.cfg.Global.RequireAllSuccess, w.cancel); err != nil {
		return err
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}

// executeParallelConfigs executes a function across all configurations in parallel
func (w *WorkflowExecutor) executeParallelConfigs(
	phaseName string,
	executeFunc func(flakeName, configName string, flake *config.Flake) error,
	nextPhases []workflow_definition.WorkflowPhase,
) error {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing %s phase\n", phaseName)
	}

	pe := newParallelExecution(w.ctx, w.cfg.Global.Verbose)

	for flakeName, flake := range w.cfg.Flakes {
		for configName := range flake.Configurations {
			f, c, fl := flakeName, configName, flake
			pe.addTask(func() error {
				return executeFunc(f, c, fl)
			})
		}
	}

	if err := pe.waitAndHandleErrors(phaseName, w.cfg.Global.RequireAllSuccess, w.cancel); err != nil {
		return err
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}
