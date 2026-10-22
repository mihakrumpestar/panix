package phaseops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandPayload is deliberately hostile to text processing: NUL, ESC, high
// bytes, tab, CR and no trailing newline.
func commandPayload() []byte {
	return []byte{
		0x00, 'P', 'A', 'N', 'I', 'X',
		0x1b, '[', '3', '1', 'm', 0x80, 0xff, 0xfe, 0x01, 0x7f,
		'\t', '\r', '\n',
	}
}

// printfPayloadCommand renders a POSIX sh command that writes payload to
// stdout. NUL bytes cannot travel through argv, so the payload is expressed as
// printf octal escapes inside the command text.
func printfPayloadCommand(payload []byte) string {
	var command strings.Builder

	command.WriteString(`printf '%b' '`)

	for _, octet := range payload {
		fmt.Fprintf(&command, `\%03o`, octet)
	}

	command.WriteString(`'`)

	return command.String()
}

// readCommandTestFile reads a file the test itself created (the path always
// comes from t.TempDir()) and fails the test on any error.
func readCommandTestFile(t *testing.T, path string) []byte {
	t.Helper()

	// #nosec G304 -- the path is a test-owned t.TempDir() path, not user input
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}

// commandTestModTime returns the modification time of a test-owned path.
func commandTestModTime(t *testing.T, path string) time.Time {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)

	return info.ModTime()
}

// assertCommandTestMode pins the permission bits of a test-owned path.
func assertCommandTestMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm())
}

type transferCommandScriptCase struct {
	name    string
	staging string
	source  attributes.TransferSource
	want    string
}

//nolint:funlen // table fixture
func transferCommandScriptCases() []transferCommandScriptCase {
	uid := uint(1000)
	gid := uint(100)

	return []transferCommandScriptCase{
		{
			name:    "no ownership defaults to 0700",
			staging: "/var/secrets/.panix-secret.key.suffix.tmp",
			source:  attributes.TransferSource{RemotePath: "/var/secrets/key"},
			want: "set -e\numask 077\nmkdir -p -- '/var/secrets'\n" +
				"if [ -L '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to write through symlink: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -d '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to write to directory: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"tmp='/var/secrets/.panix-secret.key.suffix.tmp'\n" +
				"rm -f -- \"$tmp\"\n" +
				"set -C\n" +
				"cat > \"$tmp\"\n" +
				"if [ ! -f \"$tmp\" ] || [ -L \"$tmp\" ]; then\n" +
				"  echo 'refusing non-regular staged file: /var/secrets/.panix-secret.key.suffix.tmp' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"chmod -h 0700 -- \"$tmp\"\n",
		},
		{
			name:    "uid and gid chown between cat and chmod",
			staging: "/var/secrets/.panix-secret.key.suffix.tmp",
			source: attributes.TransferSource{
				RemotePath:  "/var/secrets/key",
				UID:         &uid,
				GID:         &gid,
				Permissions: attributes.FileMode(0o640),
			},
			want: "set -e\numask 077\nmkdir -p -- '/var/secrets'\n" +
				"if [ -L '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to write through symlink: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -d '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to write to directory: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"tmp='/var/secrets/.panix-secret.key.suffix.tmp'\n" +
				"rm -f -- \"$tmp\"\n" +
				"set -C\n" +
				"cat > \"$tmp\"\n" +
				"if [ ! -f \"$tmp\" ] || [ -L \"$tmp\" ]; then\n" +
				"  echo 'refusing non-regular staged file: /var/secrets/.panix-secret.key.suffix.tmp' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"chown -h 1000:100 -- \"$tmp\"\n" +
				"chmod -h 0640 -- \"$tmp\"\n",
		},
		{
			name:    "spaces and single quotes stay one shell word",
			staging: "/var/my secrets/.panix-secret.it's a key.suffix.tmp",
			source:  attributes.TransferSource{RemotePath: "/var/my secrets/it's a key"},
			want: "set -e\numask 077\nmkdir -p -- '/var/my secrets'\n" +
				"if [ -L " + quotedTrickyPath + " ]; then\n" +
				"  echo 'refusing to write through symlink: /var/my secrets/it'\\''s a key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -d " + quotedTrickyPath + " ]; then\n" +
				"  echo 'refusing to write to directory: /var/my secrets/it'\\''s a key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"tmp=" + quotedTrickyTemp + "\n" +
				"rm -f -- \"$tmp\"\n" +
				"set -C\n" +
				"cat > \"$tmp\"\n" +
				"if [ ! -f \"$tmp\" ] || [ -L \"$tmp\" ]; then\n" +
				"  echo 'refusing non-regular staged file: /var/my secrets/.panix-secret.it'\\''s a key.suffix.tmp' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"chmod -h 0700 -- \"$tmp\"\n",
		},
	}
}

// TestTransferCommandScript pins the exact destination script: step order, the
// configured ownership and mode, noclobber staging, the post-write regular-file
// check, and shellquoting of space and quote bearing paths.
func TestTransferCommandScript(t *testing.T) {
	t.Parallel()

	for _, tt := range transferCommandScriptCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, transferCommandScript(tt.source.RemotePath, tt.staging, tt.source))
		})
	}
}

