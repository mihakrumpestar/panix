package workflow

import (
	"fmt"
	"os"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
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
					err = w.generateSecretFromCommand(exc, secret)
					if err != nil {
						return err
					}
				}

				err = w.transferSecret(exc, machine, secret)
				if err != nil {
					return err
				}
			}

			return nil
		})
}

func (w *Workflow) generateSecretFromCommand(exc *executioner.Executioner, secret *config_attributes.Secret) error {
	f, err := os.CreateTemp("", "secret-*")
	if err != nil {
		return errors.Wrap(err, "creating temp file for secret")
	}

	fileName := f.Name()
	secret.Local.Path = &fileName

	execErr := exc.Exec(
		"secrets command",
		"generating secret",
		"secrets command failed",
		[]string{"sh", "-c", *secret.Local.CommandOutput},
		executioner.DisableAutoSshCommand(),
		executioner.OnSuccess(func(log *logs_command.CommandLog) error {
			output := log.Bytes()
			if len(output) == 0 {
				return errors.New("secrets command output was empty")
			}

			n, err := f.Write(output)
			if err != nil {
				return errors.Wrap(err, "writing secrets command output")
			}

			if n == 0 {
				return errors.New("secrets command output was empty after write")
			}

			return nil
		}),
		executioner.OnDryRun(func() {
			_ = f.Close()
		}),
	)

	closeErr := f.Close()

	if execErr != nil {
		_ = os.Remove(fileName)
		return execErr
	}

	if closeErr != nil {
		_ = os.Remove(fileName)
		return errors.Wrap(closeErr, "closing temp file for secret")
	}

	return nil
}

func (w *Workflow) transferSecret(exc *executioner.Executioner, machine *config.Machine, secret *config_attributes.Secret) error {
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

	err := exc.Exec(
		"transfer of secrets",
		"transferring secrets",
		"secrets transfer failed",
		commandWithArgs,
	)
	if err != nil {
		return err
	}

	return nil
}
