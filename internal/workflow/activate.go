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
			systemClosure := machine.ParentConfiguration.MetaBuild.SystemClosure

			if !machine.MetaInspect.Bootstrapped.Load() && !w.conf.Flags.Bootstrap.DisableAuto {
				err := exc.Exec(
					"nixos-install",
					"installing NixOS",
					"nixos-install failed",
					[]string{"nixos-install", "--no-root-passwd", "--no-channel-copy", "--system", systemClosure, "--root", "/mnt"},
				)
				if err != nil {
					return err
				}

				if len(machine.Bootstrap.PostBootstrapInstallHooks) > 0 {
					err = exc.ExecuteHooks(machine.Bootstrap.PostBootstrapInstallHooks, "post bootstrap install hook")
					if err != nil {
						return err
					}
				}

				if !machine.Bootstrap.DisableAutomaticReboot {
					err = exc.Exec(
						"reboot",
						"rebooting",
						"reboot failed",
						[]string{"reboot"},
					)
					if err != nil {
						return err
					}
				}

				if len(machine.Bootstrap.PostBootstrapProvisionedHooks) > 0 {
					if !machine.Bootstrap.DisableAutomaticReboot {
						activeSSH := machine.MetaInspect.ActiveSSH
						err = executioner.WaitForDisconnect(exc, activeSSH, "waiting for machine to reboot")
						if err != nil {
							return err
						}

						machine.SwitchToRegularSSH()
						activeSSH = machine.MetaInspect.ActiveSSH

						err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to come back online", "machine did not reconnect after reboot")
						if err != nil {
							return err
						}
					}

					return exc.ExecuteHooks(machine.Bootstrap.PostBootstrapProvisionedHooks, "post bootstrap provisioned hook")
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
