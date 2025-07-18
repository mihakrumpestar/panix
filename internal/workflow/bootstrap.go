package workflow

import (
	"fmt"
	"net/url"

	"github.com/mihakrumpestar/panix/internal/config"
)

func (w *Workflow) executeBootstrapPhaseMachine(flakeName, configName string, machineName *url.URL, machine *config.Machine) error {
	// TODO: Implement nixos-anywhere bootstrap
	if w.state.Conf.Global.Verbose {
		fmt.Printf("Bootstrap for machine %s/%s/%s: TODO - implement nixos-anywhere\n", flakeName, configName, machineName)
	}

	return nil
}
