package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (w *Workflow) executeBootstrapPhaseMachine(flake *config.Flake, configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Attributes, phases.Bootstrap, machine,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {

			// DiskoScript
			installables := []string{fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.diskoScript", flake.Url, configuration.Name)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, phaseLog, flake, configuration, installables)
			if err != nil {
				return err
			}

			diskoScript := parsedOutput[0].Outputs.Out

			err = executeTransferPhaseMachineWrapper(exc, phaseLog, machine, []string{diskoScript}, false)
			if err != nil {
				return err
			}

			commandWithArgs := []string{diskoScript}

			err = exc.Exec(
				"disko",
				"diskoScript failed",
				commandWithArgs,
			)
			if err != nil {
				return err
			}

			if machine.PostBootstrapHook == "" {
				return nil
			}

			commandWithArgs = []string{machine.PostBootstrapHook}
			err = exc.Exec("post bootstrap hook", "post bootstrap hook failed", commandWithArgs)
			if err != nil {
				return err
			}

			return nil
		},
	)
}
