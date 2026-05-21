package commands_workflow

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/phase"
)

type DeployCmd struct {
	flags.WorkflowFlags
}

func (c *DeployCmd) Run() error {
	flags := flags.Flags{WorkflowFlags: c.WorkflowFlags}
	commandPhases := []phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Transfer, phase.Secrets, phase.Activate}

	return runWorkflow(flags, commandPhases)
}