// quotedTrickyPath is shellquote.Quote("/var/my secrets/it's a key"): spaces
// and a single quote stay one literal shell word.
const quotedTrickyPath = `'/var/my secrets/it'\''s a key'`

// quotedTrickyTemp is the matching staging path quote.
const quotedTrickyTemp = `'/var/my secrets/.panix-secret.it'\''s a key.suffix.tmp'`

type transferCommandProbeScriptCase struct {
	name   string
	source attributes.TransferSource
	want   string
}

//nolint:funlen // table fixture
func transferCommandProbeScriptCases() []transferCommandProbeScriptCase {
	uid := uint(1000)
	gid := uint(100)

	return []transferCommandProbeScriptCase{
		{
			name:   "no ownership enforces the default mode only",
			source: attributes.TransferSource{RemotePath: "/var/secrets/key"},
			want: "set -e\n" +
				"if [ -L '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to read through symlink: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -d '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to read directory: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -f '/var/secrets/key' ] && [ ! -L '/var/secrets/key' ]; then\n" +
				"  [ \"$(stat -c '%a' -- '/var/secrets/key')\" = '700' ] || exit 0\n" +
				"  sha256sum -- '/var/secrets/key'\n" +
				"fi\n",
		},
		{
			name: "owner line runs before the mode line",
			source: attributes.TransferSource{
				RemotePath:  "/var/secrets/key",
				UID:         &uid,
				GID:         &gid,
				Permissions: attributes.FileMode(0o640),
			},
			want: "set -e\n" +
				"if [ -L '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to read through symlink: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -d '/var/secrets/key' ]; then\n" +
				"  echo 'refusing to read directory: /var/secrets/key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -f '/var/secrets/key' ] && [ ! -L '/var/secrets/key' ]; then\n" +
				"  [ \"$(stat -c '%u:%g' -- '/var/secrets/key')\" = '1000:100' ] || exit 0\n" +
				"  [ \"$(stat -c '%a' -- '/var/secrets/key')\" = '640' ] || exit 0\n" +
				"  sha256sum -- '/var/secrets/key'\n" +
				"fi\n",
		},
		{
			name:   "spaces and single quotes stay one shell word",
			source: attributes.TransferSource{RemotePath: "/var/my secrets/it's a key"},
			want: "set -e\n" +
				"if [ -L " + quotedTrickyPath + " ]; then\n" +
				"  echo 'refusing to read through symlink: /var/my secrets/it'\\''s a key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -d " + quotedTrickyPath + " ]; then\n" +
				"  echo 'refusing to read directory: /var/my secrets/it'\\''s a key' >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"if [ -f " + quotedTrickyPath + " ] && [ ! -L " + quotedTrickyPath + " ]; then\n" +
				"  [ \"$(stat -c '%a' -- " + quotedTrickyPath + ")\" = '700' ] || exit 0\n" +
				"  sha256sum -- " + quotedTrickyPath + "\n" +
				"fi\n",
		},
	}
}

// TestTransferCommandProbeScript pins the exact read-only probe script: symlink
// and directory rejection, the owner/mode gates that print nothing on mismatch,
// the sha256sum output, and shellquoting.
func TestTransferCommandProbeScript(t *testing.T) {
	t.Parallel()

	for _, tt := range transferCommandProbeScriptCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, transferCommandProbeScript(tt.source.RemotePath, tt.source))
		})
	}
}

// TestTransferCommandCommitAndCleanupScript pins the commit script's atomic
// rename, the GNU -T feature probe with its portable fallback, and the cleanup
// script's per-spec staging path.
func TestTransferCommandCommitAndCleanupScript(t *testing.T) {
	t.Parallel()

	const staging = "/var/secrets/.panix-secret.key.suffix.tmp"

	wantCommit := "set -e\n" +
		"tmp='" + staging + "'\n" +
		"trap 'rm -f -- \"$tmp\"' EXIT HUP INT TERM\n" +
		"if [ -L '/var/secrets/key' ]; then\n" +
		"  echo 'refusing to replace symlink: /var/secrets/key' >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"if [ -d '/var/secrets/key' ]; then\n" +
		"  echo 'refusing to replace directory: /var/secrets/key' >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"case \"$(mv --help 2>&1 || true)\" in\n" +
		"  *-T*)\n" +
		"    mv -T -- \"$tmp\" '/var/secrets/key'\n" +
		"    ;;\n" +
		"  *)\n" +
		"    mv -f -- \"$tmp\" '/var/secrets/key'\n" +
		"    ;;\n" +
		"esac\n" +
		"trap - EXIT HUP INT TERM\n"

	assert.Equal(t, wantCommit, transferCommandCommitScript("/var/secrets/key", staging))
	assert.Equal(t, "rm -f -- '"+staging+"'\n", transferCommandCleanupScript(staging))
}

