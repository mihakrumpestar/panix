package phaseops

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
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
	installable := fleetLeaf.Installable
	activeSSH := machineI.GetActiveSSH()

	toURL := nixCopyToURL(activeSSH, machineI, installable.Preset, transferClosure)
	sshOpts := activeSSH.MaybeNixSSHOpts()

	baseArgs := nixCopyBaseArgs(installable, toURL)
	// User env first so panix-internal NIX_SSHOPTS takes precedence on conflict.
	env := slices.Concat(installable.Nix.GetCopyEnv(), sshOpts)
	commandWithArgs := WithEnv(env, slices.Concat(
		baseArgs,
		slices.Concat(installable.Nix.ExtraFlags, installable.Nix.CopyFlags),
		toTransfer,
	))

	err := exc.Exec("nix copy",
		"copying closure",
		"closure copy failed",
		commandWithArgs,
		executioner.SkipIfLocal(),
		executioner.DisableAutoSSHCommand(),
		executioner.Trim(),
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

func nixCopyToURL(activeSSH ssh.SSHClient, machineI *machine.Machine, preset installable.Preset, transferClosure bool) string {
	var storeURLParams []string

	// Only redirect to /mnt while NixOS-bootstrapping (the target is the
	// installer and /mnt is the future root). All other bootstrap modes copy
	// to the live system's /nix/store. Same predicate as the bootstrapping
	// root transfers (Machine.MaybeBootstrappingPath).
	if transferClosure && machineI.NixOSBootstrappingInProgress(preset.BootstrapsNixOS()) {
		storeURLParams = append(storeURLParams, "remote-store=local?root=/mnt")
	}

	return activeSSH.NixStoreURLWithParams(storeURLParams...)
}

func nixCopyBaseArgs(installable *installable.Installable, toURL string) []string {
	baseArgs := []string{"nix"}
	baseArgs = append(baseArgs, installable.Nix.GetExperimentalFeatures()...)
	baseArgs = append(baseArgs, "copy")

	if installable.Nix.BuildMode == nix.BuildModeRemote {
		// Copy from the pinned builder machine: the same first declared
		// machine the build phase built on (see remoteBuilderSSH in build.go).
		// GetActiveSSH panics on uninitialized clients, so no fallback exists:
		// remote transfer without a valid builder is a hard failure.
		baseArgs = append(baseArgs, "--from", remoteBuilderSSH(installable).NixStoreURL())
	}

	baseArgs = append(baseArgs, "--to", toURL)
	baseArgs = append(baseArgs, installable.Nix.GetCopyDefaultFlags()...)

	return baseArgs
}
