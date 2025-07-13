package workflow

import (
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeMachineTransfer(flakeName, configName, machineName string, machine *config.Machine) error {
	// Get the build output path from metadata
	buildOutputPath := w.cfg.Flakes[flakeName].Configurations[configName].GetBuildOutputPath()

	return w.transferToMachine(flakeName, configName, machineName, machine, buildOutputPath)
}

func (w *WorkflowExecutor) executeTransfer(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing transfer phase\n")
	}

	// Execute transfer for all machines in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				wg.Add(1)
				go func(f, c, m string, machine *config.Machine) {
					defer wg.Done()
					err := w.executeMachineTransfer(f, c, m, machine)
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
			return w.metadata, fmt.Errorf("transfer phase failed: %v", errors)
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

// This function is called by executeMachineTransfer for individual machine transfers
func (w *WorkflowExecutor) transferToMachine(flakeName, configName, machineName string, machine *config.Machine, buildOutputPath string) error {
	if buildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path", flakeName, configName, machineName)
	}

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, machine)
	if err != nil {
		return err
	}

	sshInfo := fmt.Sprintf("ssh://%s", machine.Ssh.Alias)
	output, err := exc.Exec("nix", "copy", "--to", sshInfo, buildOutputPath)
	if err != nil {
		return fmt.Errorf("nix copy failed: %w\n%s", err, output.Stderr.String())
	}

	if err != nil {
		return fmt.Errorf("transfer failed for %s/%s/%s: %w", flakeName, configName, machineName, err)
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Transferred %s to %s/%s/%s\n", buildOutputPath, flakeName, configName, machineName)
	}

	return nil
}
