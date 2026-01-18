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

			err := executeTransferPhaseMachineWrapper(exc, phaseLog, machine, []string{systemClosure}, true)
			if err != nil {
				return err
			}

			return nil
		})
}

func executeTransferPhaseMachineWrapper(exc *executioner.Executioner, phaseLog *logs.PhaseLog, machine *config.Machine, toTransfer []string, transferClosure bool) error {
	storeArgs := ""
	if !machine.MetaStatus.Bootstrapped.Load() && transferClosure {
		storeArgs += "?remote-store=local?root=/mnt"
	}

	commandWithArgs := append([]string{"nix", "copy", "--to", "ssh://" + machine.Ssh.Hostname + storeArgs}, toTransfer...)

	err := exc.Exec("copy",
		commandWithArgs,
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
}
