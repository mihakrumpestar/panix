package bootstrap

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

// ErrUnsupportedBootstrapMode fails closed for a BootstrapMode with no
// registered bootstrap path in RunPhase.
var ErrUnsupportedBootstrapMode = errors.New("unsupported bootstrap mode")

type Handler struct {
	OutLinks phaseops.OutLinks
}

func (Handler) ShouldSkip(fleetLeaf *fleet.FleetLeaf) bool {
	preset := fleetLeaf.Installable.Preset
	if !preset.IsBootstrappable() {
		return true
	}

	metaInspect := fleetLeaf.Machine.MetaInspect.Load()
	forceBootstrap := fleetLeaf.Machine.Bootstrap.ForceBootstrap

	if preset.BootstrapsNixOS() {
		return metaInspect != nil && metaInspect.Bootstrapped && !forceBootstrap
	}

	return metaInspect != nil && metaInspect.NixAvailable && !forceBootstrap
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	switch fleetLeaf.Installable.Preset.Bootstrap {
	case installable.BootstrapNixInstall:
		return runNixInstallBootstrap(exc, fleetLeaf.Machine)
	case installable.BootstrapNixOS:
		return runNixOSBootstrap(exc, fleetLeaf, h.OutLinks.DiskoPath(fleetLeaf.Installable))
	default:
		return errors.Wrapf(ErrUnsupportedBootstrapMode, "%q", fleetLeaf.Installable.Preset.Bootstrap)
	}
}

// runNixInstallBootstrap installs Nix on targets that lack it, then runs the
// post-bootstrap hooks. Whether an install is needed is settled once, in
// ShouldSkip (nix missing or forced); here bootstrap.disable_nix_install opts
// out of the install step only (the inspect gate already hard-errors when nix
// is missing then): the hooks run regardless of whether the install step ran.
func runNixInstallBootstrap(exc *executioner.Executioner, machineI *machine.Machine) error {
	if !machineI.Bootstrap.DisableNixInstall {
		err := installNix(exc, machineI)
		if err != nil {
			return errors.Wrap(err, "nix install failed")
		}
	}

	return runPostBootstrapHooks(exc, machineI)
}

// runNixOSBootstrap is the NixOS path: kexec, hardware config, disko, hooks.
func runNixOSBootstrap(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf, diskoOutLink string) error {
	machineI := fleetLeaf.Machine

	metaInspect := machineI.MetaInspect.Load()
	if metaInspect != nil && metaInspect.RequiresKexec {
		err := executeKexec(exc, machineI)
		if err != nil {
			return err
		}

		// The target now runs the NixOS installer, so nixos-generate-config
		// exists: generate the hardware config before disko, whose build
		// evaluates the operator's configuration, which may import it.
		err = phaseops.GenerateHardwareConfig(exc, machineI)
		if err != nil {
			return err //nolint:wrapcheck // error is pre-annotated with its own context
		}
	}

	if !machineI.Bootstrap.DisableDisko {
		err := disko(exc, fleetLeaf, diskoOutLink)
		if err != nil {
			return err
		}
	}

	return runPostBootstrapHooks(exc, machineI)
}

func runPostBootstrapHooks(exc *executioner.Executioner, machineI *machine.Machine) error {
	return errors.Wrap(exc.ExecuteHooks(machineI.Bootstrap.PostBootstrapHooks, "post bootstrap hook"), "post bootstrap hook failed")
}
