package commands_workflow

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

type BuildCmd struct {
	flags.WorkflowFlags
}

func (c *BuildCmd) Run() error {
	flags := flags.Flags{WorkflowFlags: c.WorkflowFlags}
	commandPhases := []phase.Phase{phase.Build}

	return runWorkflow(flags, commandPhases)
}
