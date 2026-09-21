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

// TransferFile rsyncs source to the machine; transferOSSecrets targets
// the bootstrapping root because the final root may not exist yet.
func TransferFile(
	exc *executioner.Executioner,
	machine *machine.Machine,
	source attributes.TransferSource,
	transferOfWhat string,
	transferOSSecrets bool,
) error {
	activeSSH := machine.GetActiveSSH()

	commandWithArgs := slices.Concat([]string{"rsync"}, machine.GetRsyncDefaultFlags())

	// Elevation: remotely the receiving rsync writes the files, so sudo rides
	// along via --rsync-path; locally rsync ignores --rsync-path, so the whole
	// command is prefixed instead.
	maybeSudo := machine.MaybeSudo()
	if len(maybeSudo) != 0 {
		if activeSSH.IsLocal() {
			commandWithArgs = slices.Concat(maybeSudo, commandWithArgs)
		} else {
			commandWithArgs = append(commandWithArgs, fmt.Sprintf("--rsync-path=%s rsync", strings.Join(maybeSudo, " ")))
		}
	}

	perms := source.Permissions.String()
	commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chmod=D%s,F%s", perms, perms))

	if source.UID != nil && source.GID != nil {
		commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chown=%d:%d", *source.UID, *source.GID))
	}

	commandWithArgs = append(commandWithArgs, source.LocalPath)

	secretRemotePath := source.RemotePath
	if transferOSSecrets {
		secretRemotePath = machine.MaybeBootstrappingPath(source.RemotePath)
	}

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
