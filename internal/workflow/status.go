package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_status"
)

func (w *WorkflowExecutor) executeStatus(nextPhases []workflow_definition.WorkflowPhase) error {
	return w.executeParallelMachines("status", w.checkMachineStatusFull, nextPhases)
}

func (w *WorkflowExecutor) checkMachineStatusFull(flakeName, configName, machineName string, machine *config.Machine) error {
	status := workflow_status.CheckHost(w.ctx, w.cfg.Global, machineName, machine, workflow_status.CheckFull)
	if status.Error != nil {
		return status.Error
	}
	return nil
}
