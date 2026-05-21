package commands_workflow

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/phase"
)

type SecretsCmd struct {
	flags.WorkflowFlags
}

func (c *SecretsCmd) Run() error {
	flags := flags.Flags{WorkflowFlags: c.WorkflowFlags}
	commandPhases := []phase.Phase{phase.Inspect, phase.Secrets}

	return runWorkflow(flags, commandPhases)
}
