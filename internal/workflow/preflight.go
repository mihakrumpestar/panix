package workflow

import (
	"github.com/mihakrumpestar/panix/internal/clients"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_status"
)

func (w *WorkflowExecutor) executePreflight(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	result := ExecutionResult{
		Machine: machine,
		Phase:   PhasePreflight,
	}

	sshClient, err := clients.New(config.Config{Global: *w.cfg}, []config.MachineConfig{machine})
	if err != nil {
		result.Error = err
		return result
	}

	status, err := workflow_status.CheckHost(sshClient, machine, workflow_status.CheckMinimal)
	if err != nil {
		result.Error = err
		return result
	}

	workflow_status.PrintStatusTable([]*workflow_status.MachineStatus{status})

	result.Output = "Status check completed"

	return result
}
