package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_status"
)

func (w *WorkflowExecutor) executeStatus(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing status phase\n")
	}

	// Status phase runs separately (fully branched) to check all machines
	// This is handled by the executeBranching function with no branching flags
	// since status needs to check all machines but doesn't cascade branching

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return &ExecutionResult{}, nil
}

// This function is called by executeMachineStatus for individual machine status checks
func (w *WorkflowExecutor) statusMachine(flakeName, configName, machineName string, machine config.Machine) error {
	if w.cfg.Global.DryRun {
		fmt.Printf("DRY RUN: Would check status of %s/%s/%s\n", flakeName, configName, machineName)
		return nil
	}

	status := workflow_status.CheckHost(w.ctx, machineName, machine, workflow_status.CheckFull)
	if status.Error != nil {
		return status.Error
	}

	workflow_status.PrintStatusTable([]*workflow_status.MachineStatus{status})

	fmt.Println("Status check completed")

	return nil
}
