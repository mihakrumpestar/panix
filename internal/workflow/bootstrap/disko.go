package bootstrap

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

var ErrDiskoNoOutputPaths = errors.New("disko build output did not contain any output paths")

// Upload disk encryption keys BEFORE running disko.
// Keys must be available for LUKS unlocking during partitioning.
func disko(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	flake := fleetLeaf.Flake
	installable := fleetLeaf.Installable
	machine := fleetLeaf.Machine

	installables := []string{fmt.Sprintf("%s#%s.%s.config.system.build.diskoScript", flake.URL, installable.Type, installable.Name)}

	diskoScript, err := phaseops.BuildInstallable(exc, fleetLeaf, installables, "disko")
	if err != nil {
		return errors.Wrap(err, "disko build failed")
	}

	if diskoScript == "" {
		return ErrDiskoNoOutputPaths
	}

	err = phaseops.CopyClosure(exc, fleetLeaf, []string{diskoScript}, false)
	if err != nil {
		return errors.Wrap(err, "disko transfer failed")
	}

	if len(machine.Bootstrap.DiskEncryptionKeys) > 0 {
		err = executeDiskEncryptionKeys(exc, machine)
		if err != nil {
			return err
		}
	}

	err = exc.Exec(
		"disko",
		"partitioning disk",
		"diskoScript failed",
		[]string{diskoScript},
		executioner.Trim(),
	)
	if err != nil {
		return errors.Wrap(err, "disko failed")
	}

	return nil
}

// executeDiskEncryptionKeys transfers disk encryption keys to the target machine.
// Must be called BEFORE disko runs, so keys are available for LUKS unlocking.
func executeDiskEncryptionKeys(exc *executioner.Executioner, machine *machine.Machine) error {
	for _, diskEncryptionKey := range machine.Bootstrap.DiskEncryptionKeys {
		err := phaseops.TransferFile(exc, machine, diskEncryptionKey, "disk encryption key", false)
		if err != nil {
			return errors.Wrapf(err, "failed to transfer disk encryption key to %s", diskEncryptionKey.RemotePath)
		}
	}

	return nil
}
