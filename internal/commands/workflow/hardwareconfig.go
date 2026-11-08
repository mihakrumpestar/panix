package commands_workflow

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/hardwareconfig"
)

// HardwareConfigCmd regenerates the hardware config of machines that already
// exist, without building, transferring, or activating anything.
type HardwareConfigCmd struct {
	flags.WorkflowFlags
}

func (c *HardwareConfigCmd) Run() error {
	flags := flags.Flags{WorkflowFlags: c.WorkflowFlags}
	commandPhases := []phase.Phase{phase.Inspect, phase.HardwareConfig}

	return runWorkflow(c, flags, commandPhases, hardwareconfig.Prepare)
}
