package workflow

import (
	"fmt"
	"os"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeSecretsPhaseMachine(machine *config.Machine) (err error) {
	return w.Phase(machine.Logs.SafeGet(phases.Secrets),
		fmt.Sprintf("Started secrets of %s", machine.Name),
		fmt.Sprintf("Finished secrets of %s", machine.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {

			secrets := machine.Attributes.Secrets

			for _, secret := range secrets {

				if secret.Local.Path == nil {
					var f *os.File
					f, err = os.CreateTemp("", "secret-*")
					if err != nil {
						return err
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
								return errors.Wrapf(err, "writing secrets command output for '%s' failed", log.Command)
							}

							if n == 0 {
								return errors.Wrapf(err, "secrets command output was empty for '%s'", log.Command)
							}

							err = f.Close()
							if err != nil {
								return errors.Wrapf(err, "closing temporary local secrets file for '%s' failed", log.Command)
							}

							return err
						},
						"sh", "-c", *secret.Local.CommandOutput,
					)
					if err != nil {
						return err
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
					return err
				}
			}

			return nil
		})
}
