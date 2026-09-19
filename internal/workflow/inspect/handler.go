package inspect

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
)

type Handler struct{}

func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machineI := fleetLeaf.Machine

	err := runCommonChecks(exc, machineI)
	if err != nil {
		return err
	}

	// su -l requires a root executing user: otherwise the PTY prompts for
	// a password and hangs until timeout. Validate before any deploy phase;
	// Activate re-validates after connection switches.
	err = phaseops.ValidateTargetUser(fleetLeaf.Installable, machineI)
	if err != nil {
		return err //nolint:wrapcheck // error names installable, target user, and executing user
	}

	if fleetLeaf.Installable.Preset.IsBootstrappable {
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
	if fleetLeaf.Installable.Preset.ProfilePath != "" {
		return readGenerations(
			exc,
			machineI,
			fleetLeaf.Installable.Preset,
			fleetLeaf.Installable.Preset.ProfilePath,
			fleetLeaf.Installable.User,
		)
	}

	return nil
}

// runCommonChecks covers the checks that apply to all output types.
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

	return phaseops.RefreshSuperuser(exc, machineI) //nolint:wrapcheck // error is pre-annotated with its own context
}

// runBootstrapInspect: order matters. The SSH and secrets validations read
// the status detected first; unbootstrapped handling runs only when not
// bootstrapped.
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
