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

			systemClosure := configuration.MetaBuild.SystemClosure

			if !machine.MetaStatus.Bootstrapped.Load() && !w.state.Conf.Global.Bootstrap.DisableAuto {

				err := exc.Exec(false, true,
					func(log *config.CommandLog, err error) error {
						return errors.Wrapf(err, "running nixos-install failed for %s", machine.Name)
					},
					nil,
					"nixos-install", "--no-root-passwd", "--no-channel-copy", "--system", systemClosure,
				)
				if err != nil {
					return err
				}

				err = exc.Exec(false, true,
					func(log *config.CommandLog, err error) error {
						return errors.Wrapf(err, "running reboot failed for %s", machine.Name)
					},
					nil,
					"reboot",
				)
				if err != nil {
					return err
				}

			} else {

				binPath := systemClosure + "/bin/switch-to-configuration"

				commandWithArgs := []string{*machine.SudoProgram}
				if commandWithArgs[0] == "" {
					commandWithArgs[0] = binPath
				} else {
					commandWithArgs = append(commandWithArgs, binPath)
				}
				commandWithArgs = append(commandWithArgs, "switch")

				err := exc.Exec(false, true,
					func(log *config.CommandLog, err error) error {
						return errors.Wrapf(err, "activation failed for %s", machine.Name)
					},
					nil,
					commandWithArgs...,
				)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}
