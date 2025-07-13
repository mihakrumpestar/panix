package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeRollback(nextPhases []workflow_definition.WorkflowPhase) error {
	return w.executeParallelMachines("rollback", w.executeMachineRollback, nextPhases)
}

func (w *WorkflowExecutor) executeMachineRollback(flakeName, configName, machineName string, machine *config.Machine) error {
	// TODO: Implement secrets deployment
	if w.cfg.Global.Verbose {
		fmt.Printf("Secrets deployment for machine %s/%s/%s: TODO - implement secrets deployment\n", flakeName, configName, machineName)
	}

	return nil
}
