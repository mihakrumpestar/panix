package workflow

import (
	"fmt"
	"os"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeSecretsPhaseMachine(machine *config.Machine) (err error) {
	secrets := machine.Secrets

	if len(secrets) == 0 {
		return nil
	}

	return w.Phase(machine.Attributes.Xpath, phases.Secrets, nil,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {

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

					commandWithArgs := []string{"sh", "-c", *secret.Local.CommandOutput}

					err = exc.Exec(
						"secrets command",
						"secrets command failed",
						commandWithArgs,
						executioner.DisableAutoSshCommand(),
						executioner.OnSuccess(func(log *logs_command.CommandLog) error {
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
						}),
						executioner.OnDryRun(func() {
							// In dry-run mode, skip writing to file
							_ = f.Close()
						}),
					)
					if err != nil {
						return err
					}
				}

				commandWithArgs := []string{"rsync", "-rcPEx"}

				maybeSudo := machine.MaybeSudo()
				if len(maybeSudo) == 1 {
					commandWithArgs = append(commandWithArgs, fmt.Sprintf("--rsync-path=%s rsync", maybeSudo[0]))
				}

				if secret.Remote.UID != nil && secret.Remote.GID != nil {
					commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chmod=%d:%d", secret.Remote.UID, secret.Remote.GID))
				}

				commandWithArgs = append(commandWithArgs, *secret.Local.Path)

				secretRemotePath := machine.MaybeBootstrappingPath(secret.Remote.Path)
				if machine.SSH.IsLocal {
					commandWithArgs = append(commandWithArgs, secretRemotePath)
				} else {
					sshArgs := machine.SSH.MaybeSshCommandArguments()
					if len(sshArgs) != 0 {
						commandWithArgs = append(commandWithArgs, "-e=ssh "+strings.Join(sshArgs, " "))
					}

					commandWithArgs = append(commandWithArgs, fmt.Sprintf("%s:%s", machine.SSH.Hostname, secretRemotePath))
				}

				err = exc.Exec(
					"transfer of secrets",
					"secrets transfer failed",
					commandWithArgs,
				)
				if err != nil {
					return err
				}
			}

			return nil
		})
}
