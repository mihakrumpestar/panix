package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
)

func (w *WorkflowExecutorForConfigurationAndMachine) executeBootstrapPhaseMachine(flakeName, configName, machineName string, machine *config.Machine) error {
	// TODO: Implement nixos-anywhere bootstrap
	if w.cfg.Verbose {
		fmt.Printf("Bootstrap for machine %s/%s/%s: TODO - implement nixos-anywhere\n", flakeName, configName, machineName)
	}

	return nil
}
