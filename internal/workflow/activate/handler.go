package activate

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/pkg/errors"
)

type Handler struct {
	ActivationMode flags.ActivationMode
	NixFlavor      nixver.Flavor
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machine := fleetLeaf.Machine

	systemClosure := fleetLeaf.Installable.MetaBuild.Closure

	isBootstrapped := false

	mi := machine.MetaInspect.Load()
	if mi != nil {
		isBootstrapped = mi.Bootstrapped
	}

	// Run bootstrap if not bootstrapped, or force bootstrap is set
	shouldBootstrap := !isBootstrapped || machine.Bootstrap.ForceBootstrap

	if fleetLeaf.Installable.Preset.IsBootstrappable && shouldBootstrap {
		return executeBootstrap(exc, machine, &fleetLeaf.Installable.Nix, systemClosure)
	}

	return executeActivation(exc, h.ActivationMode, h.NixFlavor, fleetLeaf, systemClosure, &fleetLeaf.Installable.Nix)
}

func executeActivation(
	exc *executioner.Executioner,
	activationMode flags.ActivationMode,
	nixFlavor nixver.Flavor,
	fleetLeaf *fleet.FleetLeaf,
	closure string,
	nixCfg *nix.NixConfig,
) error {
	preset := &fleetLeaf.Installable.Preset

	mode := preset.ActivationDefaultMode
	if fleetLeaf.Installable.ActivationMode != "" {
		mode = fleetLeaf.Installable.ActivationMode
	}

	override := activationMode.Get(fleetLeaf.Installable.Type.String())
	if override != "" {
		mode = override
	}

	return errors.Wrap(
		phaseops.Activate(exc, fleetLeaf.Machine, *preset, closure, mode, fleetLeaf.Installable.User, nixCfg, nixFlavor),
		"activation failed",
	)
}

func executeBootstrap(exc *executioner.Executioner, machine *machine.Machine, nixCfg *nix.NixConfig, systemClosure string) error {
	err := exc.Exec(
		"nixos-install",
		"installing NixOS",
		"nixos-install failed",
		slices.Concat(
			[]string{"nixos-install"},
			nixCfg.GetNixosInstallDefaultFlags(),
			[]string{"--system", systemClosure, "--root", "/mnt"},
			nixCfg.NixosInstallFlags,
		),
		executioner.Env(nixCfg.GetNixosInstallEnv()),
		executioner.Trim(),
	)
	if err != nil {
		return errors.Wrap(err, "nixos-install failed")
	}

	if len(machine.Bootstrap.PostBootstrapInstallHooks) > 0 {
		err = exc.ExecuteHooks(machine.Bootstrap.PostBootstrapInstallHooks, "post bootstrap install hook")
		if err != nil {
			return errors.Wrap(err, "post bootstrap install hooks failed")
		}
	}

	if !machine.Bootstrap.DisableAutomaticReboot {
		err = performReboot(exc, machine)
		if err != nil {
			return err
		}
	}

	if len(machine.Bootstrap.PostBootstrapProvisionedHooks) > 0 {
		err = exc.ExecuteHooks(machine.Bootstrap.PostBootstrapProvisionedHooks, "post bootstrap provisioned hook")
		if err != nil {
			return errors.Wrap(err, "post bootstrap provisioned hooks failed")
		}
	}

	return nil
}

// Helpers

func performReboot(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.Exec(
		"reboot",
		"rebooting",
		"reboot failed",
		[]string{"reboot"},
	)
	if err != nil {
		return errors.Wrap(err, "reboot failed")
	}

	if len(machineI.Bootstrap.PostBootstrapProvisionedHooks) == 0 {
		return nil
	}

	activeSSH := machineI.GetActiveSSH()

	err = executioner.WaitForDisconnect(exc, activeSSH, "waiting for machine to reboot")
	if err != nil {
		return errors.Wrap(err, "wait for disconnect failed")
	}

	machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeRegular })
	activeSSH = machineI.GetActiveSSH()

	err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to come back online", "machine did not reconnect after reboot")
	if err != nil {
		return errors.Wrap(err, "wait for reconnect failed")
	}

	return nil
}
