package inspect

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

type Handler struct{}

func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machineI := fleetLeaf.Machine

	err := runCommonChecks(exc, machineI)
	if err != nil {
		return err
	}

	// Bootstrap detection (only for bootstrappable types)
	if fleetLeaf.Installable.Type.IsBootstrappable() {
		err = runBootstrapInspect(exc, machineI)
		if err != nil {
			return err
		}
	} else {
		// Non-bootstrappable: check nix is available
		err = checkNixAvailable(exc, machineI)
		if err != nil {
			return err
		}
	}

	// System info detection (all types)
	err = detectSystemInfo(exc, machineI)
	if err != nil {
		return err
	}

	// Generation reading (all types with a profile path)
	preset, ok := installable.GetPreset(fleetLeaf.Installable.Type)
	if ok && preset.ProfilePath != "" {
		return readGenerations(exc, machineI, preset.ProfilePath, fleetLeaf.Installable.User)
	}

	return nil
}

// runCommonChecks runs SSH reachability, architecture, and superuser checks
// that apply to all output types.
func runCommonChecks(exc *executioner.Executioner, machineI *machine.Machine) error {
	if machineI.SSH.IsLocal() {
		machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
			mi.Reachable = true
			mi.SSHConnectable = true
		})
	} else {
		err := checkSSHReachability(exc, machineI)
		if err != nil {
			return err
		}

		err = checkSSHConnection(exc, machineI)
		if err != nil {
			return err
		}
	}

	err := detectArchitecture(exc, machineI)
	if err != nil {
		return err
	}

	return checkSuperuser(exc, machineI)
}

// runBootstrapInspect runs bootstrap-specific inspection: detects bootstrap
// status, validates SSH state and secrets paths, and handles unbootstrapped
// machines.
func runBootstrapInspect(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := detectBootstrapStatus(exc, machineI)
	if err != nil {
		return err
	}

	err = validateSSHMachineState(exc, machineI)
	if err != nil {
		return err
	}

	err = validateSecretsPaths(exc, machineI)
	if err != nil {
		return err
	}

	mi := machineI.MetaInspect.Load()
	if mi != nil && !mi.Bootstrapped {
		return handleUnbootstrapped(exc, machineI)
	}

	return nil
}
