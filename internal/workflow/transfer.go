package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeTransferPhaseMachine(machine *config.Machine) error {
	return w.Phase(&machine.Attributes, phases.Transfer, machine,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {

			systemClosure := machine.Configuration.MetaBuild.SystemClosure
			diskoScript := machine.Configuration.MetaBuild.DiskoScript

			toTransfer := []string{systemClosure}

			if diskoScript != "" && !machine.MetaStatus.Bootstrapped.Load() && !w.state.Conf.Flags.Bootstrap.DisableDisko {
				toTransfer = append(toTransfer, diskoScript)
			}
			commandWithArgs := append([]string{"nix", "copy", "--to", "ssh://" + machine.Ssh.Hostname}, toTransfer...)

			err := exc.Exec(commandWithArgs,
				executioner.SkipIfLocal(),
				executioner.DisableAutoSshCommand(),
				executioner.Env(machine.Ssh.MaybeSshEnvOpts()),
				executioner.OnFailure(
					func(l *logs.CommandLog, err error) error {
						return errors.Wrapf(err, "nix copy failed for %s", machine.Name)
					}),
			)
			if err != nil {
				return err
			}

			phaseLog.Verbose("Transferred %s to %s\n", toTransfer, machine.Name)

			return nil
		})
}
