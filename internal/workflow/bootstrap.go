package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
)

func (w *WorkflowExecutor) executeMachineBootstrap(flakeName, configName, machineName string, machine *config.Machine) error {
	// TODO: Implement nixos-anywhere bootstrap
	if w.cfg.Global.Verbose {
		fmt.Printf("Bootstrap for machine %s/%s/%s: TODO - implement nixos-anywhere\n", flakeName, configName, machineName)
	}

	return nil
}
