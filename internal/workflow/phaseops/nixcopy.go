package phaseops

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func CopyClosure(
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
		Str("machine", machineI.Name.String()).
		Strs("transferred", toTransfer).
		Msgf("Transferred %s to %s", toTransfer, machineI.Name.String())

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
