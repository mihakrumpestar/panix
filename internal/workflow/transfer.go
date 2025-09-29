package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

// This function is called by executeMachineTransfer for individual configurations -> machine transfers
func (w *Workflow) executeTransferPhaseMachine(configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Logs.SafeGet(phases.Transfer),
		fmt.Sprintf("Started transfer of %s", machine.Name),
		fmt.Sprintf("Finished transfer of %s", machine.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {

			buildOutputPath := configuration.MetaBuild.OutputPath

			if buildOutputPath == "" {
				return fmt.Errorf("machine %s has no build output path", machine.Name)
			}

			err := exc.Exec(true, true,
				func(l *config.CommandLog, err error) error {
					return errors.Wrap(err, "nix copy failed")
				},
				nil,
				"nix", "copy", "--to", machine.Name, buildOutputPath)
			if err != nil {
				return err
			}

			if w.state.Conf.Global.Verbose {
				phaseLog.AddMessageOnly(fmt.Sprintf("Transferred %s to %s\n", buildOutputPath, machine.Name))
			}

			return nil
		})
}
