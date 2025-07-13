package workflow

import (
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_status"
)

func (w *WorkflowExecutor) executeStatus(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing status phase\n")
	}

	// Collect all machine statuses
	var allStatuses []*workflow_status.MachineStatus
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	// Execute status checks for all machines in parallel
	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				wg.Add(1)
				go func(fname, cname, mname string, m *config.Machine) {
					defer wg.Done()

					status := w.statusMachineStatus(fname, cname, mname, m)

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

	// Print the combined status table ONCE after all machines are checked
	workflow_status.PrintStatusTable(allStatuses)
	fmt.Println("Status check completed")

	// Handle errors
	if len(errors) > 0 && w.cfg.Global.RequireAllSuccess {
		return w.metadata, fmt.Errorf("status check failed: %v", errors)
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return w.metadata, nil
}

// This function is called by executeStatusBranching for individual machine status checks
func (w *WorkflowExecutor) statusMachineStatus(flakeName, configName, machineName string, machine *config.Machine) *workflow_status.MachineStatus {
	status := workflow_status.CheckHost(w.ctx, w.cfg.Global, machineName, machine, workflow_status.CheckFull)
	return status
}
