package workflow

import (
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeMachineSecrets(flakeName, configName, machineName string, machine *config.Machine) error {
	// TODO: Implement secrets deployment
	if w.cfg.Global.Verbose {
		fmt.Printf("Secrets deployment for machine %s/%s/%s: TODO - implement secrets deployment\n", flakeName, configName, machineName)
	}

	return nil
}

func (w *WorkflowExecutor) executeSecrets(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing secrets phase\n")
	}

	// Deploy secrets to machines in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				wg.Add(1)
				go func(f, c, m string, machine *config.Machine) {
					defer wg.Done()
					err := w.executeMachineSecrets(f, c, m, machine)
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
			return w.metadata, fmt.Errorf("secrets phase failed: %v", errors)
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
