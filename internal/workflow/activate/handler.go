package activate

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

type Handler struct {
	ActivationModeOverride attributes.ActivationMode
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machine := fleetLeaf.Machine

	systemClosure := fleetLeaf.Configuration.MetaBuild.SystemClosure

	isBootstrapped := false

	mi := machine.MetaInspect.Load()
	if mi != nil {
		isBootstrapped = mi.Bootstrapped
	}

	// Run bootstrap if not bootstrapped, or force bootstrap is set
	shouldBootstrap := !isBootstrapped || machine.Bootstrap.ForceBootstrap

	if shouldBootstrap {
		return executeBootstrap(exc, machine, fleetLeaf.Configuration.Nix.NixosInstallFlags, systemClosure)
	}

	return executeActivation(exc, h.ActivationModeOverride, machine, systemClosure)
}

func executeActivation(
	exc *executioner.Executioner,
	activationModeOverride attributes.ActivationMode,
	machine *machine.Machine,
	systemClosure string,
) error {
	mode := machine.ActivationMode.Get()
	if activationModeOverride != "" {
		mode = activationModeOverride
	}

	if mode != attributes.ActivationModeTest {
		err := phaseops.SetSystemProfile(exc, machine, systemClosure)
		if err != nil {
			return errors.Wrap(err, "failed to set system profile")
		}
	}

	return errors.Wrap(phaseops.ActivateConfiguration(exc, machine, systemClosure, mode), "activation failed")
}

func executeBootstrap(exc *executioner.Executioner, machine *machine.Machine, nixosInstallFlags []string, systemClosure string) error {
	err := exc.Exec(
		"nixos-install",
		"installing NixOS",
		"nixos-install failed",
		slices.Concat(
			[]string{"nixos-install", "--no-root-passwd", "--no-channel-copy", "--system", systemClosure, "--root", "/mnt"},
			nixosInstallFlags,
		),
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
