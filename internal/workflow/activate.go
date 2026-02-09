package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (w *Workflow) executeActivatePhaseMachine(machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Activate, machine,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			systemClosure := machine.Configuration.MetaBuild.SystemClosure

			if !machine.MetaStatus.Bootstrapped.Load() && !w.state.Conf.Flags.Bootstrap.DisableAuto {
				err := exc.Exec(
					"nixos-install",
					"installing NixOS",
					"nixos-install failed",
					[]string{"nixos-install", "--no-root-passwd", "--no-channel-copy", "--system", systemClosure, "--root", "/mnt"},
				)
				if err != nil {
					return err
				}

				err = exc.Exec(
					"reboot",
					"rebooting",
					"reboot failed",
					[]string{"reboot"},
				)
				if err != nil {
					return err
				}
			} else {
				err := exc.Exec(
					"add system closure to profiles",
					"updating system profile",
					"setting system closure to profiles failed",
					append(machine.MaybeSudo(), "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", systemClosure),
				)
				if err != nil {
					return err
				}

				binPath := systemClosure + "/bin/switch-to-configuration"

				err = exc.Exec(
					"activate",
					"activating configuration",
					"activation failed",
					append(machine.MaybeSudo(), binPath, "switch"),
				)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}
