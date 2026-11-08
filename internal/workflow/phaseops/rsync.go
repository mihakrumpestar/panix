package phaseops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/structdefault"
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

	commandWithArgs := rsyncCommandArgs(machine, activeSSH, source)
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

// rsyncCommandArgs assembles the leading argv: the configured command, default
// flags, elevation and --chmod/--chown.
//
// Remotely the receiving rsync writes the files, so sudo rides along via
// --rsync-path; locally the whole command is elevated instead. --rsync-path is
// also the only channel a configured command reaches the remote side on, so it
// is emitted whenever the command is not the built-in one.
func rsyncCommandArgs(machine *machine.Machine, activeSSH ssh.SSHClient, source attributes.TransferSource) []string {
	commandWithArgs := slices.Concat(machine.RsyncCommand, machine.RsyncDefaultFlags)
	maybeSudo := machine.MaybeSudo()

	switch {
	case activeSSH.IsLocal() && len(maybeSudo) != 0:
		commandWithArgs = slices.Concat(maybeSudo, commandWithArgs)
	case !activeSSH.IsLocal() && (len(maybeSudo) != 0 || !isBuiltinRsyncCommand(machine.RsyncCommand)):
		commandWithArgs = append(commandWithArgs, "--rsync-path="+remoteRsyncPath(maybeSudo, machine.RsyncCommand))
	}

	commandWithArgs = append(commandWithArgs, "--chmod="+rsyncChmodFlag(source.Permissions))

	if source.UID != nil && source.GID != nil {
		commandWithArgs = append(commandWithArgs, fmt.Sprintf("--chown=%d:%d", *source.UID, *source.GID))
	}

	return commandWithArgs
}

// isBuiltinRsyncCommand reports whether the command is still the built-in one,
// which needs no --rsync-path.
func isBuiltinRsyncCommand(command []string) bool {
	return slices.Equal(command, structdefault.List[attributes.Attributes]("RsyncCommand"))
}

// remoteRsyncPath renders the --rsync-path value: the elevation command followed
// by the configured rsync command.
//
// The remote rsync tokenizes this string itself (g_subprocess_argv_parse), so
// an element containing whitespace or quotes would split or corrupt the remote
// invocation. Elements are quoted only when they are not already one inert
// word, keeping the common `sudo rsync` form unchanged.
func remoteRsyncPath(maybeSudo, rsyncCommand []string) string {
	parts := slices.Concat(maybeSudo, rsyncCommand)

	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = shellquote.QuoteIfNeeded(part)
	}

	return strings.Join(quoted, " ")
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
