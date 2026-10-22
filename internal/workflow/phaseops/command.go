package phaseops

import (
	"crypto/rand"
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

// TransferSecret dispatches command sources to streaming and plain sources to
// rsync. bootstrapsNixOS selects the bootstrapping root; staging content passes
// false.
func TransferSecret(
	exc *executioner.Executioner,
	mach *machine.Machine,
	source attributes.TransferSource,
	transferOfWhat string,
	bootstrapsNixOS bool,
) error {
	if source.Command != "" {
		return TransferCommand(exc, mach, source, transferOfWhat, bootstrapsNixOS)
	}

	return TransferFile(exc, mach, source, transferOfWhat, bootstrapsNixOS)
}

// TransferCommand streams source.Command into the destination through ExecPipe.
// The probe skips unchanged content; the write stages privately and commits only
// after the source succeeds.
func TransferCommand(
	exc *executioner.Executioner,
	mach *machine.Machine,
	source attributes.TransferSource,
	transferOfWhat string,
	bootstrapsNixOS bool,
) error {
	return transferCommandWithStagingSuffix(
		exc, mach, source, transferOfWhat, bootstrapsNixOS, rand.Text(),
	)
}

// transferCommandWithStagingSuffix lets tests pin the staging suffix.
func transferCommandWithStagingSuffix(
	exc *executioner.Executioner,
	mach *machine.Machine,
	source attributes.TransferSource,
	transferOfWhat string,
	bootstrapsNixOS bool,
	stagingSuffix string,
) error {
	spec := transferCommandPipeSpecWithStagingSuffix(mach, source, bootstrapsNixOS, stagingSuffix)

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

// transferCommandPipeSpec renders the source, probe, write, commit and cleanup
// argv for a command-sourced transfer.
func transferCommandPipeSpec(
	mach *machine.Machine,
	source attributes.TransferSource,
	bootstrapsNixOS bool,
) executioner.PipeSpec {
	return transferCommandPipeSpecWithStagingSuffix(mach, source, bootstrapsNixOS, rand.Text())
}

// One random suffix per spec gives concurrent transfers distinct staging paths
// shared by Write, Commit and Cleanup.
func transferCommandPipeSpecWithStagingSuffix(
	mach *machine.Machine,
	source attributes.TransferSource,
	bootstrapsNixOS bool,
	stagingSuffix string,
) executioner.PipeSpec {
	remotePath := mach.MaybeBootstrappingPath(source.RemotePath, bootstrapsNixOS)
	stagingPath := transferCommandTempPath(remotePath, stagingSuffix)

	return executioner.PipeSpec{
		Source:        transferCommandSourceArgv(source),
		DisplaySource: []string{"command source", remotePath},
		Probe: slices.Concat(
			mach.MaybeSudo(),
			[]string{"sh", "-c", transferCommandProbeScript(remotePath, source)},
		),
		Write: slices.Concat(
			mach.MaybeSudo(),
			[]string{"sh", "-c", transferCommandScript(remotePath, stagingPath, source)},
		),
		Commit: slices.Concat(
			mach.MaybeSudo(),
			[]string{"sh", "-c", transferCommandCommitScript(remotePath, stagingPath)},
		),
		Cleanup: slices.Concat(
			mach.MaybeSudo(),
			[]string{"sh", "-c", transferCommandCleanupScript(stagingPath)},
		),
	}
}

// transferCommandSourceArgv builds the control-host argv; the env pair survives
// sudo, su -l and ssh re-parsing.
func transferCommandSourceArgv(source attributes.TransferSource) []string {
	command := []string{"sh", "-u", "-c", source.Command}
	if source.LocalPath == "" {
		return command
	}

	return executioner.WithEnv([]string{runtimevars.SecretLocalPath + "=" + source.LocalPath}, command)
}

// transferCommandTempPath stages beside the destination with a per-transfer
// suffix; the separate commit runs only after the source succeeds.
func transferCommandTempPath(remotePath, stagingSuffix string) string {
	return path.Join(path.Dir(remotePath), ".panix-secret."+path.Base(remotePath)+"."+stagingSuffix+".tmp")
}

// The write stages under a noclobber path, rejects symlink and directory
// destinations, and keeps the bytes private until chown/chmod. The -h metadata
// flags never follow a swapped symlink.
func transferCommandScript(remotePath, stagingPath string, source attributes.TransferSource) string {
	quotedPath := shellquote.Quote(remotePath)
	// Remote paths are POSIX even when the control host is not.
	quotedDir := shellquote.Quote(path.Dir(remotePath))
	quotedTemp := shellquote.Quote(stagingPath)

	lines := []string{
		"set -e",
		"umask 077",
		"mkdir -p -- " + quotedDir,
		"if [ -L " + quotedPath + " ]; then",
		"  echo " + shellquote.Quote("refusing to write through symlink: "+remotePath) + " >&2",
		"  exit 1",
		"fi",
		"if [ -d " + quotedPath + " ]; then",
		"  echo " + shellquote.Quote("refusing to write to directory: "+remotePath) + " >&2",
		"  exit 1",
		"fi",
		"tmp=" + quotedTemp,
		"rm -f -- \"$tmp\"",
		"set -C",
		"cat > \"$tmp\"",
		"if [ ! -f \"$tmp\" ] || [ -L \"$tmp\" ]; then",
		"  echo " + shellquote.Quote("refusing non-regular staged file: "+stagingPath) + " >&2",
		"  exit 1",
		"fi",
	}

	// chown before chmod: chown clears setuid/setgid bits.
	if source.UID != nil && source.GID != nil {
		lines = append(lines, fmt.Sprintf("chown -h %d:%d -- \"$tmp\"", *source.UID, *source.GID))
	}

	lines = append(lines, "chmod -h "+source.Permissions.String()+" -- \"$tmp\"")

	return shellScript(lines...)
}

// The commit renames the staged file into place, re-checks the destination, and
// prefers mv -T so a symlink-to-directory cannot receive the payload.
func transferCommandCommitScript(remotePath, stagingPath string) string {
	quotedPath := shellquote.Quote(remotePath)
	quotedTemp := shellquote.Quote(stagingPath)

	return shellScript(
		"set -e",
		"tmp="+quotedTemp,
		"trap 'rm -f -- \"$tmp\"' EXIT HUP INT TERM",
		"if [ -L "+quotedPath+" ]; then",
		"  echo "+shellquote.Quote("refusing to replace symlink: "+remotePath)+" >&2",
		"  exit 1",
		"fi",
		"if [ -d "+quotedPath+" ]; then",
		"  echo "+shellquote.Quote("refusing to replace directory: "+remotePath)+" >&2",
		"  exit 1",
		"fi",
		"case \"$(mv --help 2>&1 || true)\" in",
		"  *-T*)",
		"    mv -T -- \"$tmp\" "+quotedPath,
		"    ;;",
		"  *)",
		"    mv -f -- \"$tmp\" "+quotedPath,
		"    ;;",
		"esac",
		"trap - EXIT HUP INT TERM",
	)
}

// The cleanup removes only this transfer's staging path.
func transferCommandCleanupScript(stagingPath string) string {
	return shellScript("rm -f -- " + shellquote.Quote(stagingPath))
}

// The probe is read-only: it rejects symlinks and directories and prints a hash
// only when the destination already has the configured owner and mode.
func transferCommandProbeScript(remotePath string, source attributes.TransferSource) string {
	quotedPath := shellquote.Quote(remotePath)
	quotedPerm := shellquote.Quote(source.Permissions.StatString())

	lines := []string{
		"set -e",
		"if [ -L " + quotedPath + " ]; then",
		"  echo " + shellquote.Quote("refusing to read through symlink: "+remotePath) + " >&2",
		"  exit 1",
		"fi",
		"if [ -d " + quotedPath + " ]; then",
		"  echo " + shellquote.Quote("refusing to read directory: "+remotePath) + " >&2",
		"  exit 1",
		"fi",
		"if [ -f " + quotedPath + " ] && [ ! -L " + quotedPath + " ]; then",
	}

	if source.UID != nil && source.GID != nil {
		quotedOwner := shellquote.Quote(fmt.Sprintf("%d:%d", *source.UID, *source.GID))
		lines = append(lines,
			"  [ \"$(stat -c '%u:%g' -- "+quotedPath+")\" = "+quotedOwner+" ] || exit 0",
		)
	}

	lines = append(lines,
		"  [ \"$(stat -c '%a' -- "+quotedPath+")\" = "+quotedPerm+" ] || exit 0",
		"  sha256sum -- "+quotedPath,
		"fi",
	)

	return shellScript(lines...)
}

// shellScript joins lines with newlines and keeps the trailing newline.
func shellScript(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}
