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

// TransferFile rsyncs source to the machine; bootstrapsNixOS routes the
// destination via the bootstrapping root, because the final root may not
// exist yet (see TransferSecret for the parameter semantics).
func TransferFile(
	exc *executioner.Executioner,
	machine *machine.Machine,
	source attributes.TransferSource,
	transferOfWhat string,
	bootstrapsNixOS bool,
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

	commandWithArgs = append(commandWithArgs, "--chmod="+rsyncChmodFlag(source.Permissions))

	if source.UID != nil && source.GID != nil {
		commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chown=%d:%d", *source.UID, *source.GID))
	}

	commandWithArgs = append(commandWithArgs, source.LocalPath)

	secretRemotePath := machine.MaybeBootstrappingPath(source.RemotePath, bootstrapsNixOS)

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

// rsyncChmodFlag renders the --chmod value: files use the configured mode,
// while directories stay traversable. Owner rwx is always set; group and other
// execute follow the corresponding read bit, so a 0600 file mode yields D700
// instead of the untraversable D600.
func rsyncChmodFlag(perms attributes.FileMode) string {
	const (
		permissionBits = 0o777
		ownerAccess    = 0o700
	)

	fileMode := perms.Get() & permissionBits

	dirMode := fileMode | ownerAccess
	if fileMode&0o040 != 0 {
		dirMode |= 0o010
	}

	if fileMode&0o004 != 0 {
		dirMode |= 0o001
	}

	return fmt.Sprintf("D%o,F%o", dirMode, fileMode)
}
