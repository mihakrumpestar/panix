package bootstrap

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

var ErrDiskoNoOutputPaths = errors.New("disko build output did not contain any output paths")

// Upload disk encryption keys BEFORE running disko.
// Keys must be available for LUKS unlocking during partitioning.
func disko(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf, outLink string) error {
	flake := fleetLeaf.Flake
	machineI := fleetLeaf.Machine

	// The disko script lives under the same NixOS system output as the
	// system closure (NixOSDiskoScriptPath), so resolve the base attrpath
	// exactly like every other installable lookup.
	attrBase := installable.ResolveFlakeAttrBase(fleetLeaf.Installable.Type, fleetLeaf.Installable.Name, fleetLeaf.Installable.Preset)
	installables := []string{fmt.Sprintf("%s#%s.%s", flake.URL, attrBase, installable.NixOSDiskoScriptPath)}

	diskoScript, err := phaseops.BuildInstallable(exc, fleetLeaf, installables, "disko", outLink)
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

	if len(machineI.Bootstrap.DiskEncryptionKeys) > 0 {
		err = executeDiskEncryptionKeys(exc, machineI)
		if err != nil {
			return err
		}
	}

	// The disko script partitions disks and must run as root.
	err = exc.Exec(
		"disko",
		"partitioning disk",
		"diskoScript failed",
		append(machineI.MaybeSudo(), diskoScript),
		executioner.Trim(),
	)
	if err != nil {
		return errors.Wrap(err, "disko failed")
	}

	return nil
}

// executeDiskEncryptionKeys must run before disko: the keys are needed for
// LUKS unlocking during partitioning.
func executeDiskEncryptionKeys(exc *executioner.Executioner, machineI *machine.Machine) error {
	for _, diskEncryptionKey := range machineI.Bootstrap.DiskEncryptionKeys {
		err := phaseops.TransferSecret(exc, machineI, diskEncryptionKey, "disk encryption key", false)
		if err != nil {
			return errors.Wrapf(err, "failed to transfer disk encryption key to %s", diskEncryptionKey.RemotePath)
		}
	}

	return nil
}
