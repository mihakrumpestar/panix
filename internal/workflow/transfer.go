package workflow

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func (w *Workflow) executeTransferPhaseMachine(machine *config.Machine) error {
	return w.Phase(machine, phases.Transfer, machine,
		func(exc *executioner.Executioner, phaseLog *phase.PhaseLog) error {
			systemClosure := machine.ParentConfiguration.MetaBuild.SystemClosure

			err := executeTransferPhaseMachineWrapper(exc, machine, []string{systemClosure}, true)
			if err != nil {
				return err
			}

			return nil
		})
}

func executeTransferPhaseMachineWrapper(
	exc *executioner.Executioner,
	machine *config.Machine,
	toTransfer []string,
	transferClosure bool,
) error {
	storeArgs := ""
	mi := machine.MetaInspect.Load()
	if mi != nil && !mi.Bootstrapped && transferClosure {
		storeArgs += "?remote-store=local?root=/mnt"
	}

	activeSSH := machine.GetActiveSSH()
	commandWithArgs := slices.Concat(
		[]string{"nix"},
		nixExperimentalFeatures,
		[]string{"copy", "--to", "ssh://" + activeSSH.Hostname + storeArgs},
		slices.Concat(machine.Nix.ExtraFlags, machine.Nix.CopyFlags),
		toTransfer,
	)

	err := exc.Exec("nix copy",
		"copying closure",
		"closure copy failed",
		commandWithArgs,
		executioner.SkipIfLocal(),
		executioner.DisableAutoSSHCommand(),
		executioner.Env(activeSSH.MaybeSSHEnvOpts()),
	)
	if err != nil {
		return errors.Wrap(err, "transfer failed")
	}

	log.Info().
		Str("machine", machine.Name).
		Strs("transferred", toTransfer).
		Msgf("Transferred %s to %s", toTransfer, machine.Name)

	return nil
}
