package workflow

import (
	"fmt"
	"os"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeSecretsPhaseMachine(machine *config.Machine) (err error) {
	secrets := machine.Secrets

	if len(secrets) == 0 {
		return nil
	}

	return w.Phase(&machine.Attributes, phases.Secrets, nil,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {

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

					err = exc.Exec(false, true,
						func(log *logs.CommandLog, err error) error {
							return errors.Wrapf(err, "secrets command failed for %s", machine.Name)
						},
						func(log *logs.CommandLog) error {
							output := log.Bytes()

							var n int
							n, err = f.Write(output)
							if err != nil {
								return errors.Wrapf(err, "writing secrets command output for '%s' failed", log.Command.Load())
							}

							if n == 0 {
								return errors.Wrapf(err, "secrets command output was empty for '%s'", log.Command.Load())
							}

							err = f.Close()
							if err != nil {
								return errors.Wrapf(err, "closing temporary local secrets file for '%s' failed", log.Command.Load())
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
					commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chmod=%d:%d", secret.Remote.UID, secret.Remote.GID))
				}

				secretRemotePath := secret.Remote.Path
				if !machine.MetaStatus.Bootstrapped.Load() && !w.state.Conf.Flags.Bootstrap.DisableAuto {
					secretRemotePath = `/mnt` + secretRemotePath
				}

				commandWithArgs = append(commandWithArgs, *secret.Local.Path)

				if machine.Ssh.IsLocal {
					commandWithArgs = append(commandWithArgs, secretRemotePath)
				} else {
					if machine.Ssh.HostnameIsAlias {
						commandWithArgs = append(commandWithArgs, fmt.Sprintf("%s:%s", machine.Ssh.Hostname, secretRemotePath))
					} else {

						maybeIdentityFile := ""
						if machine.Ssh.IdentityFile != "" {
							maybeIdentityFile = " -i " + machine.Ssh.IdentityFile
						}

						commandWithArgs = append(commandWithArgs, "-e", fmt.Sprintf("'ssh -p %d%s'", machine.Ssh.Port, maybeIdentityFile))

						commandWithArgs = append(commandWithArgs, fmt.Sprintf("%s@%s:%s", machine.Ssh.Username, machine.Ssh.Hostname, secretRemotePath))
					}
				}

				err = exc.Exec(false, false,
					func(log *logs.CommandLog, err error) error {
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
