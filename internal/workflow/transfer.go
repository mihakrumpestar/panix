package workflow

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func (w *Workflow) executeTransferPhaseMachine(fleetLeaf *fleet.FleetLeaf) error {
	mode := fleetLeaf.Configuration.Nix.BuildMode

	// BuildModeRemote + single machine: transfer is skipped — the closure is already on the target
	if mode == nix.BuildModeRemote {
		machineCount := 0
		for range fleetLeaf.Configuration.Machines.Pairs() {
			machineCount++
		}

		if machineCount <= 1 {
			return nil
		}
	}

	// Local machine: skip
	if fleetLeaf.Machine.SSH.IsLocal() {
		return nil
	}

	return w.Phase(phase.Transfer, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error {
			systemClosure := fleetLeaf.Configuration.MetaBuild.SystemClosure

			err := executeTransferPhaseMachineWrapper(exc, fleetLeaf, []string{systemClosure}, true)
			if err != nil {
				return err
			}

			return nil
		})
}

func executeTransferPhaseMachineWrapper(
	exc *executioner.Executioner,
	fleetLeaf *fleet.FleetLeaf,
	toTransfer []string,
	transferClosure bool,
) error {
	machineI := fleetLeaf.Machine
	configurationI := fleetLeaf.Configuration
	activeSSH := machineI.GetActiveSSH()

	toURL := nixCopyToURL(activeSSH, machineI, transferClosure)
	sshOpts := activeSSH.MaybeNixSSHOpts()

	baseArgs := nixCopyBaseArgs(configurationI, toURL)
	commandWithArgs := slices.Concat(
		baseArgs,
		slices.Concat(configurationI.Nix.ExtraFlags, configurationI.Nix.CopyFlags),
		toTransfer,
	)

	err := exc.Exec("nix copy",
		"copying closure",
		"closure copy failed",
		commandWithArgs,
		executioner.SkipIfLocal(),
		executioner.DisableAutoSSHCommand(),
		executioner.Env(sshOpts),
	)
	if err != nil {
		return errors.Wrap(err, "transfer failed")
	}

	log.Info().
		Str("machine", machineI.Name).
		Strs("transferred", toTransfer).
		Msgf("Transferred %s to %s", toTransfer, machineI.Name)

	return nil
}

func nixCopyToURL(activeSSH ssh.SSHClient, machineI *machine.Machine, transferClosure bool) string {
	var storeURLParams []string

	mi := machineI.MetaInspect.Load()
	if mi != nil && !mi.Bootstrapped && transferClosure {
		storeURLParams = append(storeURLParams, "remote-store=local?root=/mnt")
	}

	return activeSSH.NixStoreURLWithParams(storeURLParams...)
}

func nixCopyBaseArgs(configurationI *configuration.Configuration, toURL string) []string {
	baseArgs := []string{"nix"}
	baseArgs = append(baseArgs, NixExperimentalFeatures...)
	baseArgs = append(baseArgs, "copy")

	if configurationI.Nix.BuildMode == nix.BuildModeRemote {
		var fromSSH ssh.SSHClient

		for i, pair := range configurationI.Machines.Pairs() {
			if i == 0 && pair.Value != nil {
				fromSSH = pair.Value.GetActiveSSH()

				break
			}
		}

		if fromSSH.IsInitialized() {
			baseArgs = append(baseArgs, "--from", fromSSH.NixStoreURL())
		}
	}

	baseArgs = append(baseArgs, "--to", toURL, "--no-check-sigs")

	return baseArgs
}
