package workflow

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeActivatePhaseMachine(fleetLeaf *fleet.FleetLeaf) error {
	return w.Phase(phases.Activate, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phase.PhaseLog) error {
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
				return executeBootstrap(exc, machine, systemClosure)
			}

			return executeActivation(exc, machine, systemClosure)
		},
	)
}

func executeBootstrap(exc *executioner.Executioner, machine *machine.Machine, systemClosure string) error {
	err := exc.Exec(
		"nixos-install",
		"installing NixOS",
		"nixos-install failed",
		slices.Concat(
			[]string{"nixos-install", "--no-root-passwd", "--no-channel-copy", "--system", systemClosure, "--root", "/mnt"},
			machine.Nix.NixosInstallFlags,
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

	machineI.State.ActiveSSH = machine.SSHTypeRegular
	activeSSH = machineI.GetActiveSSH()

	err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to come back online", "machine did not reconnect after reboot")
	if err != nil {
		return errors.Wrap(err, "wait for reconnect failed")
	}

	return nil
}

func executeActivation(exc *executioner.Executioner, machine *machine.Machine, systemClosure string) error {
	mode := machine.ActivationMode
	if machine.Flags().ActivationMode != flags.ActivationModeSwitch {
		mode = machine.Flags().ActivationMode
	}

	if mode != flags.ActivationModeTest {
		err := setSystemProfile(exc, machine, systemClosure)
		if err != nil {
			return errors.Wrap(err, "failed to set system profile")
		}
	}

	return activateConfiguration(exc, machine, systemClosure, mode)
}

// Helpers

func setSystemProfile(exc *executioner.Executioner, machine *machine.Machine, closurePath string) error {
	err := exc.Exec(
		"set system profile",
		"setting system profile",
		"failed to set system profile",
		append(machine.MaybeSudo(), "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", closurePath),
	)

	return errors.Wrap(err, "failed to set system profile")
}

func activateConfiguration(exc *executioner.Executioner, machine *machine.Machine, closurePath string, mode flags.ActivationMode) error {
	binPath := closurePath + "/bin/switch-to-configuration"

	err := exc.Exec(
		"activate",
		"activating configuration",
		"activation failed",
		append(machine.MaybeSudo(), binPath, string(mode)),
	)

	return errors.Wrap(err, "failed to activate configuration")
}