// stagingPathFromScript extracts the shell-quoted tmp= assignment from a
// rendered write or commit script. Scripts under test only use simple paths.
func stagingPathFromScript(t *testing.T, script string) string {
	t.Helper()

	const prefix = "tmp='"

	start := strings.Index(script, prefix)
	require.NotEqual(t, -1, start, "script must assign tmp")

	rest := script[start+len(prefix):]

	end := strings.Index(rest, "'\n")
	require.NotEqual(t, -1, end, "tmp assignment must be shell-quoted")

	return rest[:end]
}

// runTransferCommandProbeScript runs a probe script directly, returning stdout
// and the process error for assertions.
func runTransferCommandProbeScript(t *testing.T, script string) (string, error) {
	t.Helper()

	output, err := exec.CommandContext(t.Context(), "sh", "-c", script).Output() //nolint:gosec // script is the unit under test
	if err != nil {
		return string(output), err
	}

	return string(output), nil
}

// TestTransferCommandProbeScript_IsReadOnly pins that a metadata mismatch
// prints no hash and leaves the destination's content, mode and mtime intact:
// repair belongs to the write+commit path.
func TestTransferCommandProbeScript_IsReadOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	payload := []byte("existing-secret")
	require.NoError(t, os.WriteFile(path, payload, 0o644)) //nolint:gosec // intentionally loose mode under test

	backdated := time.Now().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, os.Chtimes(path, backdated, backdated))

	source := attributes.TransferSource{RemotePath: path, Permissions: attributes.FileMode(0o600)}

	output, err := runTransferCommandProbeScript(t, transferCommandProbeScript(path, source))
	require.NoError(t, err)
	assert.Empty(t, output, "a metadata mismatch must print no hash so the write+commit repairs it")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "the probe must not chmod the destination")
	assert.Equal(t, backdated, info.ModTime(), "the probe must not touch the destination")
	assert.Equal(t, payload, readCommandTestFile(t, path))
}

// TestTransferCommandProbeScript_RejectsSymlink pins that a symlink is rejected
// before any stat/sha256sum, so the target is never read or modified even when
// its metadata would otherwise match.
func TestTransferCommandProbeScript_RejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	payload := []byte("keep me")
	require.NoError(t, os.WriteFile(victim, payload, 0o600))

	destination := filepath.Join(dir, "key")
	require.NoError(t, os.Symlink(victim, destination))

	source := attributes.TransferSource{RemotePath: destination, Permissions: attributes.FileMode(0o600)}

	output, err := runTransferCommandProbeScript(t, transferCommandProbeScript(destination, source))
	require.Error(t, err)
	assert.Empty(t, output, "a symlink must never be followed or hashed")

	info, err := os.Stat(victim)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.Equal(t, payload, readCommandTestFile(t, victim))
}

// TestTransferCommandPipeSpecElevation pins elevation on the destination
// argv: sudo only for non-root, always sh -c running the script.
func TestTransferCommandPipeSpecElevation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		isRoot   bool
		wantSudo bool
	}{
		{name: "root runs sh -c without elevation", isRoot: true},
		{name: "non-root runs sudo sh -c", isRoot: false, wantSudo: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newLocalTransferMachine(t, tt.isRoot)
			source := attributes.TransferSource{Command: "printf secret", RemotePath: "/var/secrets/key"}

			plan := transferCommandPipeSpec(mach, source, false)

			assert.Equal(t, []string{"sh", "-u", "-c", "printf secret"}, plan.Source)

			script := assertShScriptArgv(t, plan.Write, tt.wantSudo)
			staging := stagingPathFromScript(t, script)

			assert.Contains(t, staging, "/var/secrets/.panix-secret.key.")
			assert.True(t, strings.HasSuffix(staging, ".tmp"), "staging path must keep the tmp suffix")
			assert.Contains(t, script, "set -C")
			assert.Contains(t, script, "chmod -h 0700 -- \"$tmp\"")
			assert.NotContains(t, script, "mv -f", "the write stages only; commit renames")

			commitScript := assertShScriptArgv(t, plan.Commit, tt.wantSudo)
			assert.Contains(t, commitScript, "tmp='"+staging+"'")
			assert.Contains(t, commitScript, "mv -T -- \"$tmp\" '/var/secrets/key'")
			assert.Contains(t, commitScript, "mv -f -- \"$tmp\" '/var/secrets/key'")

			cleanupScript := assertShScriptArgv(t, plan.Cleanup, tt.wantSudo)
			assert.Equal(t, "rm -f -- '"+staging+"'\n", cleanupScript)
		})
	}
}

