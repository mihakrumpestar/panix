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

	toURL := nixCopyToURL(activeSSH, machineI, installable.Type, transferClosure)
	sshOpts := activeSSH.MaybeNixSSHOpts()

	baseArgs := nixCopyBaseArgs(installable, toURL)
	commandWithArgs := slices.Concat(
		baseArgs,
		slices.Concat(installable.Nix.ExtraFlags, installable.Nix.CopyFlags),
		toTransfer,
	)

	// User env first so panix-internal NIX_SSHOPTS takes precedence on conflict.
	env := slices.Concat(installable.Nix.GetCopyEnv(), sshOpts)

	err := exc.Exec("nix copy",
		"copying closure",
		"closure copy failed",
		commandWithArgs,
		executioner.SkipIfLocal(),
		executioner.DisableAutoSSHCommand(),
		executioner.Env(env),
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

func nixCopyToURL(activeSSH ssh.SSHClient, machineI *machine.Machine, outputType installable.FlakeOutputType, transferClosure bool) string {
	var storeURLParams []string

	// Only redirect to /mnt for bootstrappable output types that are being bootstrapped.
	// Non-bootstrappable output types (homeConfigurations, packages, etc.) always copy
	// to the live system's /nix/store, not /mnt.
	mi := machineI.MetaInspect.Load()
	if mi != nil && !mi.Bootstrapped && transferClosure && installable.IsBootstrappableType(outputType) {
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
