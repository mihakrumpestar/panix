package commands_workflow

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/phase"
)

type RollbackCmd struct {
	flags.RollbackFlags
	flags.WorkflowFlags
}

func (c *RollbackCmd) Run() error {
	flags := flags.Flags{RollbackFlags: c.RollbackFlags, WorkflowFlags: c.WorkflowFlags}
	commandPhases := []phase.Phase{phase.Inspect, phase.Rollback}

	return runWorkflow(flags, commandPhases)
}