// TestTransferCommandPipeSpec_UniqueStagingPath pins that every PipeSpec gets
// its own staging name while Write, Commit and Cleanup within one spec agree.
func TestTransferCommandPipeSpec_UniqueStagingPath(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	source := attributes.TransferSource{Command: "printf secret", RemotePath: "/var/secrets/key"}

	first := transferCommandPipeSpec(mach, source, false)
	second := transferCommandPipeSpec(mach, source, false)

	firstStaging := stagingPathFromScript(t, assertShScriptArgv(t, first.Write, false))
	secondStaging := stagingPathFromScript(t, assertShScriptArgv(t, second.Write, false))

	assert.NotEqual(t, firstStaging, secondStaging,
		"two transfers to the same destination must not share a staging path")

	assert.Contains(t, assertShScriptArgv(t, first.Commit, false), "tmp='"+firstStaging+"'")
	assert.Equal(t, "rm -f -- '"+firstStaging+"'\n", assertShScriptArgv(t, first.Cleanup, false))
	assert.Contains(t, assertShScriptArgv(t, second.Commit, false), "tmp='"+secondStaging+"'")
	assert.Equal(t, "rm -f -- '"+secondStaging+"'\n", assertShScriptArgv(t, second.Cleanup, false))
}

// TestTransferCommandPipeSpecBootstrapping pins the /mnt prefix on the
// destination script: NixOS-bootstrapped content is prefixed until the
// machine is bootstrapped, staging content (and every other bootstrap mode)
// never is.
func TestTransferCommandPipeSpecBootstrapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		bootstrapped    bool
		bootstrapsNixOS bool
		wantScript      string
		wantNoScript    string
	}{
		{
			name:            "nixos-bootstrapped content gets the bootstrapping prefix",
			bootstrapsNixOS: true,
			wantScript:      "mv -T -- \"$tmp\" '/mnt/var/secrets/key'",
		},
		{
			name:            "nixos-bootstrapped content drops the prefix once bootstrapped",
			bootstrapped:    true,
			bootstrapsNixOS: true,
			wantScript:      "mv -T -- \"$tmp\" '/var/secrets/key'",
			wantNoScript:    "/mnt",
		},
		{
			name:         "staging transfers never get the prefix",
			wantScript:   "mv -T -- \"$tmp\" '/var/secrets/key'",
			wantNoScript: "/mnt",
		},
		{
			name:            "non-NixOS bootstrap modes never get the prefix",
			bootstrapsNixOS: false,
			wantScript:      "mv -T -- \"$tmp\" '/var/secrets/key'",
			wantNoScript:    "/mnt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newLocalTransferMachine(t, true)
			mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: true, Bootstrapped: tt.bootstrapped})

			source := attributes.TransferSource{Command: "printf secret", RemotePath: "/var/secrets/key"}
			plan := transferCommandPipeSpec(mach, source, tt.bootstrapsNixOS)

			script := assertShScriptArgv(t, plan.Commit, false)
			assert.Contains(t, script, tt.wantScript)

			if tt.wantNoScript != "" {
				assert.NotContains(t, script, tt.wantNoScript)
			}
		})
	}
}

// TestTransferCommandPipeSpecProbe pins probe argv routing: the same
// elevation and bootstrapping prefix rules as the write argv, plus the
// sha256sum marker.
func TestTransferCommandPipeSpecProbe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		isRoot          bool
		bootstrapped    bool
		bootstrapsNixOS bool
		wantSudo        bool
		wantScript      string
	}{
		{
			name:       "root runs the probe directly",
			isRoot:     true,
			wantScript: "sha256sum -- '/var/secrets/key'",
		},
		{
			name:       "non-root elevates the probe",
			isRoot:     false,
			wantSudo:   true,
			wantScript: "sha256sum -- '/var/secrets/key'",
		},
		{
			name:            "nixos-bootstrapped content probes the bootstrapping root",
			isRoot:          true,
			bootstrapsNixOS: true,
			wantScript:      "sha256sum -- '/mnt/var/secrets/key'",
		},
		{
			name:            "bootstrapped nixos content probes the final root",
			isRoot:          true,
			bootstrapped:    true,
			bootstrapsNixOS: true,
			wantScript:      "sha256sum -- '/var/secrets/key'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newLocalTransferMachine(t, tt.isRoot)
			mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: tt.isRoot, Bootstrapped: tt.bootstrapped})

			source := attributes.TransferSource{Command: "printf secret", RemotePath: "/var/secrets/key"}
			plan := transferCommandPipeSpec(mach, source, tt.bootstrapsNixOS)

			script := assertShScriptArgv(t, plan.Probe, tt.wantSudo)
			assert.Contains(t, script, tt.wantScript)
		})
	}
}

// assertShScriptArgv pins a destination argv shape: sh -c always runs the
// script, and sudo is present exactly for non-root machines. It returns the
// script for content assertions.
func assertShScriptArgv(t *testing.T, argv []string, wantSudo bool) string {
	t.Helper()

	script := argv[len(argv)-1]
	assert.Equal(t, []string{"sh", "-c", script}, argv[len(argv)-3:])

	if wantSudo {
		assert.Equal(t, []string{"sudo"}, argv[:len(argv)-3])

		return script
	}

	assert.Empty(t, argv[:len(argv)-3], "elevation must be absent")

	return script
}

