package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeActivatePhaseMachine(configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Logs.SafeGet(phases.Activate),
		fmt.Sprintf("Started activation of %s", machine.Name),
		fmt.Sprintf("Finished activation of %s", machine.Name),
		machine,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {
			binPath := configuration.MetaBuild.OutputPath + "/bin/switch-to-configuration"

			commandWithArgs := []string{*machine.SudoProgram}
			if commandWithArgs[0] == "" {
				commandWithArgs[0] = binPath
			} else {
				commandWithArgs = append(commandWithArgs, binPath)
			}
			commandWithArgs = append(commandWithArgs, "switch")

			// Build a configuration
			err := exc.Exec(false, true,
				func(log *config.CommandLog, err error) error {
					return errors.Wrapf(err, "activation failed for %s", machine.Name)
				},
				nil,
				commandWithArgs...,
			)

			return err
		},
	)
}
