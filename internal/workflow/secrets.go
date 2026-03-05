package workflow

import (
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (w *Workflow) executeSecretsPhaseMachine(machine *config.Machine) (err error) {
	secrets := machine.Secrets

	if len(secrets) == 0 {
		return nil
	}

	return w.Phase(machine.Attributes.Xpath, phases.Secrets, nil,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			for _, secret := range secrets {
				err = w.transferPlainFileOrDir(exc, machine, secret, "secrets", true)
				if err != nil {
					return err
				}
			}

			return nil
		})
}

func (w *Workflow) transferPlainFileOrDir(exc *executioner.Executioner, machine *config.Machine, plainFileOrDir *config_attributes.PlainFileOrDirToTransfer, transferOfWhat string, transferOSSecrets bool) error {
	commandWithArgs := []string{"rsync", "-rcPEx", "--mkpath"}

	maybeSudo := machine.MaybeSudo()
	if len(maybeSudo) != 0 {
		commandWithArgs = append(commandWithArgs, fmt.Sprintf("--rsync-path=%s rsync", strings.Join(maybeSudo, " ")))
	}

	perms := plainFileOrDir.GetPermissions()
	commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chmod=D%o,F%o", perms, perms))

	if plainFileOrDir.UID != nil && plainFileOrDir.GID != nil {
		commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chown=%d:%d", *plainFileOrDir.UID, *plainFileOrDir.GID))
	}

	commandWithArgs = append(commandWithArgs, plainFileOrDir.LocalPath)

	secretRemotePath := plainFileOrDir.RemotePath
	if transferOSSecrets {
		secretRemotePath = machine.MaybeBootstrappingPath(plainFileOrDir.RemotePath)
	}

	activeSSH := machine.MetaInspect.GetActiveSSH()
	if activeSSH.IsLocal {
		commandWithArgs = append(commandWithArgs, secretRemotePath)
	} else {
		sshArgs := activeSSH.MaybeSshCommandArguments()
		if len(sshArgs) != 0 {
			commandWithArgs = append(commandWithArgs, "-e=ssh "+strings.Join(sshArgs, " "))
		}

		commandWithArgs = append(commandWithArgs, fmt.Sprintf("%s:%s", activeSSH.Hostname, secretRemotePath))
	}

	err := exc.Exec(
		fmt.Sprintf("transfer of %s", transferOfWhat),
		fmt.Sprintf("transferring %s", transferOfWhat),
		fmt.Sprintf("%s transfer failed", transferOfWhat),
		commandWithArgs,
		executioner.DisableAutoSshCommand(),
	)
	if err != nil {
		return err
	}

	return nil
}