// TestTransferCommandSourceArgv pins the control-host argv: sh -u -c with the
// raw command, and PANIX_SECRET_LOCAL_PATH exported only when local_path is
// set.
func TestTransferCommandSourceArgv(t *testing.T) {
	t.Parallel()

	const command = `printf %s "$PANIX_SECRET_LOCAL_PATH"`

	assert.Equal(t,
		[]string{"sh", "-u", "-c", command},
		transferCommandSourceArgv(attributes.TransferSource{Command: command}),
	)

	assert.Equal(t,
		[]string{"env", "PANIX_SECRET_LOCAL_PATH=/tmp/age.key", "sh", "-u", "-c", command},
		transferCommandSourceArgv(attributes.TransferSource{Command: command, LocalPath: "/tmp/age.key"}),
	)
}

// TestTransferCommand_LocalStreamsBinaryAndAppliesMode runs a real local
// transfer: the binary payload must survive byte for byte and the configured
// mode must beat the umask.
func TestTransferCommand_LocalStreamsBinaryAndAppliesMode(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	payload := commandPayload()
	destination := filepath.Join(t.TempDir(), "secret.bin")

	source := attributes.TransferSource{
		Command:     printfPayloadCommand(payload),
		RemotePath:  destination,
		Permissions: attributes.FileMode(0o640),
	}

	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))

	assert.Equal(t, payload, readCommandTestFile(t, destination), "payload must survive the pipe byte for byte")

	info, err := os.Stat(destination)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm(),
		"final chmod must apply the configured mode, not the umask")
}

// TestTransferCommand_LocalPathEnvInjection pins that local_path reaches the
// command as PANIX_SECRET_LOCAL_PATH.
func TestTransferCommand_LocalPathEnvInjection(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	localPath := filepath.Join(t.TempDir(), "age.key")
	destination := filepath.Join(t.TempDir(), "secret.bin")

	source := attributes.TransferSource{
		LocalPath:  localPath,
		Command:    `printf %s "$PANIX_SECRET_LOCAL_PATH"`,
		RemotePath: destination,
	}

	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))

	assert.Equal(t, localPath, string(readCommandTestFile(t, destination)))
}

// TestTransferCommand_MissingLocalPathFailsLoudly pins sh -u: referencing
// PANIX_SECRET_LOCAL_PATH without local_path must fail the transfer instead of
// streaming an empty secret.
func TestTransferCommand_MissingLocalPathFailsLoudly(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	source := attributes.TransferSource{
		Command:    `printf %s "$PANIX_SECRET_LOCAL_PATH"`,
		RemotePath: filepath.Join(t.TempDir(), "secret.bin"),
	}

	err := TransferCommand(exc, mach, source, "secrets", false)
	require.Error(t, err)
	require.ErrorContains(t, err, "source command failed")
	require.ErrorContains(t, err, "PANIX_SECRET_LOCAL_PATH")
}

// TestTransferCommand_ConditionalWrite runs the real transfer repeatedly: an
// identical run must skip the write (mtime preserved), a permissions-only
// change must still be enforced on the skip path, and changed content must
// update the file. The mtime is backdated so the assertion cannot pass by
// timestamp coincidence.
func TestTransferCommand_ConditionalWrite(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	destination := filepath.Join(t.TempDir(), "secret.bin")
	payloadA := commandPayload()
	payloadB := []byte("changed payload")
	backdated := time.Now().Add(-time.Hour).Truncate(time.Second)

	source := attributes.TransferSource{
		Command:     printfPayloadCommand(payloadA),
		RemotePath:  destination,
		Permissions: attributes.FileMode(0o600),
	}

	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))
	assert.Equal(t, payloadA, readCommandTestFile(t, destination))
	assertCommandTestMode(t, destination, 0o600)

	require.NoError(t, os.Chtimes(destination, backdated, backdated))

	// Identical content: the write must be skipped, keeping the mtime.
	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))
	assert.Equal(t, payloadA, readCommandTestFile(t, destination))
	assert.Equal(t, backdated, commandTestModTime(t, destination), "unchanged content must keep the mtime")

	// Metadata-only change: the read-only probe prints no hash, so the
	// write+commit path repairs the metadata.
	source.Permissions = attributes.FileMode(0o640)
	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))
	assert.Equal(t, payloadA, readCommandTestFile(t, destination))
	assert.True(t, commandTestModTime(t, destination).After(backdated),
		"a metadata mismatch must be repaired by the write+commit path")
	assertCommandTestMode(t, destination, 0o640)

	// Changed content: the file is rewritten and keeps the enforced mode.
	source.Command = printfPayloadCommand(payloadB)
	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))
	assert.Equal(t, payloadB, readCommandTestFile(t, destination))
	assert.True(t, commandTestModTime(t, destination).After(backdated), "changed content must rewrite the file")
	assertCommandTestMode(t, destination, 0o640)
}

