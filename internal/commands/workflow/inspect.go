package commands_workflow

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/phase"
)

type InspectCmd struct {
	flags.WorkflowFlags
}

func (c *InspectCmd) Run() error {
	flags := flags.Flags{WorkflowFlags: c.WorkflowFlags}
	commandPhases := []phase.Phase{phase.Inspect}

	return runWorkflow(flags, commandPhases)
}
