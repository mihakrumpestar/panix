package workflow

import (
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeMachineActivate(flakeName, configName, machineName string, machine *config.Machine) error {
	// Get the build output path from metadata
	buildOutputPath := w.cfg.Flakes[flakeName].Configurations[configName].GetBuildOutputPath()

	return w.activateMachine(flakeName, configName, machineName, machine, buildOutputPath)
}

func (w *WorkflowExecutor) executeActivatePhase(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	var allMachines []machineInfo

	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				allMachines = append(allMachines, machineInfo{
					flakeName:   flakeName,
					configName:  configName,
					machineName: machineName,
					machine:     machine,
				})
			}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var successCount int
	var failedMachines []machineInfo

	for _, mi := range allMachines {
		wg.Add(1)
		go func(mi machineInfo) {
			defer wg.Done()
			err := w.executeMachineActivate(mi.flakeName, mi.configName, mi.machineName, mi.machine)
			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Errorf("machine %s: %w", mi.machineName, err))
				failedMachines = append(failedMachines, mi)
			} else {
				successCount++
			}
			mu.Unlock()
		}(mi)
	}

	wg.Wait()

	if len(errors) > 0 {
		if w.cfg.Global.RequireAllSuccess {
			fmt.Printf("Activation failed, rolling back all machines...\n")
			w.rollbackMachines(allMachines)
			return nil, fmt.Errorf("activation failed: %v", errors)
		}
		if successCount == 0 {
			return nil, fmt.Errorf("activation: all machines failed")
		}
		for _, err := range errors {
			fmt.Printf("Warning: %v\n", err)
		}
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return &ExecutionResult{}, nil
}

// This function is called by executeMachineActivate for individual machine activation
func (w *WorkflowExecutor) activateMachine(flakeName, configName, machineName string, machine *config.Machine, buildOutputPath string) error {
	if buildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path, cannot activate", flakeName, configName, machineName)
	}

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, machine)
	if err != nil {
		return err
	}

	output, err := exc.Exec("sudo", fmt.Sprintf("%s/bin/switch-to-configuration", buildOutputPath), "switch")
	if err != nil {
		return fmt.Errorf("activation failed for %s/%s/%s: %w\nOutput: %s", flakeName, configName, machineName, err, output.Stderr.String())
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Activated %s/%s/%s successfully\n", flakeName, configName, machineName)
	}

	return nil
}
