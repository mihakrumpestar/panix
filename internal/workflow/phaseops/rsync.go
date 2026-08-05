package phaseops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/pkg/errors"
)

func TransferFile(
	exc *executioner.Executioner,
	machine *machine.Machine,
	plainFileOrDir attributes.PlainFileOrDirToTransfer,
	transferOfWhat string,
	transferOSSecrets bool,
) error {
	commandWithArgs := slices.Concat([]string{"rsync"}, machine.GetRsyncDefaultFlags())

	maybeSudo := machine.MaybeSudo()
	if len(maybeSudo) != 0 {
		commandWithArgs = append(commandWithArgs, fmt.Sprintf("--rsync-path=%s rsync", strings.Join(maybeSudo, " ")))
	}

	perms := plainFileOrDir.Permissions.String()
	commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chmod=D%s,F%s", perms, perms))

	if plainFileOrDir.UID != nil && plainFileOrDir.GID != nil {
		commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chown=%d:%d", *plainFileOrDir.UID, *plainFileOrDir.GID))
	}

	commandWithArgs = append(commandWithArgs, plainFileOrDir.LocalPath)

	secretRemotePath := plainFileOrDir.RemotePath
	if transferOSSecrets {
		secretRemotePath = machine.MaybeBootstrappingPath(plainFileOrDir.RemotePath)
	}

	activeSSH := machine.GetActiveSSH()
	if activeSSH.IsLocal() {
		commandWithArgs = append(commandWithArgs, secretRemotePath)
	} else {
		sshArgs := activeSSH.MaybeSSHCommandArguments()
		if len(sshArgs) != 0 {
			commandWithArgs = append(commandWithArgs, "-e=ssh "+strings.Join(sshArgs, " "))
		}

		commandWithArgs = append(commandWithArgs, fmt.Sprintf("%s:%s", activeSSH.SSHTarget(), secretRemotePath))
	}

	err := exc.Exec(
		"transfer of "+transferOfWhat,
		"transferring "+transferOfWhat,
		transferOfWhat+" transfer failed",
		commandWithArgs,
		executioner.DisableAutoSSHCommand(),
		executioner.Trim(),
	)
	if err != nil {
		return errors.Wrap(err, "transfer failed")
	}

	return nil
}
