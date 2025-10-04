package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeTransferPhaseMachine(configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Logs.SafeGet(phases.Transfer),
		fmt.Sprintf("Started transfer of %s", machine.Name),
		fmt.Sprintf("Finished transfer of %s", machine.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {

			systemClosure := configuration.MetaBuild.SystemClosure
			diskoScript := configuration.MetaBuild.DiskoScript
			toTransfer := []string{systemClosure}

			if diskoScript != "" {
				toTransfer = append(toTransfer, diskoScript)
			}
			commandWithArgs := append([]string{"nix", "copy", "--to", machine.Name}, toTransfer...)

			err := exc.Exec(true, true,
				func(l *config.CommandLog, err error) error {
					return errors.Wrap(err, "nix copy failed")
				},
				nil,
				commandWithArgs...,
			)
			if err != nil {
				return err
			}

			if w.state.Conf.Global.Verbose {
				phaseLog.AddMessageOnly(fmt.Sprintf("Transferred %s to %s\n", toTransfer, machine.Name))
			}

			return nil
		})
}