// TestTransferCommand_RefusesSymlinkDestination pins that a symlink planted at
// the destination is rejected by both the probe and the write script, so the
// file it points at is never clobbered.
func TestTransferCommand_RefusesSymlinkDestination(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("keep me"), 0o600))

	destination := filepath.Join(dir, "secret.bin")
	require.NoError(t, os.Symlink(victim, destination))

	source := attributes.TransferSource{
		Command:     "printf replacement",
		RemotePath:  destination,
		Permissions: attributes.FileMode(0o600),
	}

	err := TransferCommand(exc, mach, source, "secrets", false)
	require.Error(t, err)
	require.ErrorContains(t, err, "symlink")

	assert.Equal(t, []byte("keep me"), readCommandTestFile(t, victim),
		"the symlink target must never be written")
	assertCommandTestMode(t, victim, 0o600)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "the refused transfer must not leave a temp file behind")
}

// TestTransferCommand_CleansTempFileOnFailure pins the trap: a failure after
// the payload is written removes the temp file and never creates the final
// destination. An impossible numeric owner makes the failure deterministic.
func TestTransferCommand_CleansTempFileOnFailure(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	dir := t.TempDir()
	destination := filepath.Join(dir, "secret.bin")
	invalid := uint(4294967295)

	source := attributes.TransferSource{
		Command:    "printf secret",
		RemotePath: destination,
		UID:        &invalid,
		GID:        &invalid,
	}

	err := TransferCommand(exc, mach, source, "secrets", false)
	require.Error(t, err)

	_, err = os.Stat(destination)
	require.ErrorIs(t, err, os.ErrNotExist, "a failed transfer must not leave a final file")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the trap must remove the temp file")
}

// TestTransferCommand_StreamedPayloadLandsAtomically covers the >4 MiB stream
// path: the payload still lands byte for byte, with the final mode, and only
// the destination remains in the directory.
func TestTransferCommand_StreamedPayloadLandsAtomically(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	const streamSize = (4 << 20) + 1024

	dir := t.TempDir()
	destination := filepath.Join(dir, "secret.bin")

	source := attributes.TransferSource{
		Command:     fmt.Sprintf("head -c %d /dev/zero | tr '\\0' 'A'", streamSize),
		RemotePath:  destination,
		Permissions: attributes.FileMode(0o600),
	}

	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))

	assert.Equal(t, bytes.Repeat([]byte("A"), streamSize), readCommandTestFile(t, destination))
	assertCommandTestMode(t, destination, 0o600)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the streamed write must leave only the final file")
}

// TestTransferCommand_StreamFailureLeavesNoPartialFile pins that the streamed
// path also lands in the temp file: a post-stream failure leaves neither a
// partial final file nor a temp file.
func TestTransferCommand_StreamFailureLeavesNoPartialFile(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	const streamSize = (4 << 20) + 1024

	dir := t.TempDir()
	destination := filepath.Join(dir, "secret.bin")
	invalid := uint(4294967295)

	source := attributes.TransferSource{
		Command:    fmt.Sprintf("head -c %d /dev/zero | tr '\\0' 'A'", streamSize),
		RemotePath: destination,
		UID:        &invalid,
		GID:        &invalid,
	}

	err := TransferCommand(exc, mach, source, "secrets", false)
	require.Error(t, err)

	_, err = os.Stat(destination)
	require.ErrorIs(t, err, os.ErrNotExist, "the streamed write must not land a partial final file")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the trap must remove the streamed temp file")
}

// TestTransferCommand_RefusesDirectoryDestination pins that a directory at the
// destination is rejected instead of having the secret moved inside it, and no
// staged file remains.
func TestTransferCommand_RefusesDirectoryDestination(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	dir := t.TempDir()
	destination := filepath.Join(dir, "secret.bin")
	require.NoError(t, os.Mkdir(destination, 0o700))

	source := attributes.TransferSource{
		Command:     "printf secret",
		RemotePath:  destination,
		Permissions: attributes.FileMode(0o600),
	}

	err := TransferCommand(exc, mach, source, "secrets", false)
	require.Error(t, err)
	require.ErrorContains(t, err, "directory")

	entries, err := os.ReadDir(destination)
	require.NoError(t, err)
	assert.Empty(t, entries, "the secret must not be moved inside the directory")

	assertNoStagedTransferFiles(t, dir)
}

// TestTransferCommand_RefusesSymlinkToDirectoryDestination pins that a symlink
// to a directory is rejected and nothing lands inside the target directory.
func TestTransferCommand_RefusesSymlinkToDirectoryDestination(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(targetDir, 0o700))

	destination := filepath.Join(dir, "secret.bin")
	require.NoError(t, os.Symlink(targetDir, destination))

	source := attributes.TransferSource{
		Command:     "printf secret",
		RemotePath:  destination,
		Permissions: attributes.FileMode(0o600),
	}

	err := TransferCommand(exc, mach, source, "secrets", false)
	require.Error(t, err)
	require.ErrorContains(t, err, "symlink")

	entries, err := os.ReadDir(targetDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the symlinked directory must stay empty")

	assertNoStagedTransferFiles(t, dir)
}

