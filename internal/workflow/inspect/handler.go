package inspect

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
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

	err = runBootstrapModeInspect(exc, fleetLeaf)
	if err != nil {
		return err
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

// runBootstrapModeInspect is the bootstrap-mode matrix: full NixOS bootstrap
// inspects the installer state, nix-install only probes nix (the bootstrap
// phase installs it), everything else requires nix to be present.
func runBootstrapModeInspect(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machineI := fleetLeaf.Machine

	// The NixOS bootstrap (kexec, disko, nixos-install) requires a Linux
	// target; a macOS host cannot run it. MetaInspect.OS is populated by
	// runCommonChecks, which always runs first.
	if fleetLeaf.Installable.Preset.BootstrapsNixOS() && isDarwinHost(machineI) {
		return errors.New("NixOS bootstrap requires a Linux target, but macOS was detected; deploy a darwinConfigurations (or homeConfigurations) output instead") //nolint:lll
	}

	switch fleetLeaf.Installable.Preset.Bootstrap {
	case installable.BootstrapNixOS:
		return runBootstrapInspect(exc, machineI)
	case installable.BootstrapNixInstall:
		return checkNixForInstall(exc, machineI)
	default:
		return phaseops.ProbeNixAvailable(exc, machineI) //nolint:wrapcheck // error is pre-annotated with its own context
	}
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

	err = detectOS(exc, machineI)
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
