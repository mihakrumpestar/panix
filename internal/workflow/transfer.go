package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeTransferPhaseMachine(machine *config.Machine) error {
	return w.Phase(&machine.Attributes, phases.Transfer, nil,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {

			systemClosure := machine.Configuration.MetaBuild.SystemClosure
			diskoScript := machine.Configuration.MetaBuild.DiskoScript
			toTransfer := []string{systemClosure}

			if diskoScript != "" {
				toTransfer = append(toTransfer, diskoScript)
			}
			commandWithArgs := append([]string{"nix", "copy", "--to", machine.Name}, toTransfer...)

			err := exc.Exec(true, true,
				func(l *logs.CommandLog, err error) error {
					return errors.Wrap(err, "nix copy failed")
				},
				nil,
				commandWithArgs...,
			)
			if err != nil {
				return err
			}

			phaseLog.Verbose("Transferred %s to %s\n", toTransfer, machine.Name)

			return nil
		})
}
