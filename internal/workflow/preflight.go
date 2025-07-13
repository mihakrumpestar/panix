package workflow

import (
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_status"
)

func (w *WorkflowExecutor) executePreflight(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing preflight phase\n")
	}

	// Preflight runs separately but branches internally to contact all machines
	// This is handled by the executeBranching function with no branching flags
	// since preflight needs to contact all machines but doesn't cascade branching

	// Collect all machine statuses
	var allStatuses []*workflow_status.MachineStatus
	var mu sync.Mutex

	// Execute the preflight phase with branching, collecting results
	err := w.executePreflightBranching(allStatuses, &mu)
	if err != nil {
		return w.metadata, err
	}

	// Print the combined status table ONCE after all machines are checked
	workflow_status.PrintStatusTable(allStatuses)
	fmt.Println("Preflight check completed")

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return w.metadata, nil
}

// executePreflightBranching handles the parallel execution of preflight checks
func (w *WorkflowExecutor) executePreflightBranching(allStatuses []*workflow_status.MachineStatus, mu *sync.Mutex) error {
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				wg.Add(1)
				go func(fname, cname, mname string, m *config.Machine) {
					defer wg.Done()

					status := w.preflightMachineStatus(fname, cname, mname, m)

					mu.Lock()
					allStatuses = append(allStatuses, status)
					mu.Unlock()

					if status.Error != nil {
						errorMu.Lock()
						errors = append(errors, status.Error)
						errorMu.Unlock()
					}
				}(flakeName, configName, machineName, machine)
			}
		}
	}

	wg.Wait()

	if len(errors) > 0 && w.cfg.Global.RequireAllSuccess {
		return fmt.Errorf("preflight failed: %v", errors)
	}

	return nil
}

// This function is called by executePreflightBranching for individual machine preflight checks
func (w *WorkflowExecutor) preflightMachineStatus(flakeName, configName, machineName string, machine *config.Machine) *workflow_status.MachineStatus {
	status := workflow_status.CheckHost(w.ctx, w.cfg.Global, machineName, machine, workflow_status.CheckMinimal)
	return status
}

// This function is called by executeMachinePreflight for individual machine preflight checks (legacy)
func (w *WorkflowExecutor) preflightMachine(flakeName, configName, machineName string, machine *config.Machine) error {
	status := w.preflightMachineStatus(flakeName, configName, machineName, machine)
	return status.Error
}
