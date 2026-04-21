package commands_workflow

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

type DeployCmd struct {
	flags.WorkflowFlags
}

func (c *DeployCmd) Run() error {
	flags := flags.Flags{WorkflowFlags: c.WorkflowFlags}
	commandPhases := []phase.Phase{phase.Inspect, phase.Build, phase.Bootstrap, phase.Transfer, phase.Secrets, phase.Activate}

	return runWorkflow(flags, commandPhases)
}
