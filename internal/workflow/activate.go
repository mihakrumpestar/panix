package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeActivatePhaseMachine(machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Activate, machine,
		func(exc *executioner.Executioner, phaseLog *phase.PhaseLog) error {
			systemClosure := machine.ParentConfiguration.MetaBuild.SystemClosure

			if !machine.MetaInspect.Bootstrapped.Load() && !w.conf.Flags.Bootstrap.DisableAuto {
				return executeBootstrap(exc, machine, systemClosure)
			}

			return executeActivation(exc, machine, systemClosure)
		},
	)
}

func executeBootstrap(exc *executioner.Executioner, machine *config.Machine, systemClosure string) error {
	err := exc.Exec(
		"nixos-install",
		"installing NixOS",
		"nixos-install failed",
		[]string{"nixos-install", "--no-root-passwd", "--no-channel-copy", "--system", systemClosure, "--root", "/mnt"},
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
		err := performReboot(exc, machine)
		if err != nil {
			return err
		}
	}

	if len(machine.Bootstrap.PostBootstrapProvisionedHooks) > 0 {
		err := exc.ExecuteHooks(machine.Bootstrap.PostBootstrapProvisionedHooks, "post bootstrap provisioned hook")
		if err != nil {
			return errors.Wrap(err, "post bootstrap provisioned hooks failed")
		}
	}

	return nil
}

func performReboot(exc *executioner.Executioner, machine *config.Machine) error {
	err := exc.Exec(
		"reboot",
		"rebooting",
		"reboot failed",
		[]string{"reboot"},
	)
	if err != nil {
		return errors.Wrap(err, "reboot failed")
	}

	if len(machine.Bootstrap.PostBootstrapProvisionedHooks) == 0 {
		return nil
	}

	activeSSH := machine.MetaInspect.GetActiveSSH()

	err = executioner.WaitForDisconnect(exc, activeSSH, "waiting for machine to reboot")
	if err != nil {
		return errors.Wrap(err, "wait for disconnect failed")
	}

	machine.SwitchToRegularSSH()
	activeSSH = machine.MetaInspect.GetActiveSSH()

	err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to come back online", "machine did not reconnect after reboot")
	if err != nil {
		return errors.Wrap(err, "wait for reconnect failed")
	}

	return nil
}

func executeActivation(exc *executioner.Executioner, machine *config.Machine, systemClosure string) error {
	err := exc.Exec(
		"add system closure to profiles",
		"updating system profile",
		"setting system closure to profiles failed",
		append(machine.MaybeSudo(), "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", systemClosure),
	)
	if err != nil {
		return errors.Wrap(err, "nix-env set system profile failed")
	}

	binPath := systemClosure + "/bin/switch-to-configuration"

	err = exc.Exec(
		"activate",
		"activating configuration",
		"activation failed",
		append(machine.MaybeSudo(), binPath, "switch"),
	)
	if err != nil {
		return errors.Wrap(err, "activation failed")
	}

	return nil
}
