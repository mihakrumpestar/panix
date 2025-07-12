package workflow

import (
	"fmt"

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

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return &ExecutionResult{}, nil
}

// This function is called by executeMachinePreflight for individual machine preflight checks
func (w *WorkflowExecutor) preflightMachine(flakeName, configName, machineName string, machine config.Machine) error {
	if w.cfg.Global.DryRun {
		fmt.Printf("DRY RUN: Would check preflight of %s/%s/%s\n", flakeName, configName, machineName)
		return nil
	}

	//fmt.Printf("\nsshConfig: %+v\n\n", machine.Ssh)

	status := workflow_status.CheckHost(w.ctx, machineName, machine, workflow_status.CheckMinimal)
	if status.Error != nil {
		return status.Error
	}

	workflow_status.PrintStatusTable([]*workflow_status.MachineStatus{status})

	fmt.Println("Preflight check completed")

	return nil
}
