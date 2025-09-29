package workflow

import (
	"fmt"
	"os"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) ExecuteSecretsPhase() error {
	return poolChildren(w, w.state.Conf.Root, true, func(flake *config.Flake) error {
		return poolChildren(w, flake, true, func(configuration *config.Configuration) error {
			return poolChildren(w, configuration, true, func(machine *config.Machine) error {
				return w.executeSecretsPhaseMachine(machine)
			})
		})
	})
}

func (w *Workflow) executeSecretsPhaseMachine(machine *config.Machine) (err error) {
	log := machine.Logs.SafeGet(phases.Secrets)
	log.TimeAndState.StartTimer()
	defer func() {
		log.TimeAndState.EndTimerWithError(err)
	}()

	if w.state.Conf.Global.Verbose {
		log.AddMessageOnly(fmt.Sprintf("Starting secrets of %s", machine.Name))
	}

	if w.state.Conf.Global.DryRun {
		return
	}

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, nil, log, w.hook.OnUpdateHook)

	secrets := machine.Attributes.Secrets

	for _, secret := range secrets {

		if secret.Local.Path == nil {
			var f *os.File
			f, err = os.CreateTemp("", "secret-*")
			if err != nil {
				return
			}

			fileName := f.Name()
			defer os.Remove(fileName)
			secret.Local.Path = &fileName

			err = exc.Exec(false, false,
				func(log *config.CommandLog, err error) error {
					return errors.Wrapf(err, "secrets command failed for %s", machine.Name)
				},
				func(log *config.CommandLog) error {
					output := log.Bytes()

					var n int
					n, err = f.Write(output)
					if err != nil {
						errors.Wrapf(err, "writing secrets command output for '%s' failed", log.Command)
						return err
					}

					if n == 0 {
						errors.Wrapf(err, "secrets command output was empty for '%s'", log.Command)
						return err
					}

					err = f.Close()
					if err != nil {
						errors.Wrapf(err, "closing temporary local secrets file for '%s' failed", log.Command)
						return err
					}

					return err
				},
				"sh", "-c", *secret.Local.CommandOutput,
			)
			if err != nil {
				return
			}
		}

		commandWithArgs := []string{"rsync", fmt.Sprintf("--rsync-path=%s rsync", *machine.SudoProgram), "-rcPEx"}

		if secret.Remote.UID != nil && secret.Remote.GID != nil {
			commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chmod=%s:%s", secret.Remote.UID, secret.Remote.GID))
		}

		commandWithArgs = append(commandWithArgs, *secret.Local.Path, fmt.Sprintf("%s:%s", machine.Ssh.Hostname, secret.Remote.Path))

		err = exc.Exec(false, true,
			func(log *config.CommandLog, err error) error {
				return errors.Wrapf(err, "secrets failed for %s", machine.Name)
			},
			nil,
			commandWithArgs...,
		)
		if err != nil {
			return
		}
	}

	if w.state.Conf.Global.Verbose {
		log.AddMessageOnly(fmt.Sprintf("Secrets %s successfully", machine.Name))
	}

	return
}
