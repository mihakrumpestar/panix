package phaseops

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/runtimevars"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/pkg/errors"
)

// TransferSecret sends a config-declared source: command sources stream, plain sources rsync.
func TransferSecret(
	exc *executioner.Executioner,
	mach *machine.Machine,
	source attributes.TransferSource,
	transferOfWhat string,
	transferOSSecrets bool,
) error {
	if source.Command != "" {
		return TransferCommand(exc, mach, source, transferOfWhat, transferOSSecrets)
	}

	return TransferFile(exc, mach, source, transferOfWhat, transferOSSecrets)
}

// TransferCommand streams source.Command's stdout into the final destination
// path through ExecPipe: the probe skips the write when the content is
// unchanged while still enforcing ownership and mode, so an unchanged secret
// keeps its inode and mtime. sh -u -c makes unset variable references fail
// loudly, LocalPath is exported as PANIX_SECRET_LOCAL_PATH only when set, and
// transferOSSecrets targets the bootstrapping root. Elevation rides on the
// destination argv for local and remote machines alike.
func TransferCommand(
	exc *executioner.Executioner,
	mach *machine.Machine,
	source attributes.TransferSource,
	transferOfWhat string,
	transferOSSecrets bool,
) error {
	spec := transferCommandPipeSpec(mach, source, transferOSSecrets)

	err := exc.ExecPipe(
		"transfer of "+transferOfWhat,
		"transferring "+transferOfWhat,
		transferOfWhat+" transfer failed",
		spec,
	)
	if err != nil {
		return errors.Wrap(err, "transfer failed")
	}

	return nil
}

// transferCommandPipeSpec renders the source, probe and write argv for a command-sourced transfer.
func transferCommandPipeSpec(
	mach *machine.Machine,
	source attributes.TransferSource,
	transferOSSecrets bool,
) executioner.PipeSpec {
	remotePath := source.RemotePath
	if transferOSSecrets {
		remotePath = mach.MaybeBootstrappingPath(source.RemotePath)
	}

	return executioner.PipeSpec{
		Source: transferCommandSourceArgv(source),
		Probe: slices.Concat(
			mach.MaybeSudo(),
			[]string{"sh", "-c", transferCommandProbeScript(remotePath, source)},
		),
		Write: slices.Concat(
			mach.MaybeSudo(),
			[]string{"sh", "-c", transferCommandScript(remotePath, source)},
		),
	}
}

// transferCommandSourceArgv builds the control-host argv; the env pair survives sudo, su -l and ssh re-parsing.
func transferCommandSourceArgv(source attributes.TransferSource) []string {
	command := []string{"sh", "-u", "-c", source.Command}
	if source.LocalPath == "" {
		return command
	}

	return WithEnv([]string{runtimevars.SecretLocalPath + "=" + source.LocalPath}, command)
}

// transferCommandScript returns the destination sh script: set -e aborts on
// the first failure, umask 077 keeps the streamed bytes private until chmod,
// and chown runs before chmod because chown clears setuid/setgid bits.
func transferCommandScript(remotePath string, source attributes.TransferSource) string {
	var script strings.Builder

	script.WriteString("set -e\n")
	script.WriteString("umask 077\n")
	// Remote paths are POSIX, so path.Dir is the right split on any control host.
	script.WriteString("mkdir -p -- " + shellquote.Quote(path.Dir(remotePath)) + "\n")
	script.WriteString("cat > " + shellquote.Quote(remotePath) + "\n")

	if source.UID != nil && source.GID != nil {
		fmt.Fprintf(&script, "chown %d:%d -- %s\n", *source.UID, *source.GID, shellquote.Quote(remotePath))
	}

	script.WriteString("chmod " + source.Permissions.String() + " -- " + shellquote.Quote(remotePath) + "\n")

	return script.String()
}

// transferCommandProbeScript returns the probe sh script: the owner line runs
// before the mode line for the same setuid/setgid reason; a missing file
// prints nothing (update), and the sha256sum output drives the skip.
func transferCommandProbeScript(remotePath string, source attributes.TransferSource) string {
	quotedPath := shellquote.Quote(remotePath)
	quotedPerm := shellquote.Quote(source.Permissions.String())

	var script strings.Builder

	script.WriteString("set -e\n")
	script.WriteString("if [ -e " + quotedPath + " ]; then\n")

	if source.UID != nil && source.GID != nil {
		quotedOwner := shellquote.Quote(fmt.Sprintf("%d:%d", *source.UID, *source.GID))
		fmt.Fprintf(&script, "  [ \"$(stat -c '%%u:%%g' -- %s)\" = %s ] || chown %s -- %s\n",
			quotedPath, quotedOwner, quotedOwner, quotedPath)
	}

	fmt.Fprintf(&script, "  [ \"$(stat -c '%%a' -- %s)\" = %s ] || chmod %s -- %s\n",
		quotedPath, quotedPerm, quotedPerm, quotedPath)

	script.WriteString("  sha256sum -- " + quotedPath + "\n")
	script.WriteString("fi\n")

	return script.String()
}