// TestTransferCommand_StreamedSourceFailureLeavesNothing pins the >4 MiB path:
// a source that emits its payload and then fails must not leave a partial file
// at the destination or a staged file behind.
func TestTransferCommand_StreamedSourceFailureLeavesNothing(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, _ := realExecutionerFor(t, mach)

	const streamSize = (4 << 20) + 1024

	dir := t.TempDir()
	destination := filepath.Join(dir, "secret.bin")

	source := attributes.TransferSource{
		Command:     fmt.Sprintf("head -c %d /dev/zero | tr '\\0' 'A'; exit 9", streamSize),
		RemotePath:  destination,
		Permissions: attributes.FileMode(0o600),
	}

	err := TransferCommand(exc, mach, source, "secrets", false)
	require.Error(t, err)
	require.ErrorContains(t, err, "source command failed")
	require.ErrorContains(t, err, "exit status 9")

	assert.NoFileExists(t, destination, "a failed source must never commit the staged payload")
	assertNoStagedTransferFiles(t, dir)
}

// assertNoStagedTransferFiles pins that no per-spec staging file survived.
func assertNoStagedTransferFiles(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".panix-secret.",
			"a failed transfer must remove its staged file")
	}
}

// TestTransferCommand_DryRunRecordsSourceAndSkipsExecution pins that dry-run
// records an opaque label and starts neither process.
func TestTransferCommand_DryRunRecordsSourceAndSkipsExecution(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

	destination := filepath.Join(t.TempDir(), "must-not-exist.bin")
	source := attributes.TransferSource{Command: "printf secret", RemotePath: destination}

	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))

	line := testutil.LastCommandLine(t, phaseLog)
	assert.Equal(t, "command source "+destination, line)
	assert.NotContains(t, line, "printf secret")
	assert.NotContains(t, line, "sh -u")

	_, err := os.Stat(destination)
	require.ErrorIs(t, err, os.ErrNotExist, "dry run must not start the destination")
}

