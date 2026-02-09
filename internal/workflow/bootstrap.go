package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (w *Workflow) executeBootstrapPhaseMachine(flake *config.Flake, configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Bootstrap, machine,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			installables := []string{fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.diskoScript", flake.URL, configuration.Name)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, phaseLog, flake, configuration, installables)
			if err != nil {
				return err
			}

			diskoScript := parsedOutput[0].Outputs.Out

			err = executeTransferPhaseMachineWrapper(exc, phaseLog, machine, []string{diskoScript}, false)
			if err != nil {
				return err
			}

			err = exc.Exec(
				"disko",
				"partitioning disk",
				"diskoScript failed",
				[]string{diskoScript},
			)
			if err != nil {
				return err
			}

			if machine.PostBootstrapHook == "" {
				return nil
			}

			err = exc.Exec("post bootstrap hook", "running post-bootstrap hook", "post bootstrap hook failed", []string{machine.PostBootstrapHook})
			if err != nil {
				return err
			}

			return nil
		},
	)
}
