package workflow

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func (w *Workflow) executeTransferPhaseMachine(fleetLeaf *fleet.FleetLeaf) error {
	if w.conf.Flags.LocalMachineHostname == fleetLeaf.Machine.SSH.Hostname {
		return nil
	}

	return w.Phase(phase.Transfer, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error {
			systemClosure := fleetLeaf.Configuration.MetaBuild.SystemClosure

			err := executeTransferPhaseMachineWrapper(exc, fleetLeaf.Machine, []string{systemClosure}, true)
			if err != nil {
				return err
			}

			return nil
		})
}

func executeTransferPhaseMachineWrapper(
	exc *executioner.Executioner,
	machine *machine.Machine,
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