// TestTransferCommand_StagingSymlinkNeverWritesVictim simulates an attacker
// winning the window between the stale rm and the write redirection. The rm
// wrapper plants the hostile entry after removing the stale path, so the real
// TransferCommand pipeline runs with a symlink already at the staging path;
// noclobber must fail closed and the victim must never receive the payload.
func TestTransferCommand_StagingSymlinkNeverWritesVictim(t *testing.T) {
	tests := []struct {
		name  string
		plant string
	}{
		{name: "symlink to victim", plant: `ln -s "$PANIX_TEST_VICTIM" "$last"`},
		{name: "symlink loop", plant: `ln -s "$last" "$last"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mach := newLocalTransferMachine(t, true)
			exc, _ := realExecutionerFor(t, mach)

			dir := t.TempDir()
			victim := filepath.Join(dir, "victim")
			require.NoError(t, os.WriteFile(victim, []byte("keep me"), 0o600))

			destination := filepath.Join(dir, "secret.bin")

			binDir := t.TempDir()
			rmStub := filepath.Join(binDir, "rm")
			rmPath, err := exec.LookPath("rm")
			require.NoError(t, err)

			wrapper := "#!/bin/sh\n" +
				"last=\n" +
				"for arg do last=$arg; done\n" +
				"'" + rmPath + "' -f -- \"$last\"\n" +
				"if [ ! -e \"$PANIX_TEST_RM_MARKER\" ]; then\n" +
				"  : > \"$PANIX_TEST_RM_MARKER\"\n" +
				"  " + tt.plant + "\n" +
				"fi\n" +
				"exit 0\n"
			//nolint:gosec // the stub must be executable to be found on PATH
			require.NoError(t, os.WriteFile(rmStub, []byte(wrapper), 0o700))

			t.Setenv("PANIX_TEST_VICTIM", victim)
			t.Setenv("PANIX_TEST_RM_MARKER", filepath.Join(binDir, "planted"))
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			source := attributes.TransferSource{
				Command:     "printf 'PANIX-PAYLOAD'",
				RemotePath:  destination,
				Permissions: attributes.FileMode(0o600),
			}

			err = transferCommandWithStagingSuffix(exc, mach, source, "secrets", false, "race")
			require.Error(t, err, "a planted staging symlink must fail the write closed")

			assert.Equal(t, []byte("keep me"), readCommandTestFile(t, victim),
				"the victim must never receive the payload")
			assert.NoFileExists(t, destination)
			assertNoStagedTransferFiles(t, dir)
		})
	}
}

// TestTransferCommand_CommitNeverMovesIntoSymlinkToDirectory carries the race
// past the commit script's -L check: an mv wrapper plants a symlink-to-directory
// at the destination immediately before the real mv runs. GNU mv -T must replace
// the symlink instead of moving the payload inside the directory it points at.
func TestTransferCommand_CommitNeverMovesIntoSymlinkToDirectory(t *testing.T) {
	mvPath, err := exec.LookPath("mv")
	require.NoError(t, err)

	help, err := exec.CommandContext(t.Context(), mvPath, "--help").CombinedOutput() //nolint:gosec // mvPath comes from LookPath
	if err != nil || !strings.Contains(string(help), "-T") {
		t.Skip("mv -T is not supported on this platform")
	}

	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(targetDir, 0o700))

	destination := filepath.Join(dir, "secret.bin")
	staging := transferCommandTempPath(destination, "race")
	require.NoError(t, os.WriteFile(staging, []byte("PANIX-PAYLOAD"), 0o600))

	binDir := t.TempDir()
	mvStub := filepath.Join(binDir, "mv")
	wrapper := "#!/bin/sh\n" +
		"case \" $* \" in\n" +
		"  *\" --help \"*) exec '" + mvPath + "' \"$@\";;\n" +
		"esac\n" +
		"if [ ! -e \"$PANIX_TEST_DEST\" ] && [ ! -L \"$PANIX_TEST_DEST\" ]; then\n" +
		"  ln -s \"$PANIX_TEST_TARGET\" \"$PANIX_TEST_DEST\"\n" +
		"fi\n" +
		"exec '" + mvPath + "' \"$@\"\n"
	//nolint:gosec // the stub must be executable to be found on PATH
	require.NoError(t, os.WriteFile(mvStub, []byte(wrapper), 0o700))

	t.Setenv("PANIX_TEST_DEST", destination)
	t.Setenv("PANIX_TEST_TARGET", targetDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err = exec.CommandContext(t.Context(), "sh", "-c", //nolint:gosec // script is the unit under test
		transferCommandCommitScript(destination, staging)).Run()
	require.NoError(t, err)

	entries, err := os.ReadDir(targetDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the payload must never be moved inside the symlinked directory")

	assert.Equal(t, []byte("PANIX-PAYLOAD"), readCommandTestFile(t, destination),
		"mv -T must replace the symlink with the staged payload")
	assert.NoFileExists(t, staging, "the commit trap must remove staged content")
}

// TestTransferCommand_PinnedStagingLeavesOnlyFinal pins the end states with a
// known staging name: success leaves only the final file, while a streamed
// failure leaves neither a partial final file nor a staged file.
func TestTransferCommand_PinnedStagingLeavesOnlyFinal(t *testing.T) {
	t.Parallel()

	const streamSize = (4 << 20) + 1024

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mach := newLocalTransferMachine(t, true)
		exc, _ := realExecutionerFor(t, mach)

		dir := t.TempDir()
		destination := filepath.Join(dir, "secret.bin")

		source := attributes.TransferSource{
			Command:     fmt.Sprintf("head -c %d /dev/zero | tr '\\0' 'A'", streamSize),
			RemotePath:  destination,
			Permissions: attributes.FileMode(0o600),
		}

		require.NoError(t, transferCommandWithStagingSuffix(exc, mach, source, "secrets", false, "race"))

		assert.Len(t, readCommandTestFile(t, destination), streamSize)
		assertNoStagedTransferFiles(t, dir)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "success must leave only the final file")
	})

	t.Run("streamed source failure", func(t *testing.T) {
		t.Parallel()

		mach := newLocalTransferMachine(t, true)
		exc, _ := realExecutionerFor(t, mach)

		dir := t.TempDir()
		destination := filepath.Join(dir, "secret.bin")

		source := attributes.TransferSource{
			Command:     fmt.Sprintf("head -c %d /dev/zero | tr '\\0' 'A'; exit 9", streamSize),
			RemotePath:  destination,
			Permissions: attributes.FileMode(0o600),
		}

		err := transferCommandWithStagingSuffix(exc, mach, source, "secrets", false, "race")
		require.Error(t, err)

		assert.NoFileExists(t, destination, "a streamed failure must not land a partial final file")
		assertNoStagedTransferFiles(t, dir)
	})
}

// TestTransferSecret_DispatchesBySource pins the single decision point:
// command sources stream, plain sources rsync.
func TestTransferSecret_DispatchesBySource(t *testing.T) {
	t.Parallel()

	t.Run("command source streams", func(t *testing.T) {
		t.Parallel()

		mach := newLocalTransferMachine(t, true)
		exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

		source := attributes.TransferSource{Command: "printf secret", RemotePath: "/var/secrets/key"}
		require.NoError(t, TransferSecret(exc, mach, source, "secrets", false))

		line := testutil.LastCommandLine(t, phaseLog)
		assert.Equal(t, "command source /var/secrets/key", line)
		assert.NotContains(t, line, "printf secret")
	})

	t.Run("plain source rsyncs", func(t *testing.T) {
		t.Parallel()

		mach := newLocalTransferMachine(t, true)
		exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

		source := attributes.TransferSource{LocalPath: "/tmp/src.key", RemotePath: "/var/secrets/key"}
		require.NoError(t, TransferSecret(exc, mach, source, "secrets", false))

		line := testutil.LastCommandLine(t, phaseLog)
		assert.Contains(t, line, "rsync")
		assert.NotContains(t, line, "sh -u")
	})
}
