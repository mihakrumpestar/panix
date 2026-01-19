package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeActivatePhaseMachine(machine *config.Machine) error {
	return w.Phase(machine.Attributes, phases.Activate, machine,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {

			systemClosure := machine.Configuration.MetaBuild.SystemClosure

			if !machine.MetaStatus.Bootstrapped.Load() && !w.state.Conf.Flags.Bootstrap.DisableAuto {

				commandWithArgs := []string{"nixos-install", "--no-root-passwd", "--no-channel-copy", "--system", systemClosure, "--root", "/mnt"}

				err := exc.Exec("nixos-install",
					commandWithArgs,
					executioner.OnFailure(func(log *logs.CommandLog, err error) error {
						return errors.Wrapf(err, "running nixos-install failed for %s", machine.Name)
					}),
				)
				if err != nil {
					return err
				}

				commandWithArgs = []string{"reboot"}

				err = exc.Exec("reboot",
					commandWithArgs,
					executioner.OnFailure(func(log *logs.CommandLog, err error) error {
						return errors.Wrapf(err, "running reboot failed for %s", machine.Name)
					}),
				)
				if err != nil {
					return err
				}

			} else {

				commandWithArgs := append(machine.MaybeSudo(), "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", systemClosure)

				err := exc.Exec("add system closure to profiles",
					commandWithArgs,
					executioner.OnFailure(func(log *logs.CommandLog, err error) error {
						return errors.Wrapf(err, "adding new system closure to profiles failed for %s", machine.Name)
					}),
				)
				if err != nil {
					return err
				}

				binPath := systemClosure + "/bin/switch-to-configuration"

				commandWithArgs = append(machine.MaybeSudo(), binPath, "switch")

				err = exc.Exec("activate",
					commandWithArgs,
					executioner.OnFailure(func(log *logs.CommandLog, err error) error {
						return errors.Wrapf(err, "activation failed for %s", machine.Name)
					}),
				)
				if err != nil {
					return err
				}

			}

			return nil
		},
	)
}
