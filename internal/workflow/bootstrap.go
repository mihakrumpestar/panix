package workflow

import (
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeBootstrap(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing bootstrap phase\n")
	}

	// Bootstrap machines in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				wg.Add(1)
				go func(f, c, m string, machine *config.Machine) {
					defer wg.Done()
					err := w.executeMachineBootstrap(f, c, m, machine)
					if err != nil {
						mu.Lock()
						errors = append(errors, fmt.Errorf("%s/%s/%s: %w", f, c, m, err))
						mu.Unlock()
					}
				}(flakeName, configName, machineName, machine)
			}
		}
	}

	wg.Wait()

	// Handle errors
	if len(errors) > 0 {
		if w.cfg.Global.RequireAllSuccess {
			return w.metadata, fmt.Errorf("bootstrap phase failed: %v", errors)
		}
		for _, err := range errors {
			fmt.Printf("Warning: %v\n", err)
		}
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return w.metadata, nil
}

func (w *WorkflowExecutor) executeMachineBootstrap(flakeName, configName, machineName string, machine *config.Machine) error {
	// TODO: Implement nixos-anywhere bootstrap
	if w.cfg.Global.Verbose {
		fmt.Printf("Bootstrap for machine %s/%s/%s: TODO - implement nixos-anywhere\n", flakeName, configName, machineName)
	}

	return nil
}
