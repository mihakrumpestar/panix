package phaseops

import (
	"fmt"
	"os"
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

// TestTransferCommandScript pins the exact destination script: step order, the
// configured ownership and mode, and shellquoting of space and quote bearing
// paths.
func TestTransferCommandScript(t *testing.T) {
	t.Parallel()

	uid := uint(1000)
	gid := uint(100)

	tests := []struct {
		name   string
		source attributes.TransferSource
		want   string
	}{
		{
			name:   "no ownership defaults to 0700",
			source: attributes.TransferSource{RemotePath: "/var/secrets/key"},
			want: "set -e\numask 077\nmkdir -p -- '/var/secrets'\n" +
				"cat > '/var/secrets/key'\nchmod 700 -- '/var/secrets/key'\n",
		},
		{
			name: "uid and gid chown between cat and chmod",
			source: attributes.TransferSource{
				RemotePath:  "/var/secrets/key",
				UID:         &uid,
				GID:         &gid,
				Permissions: attributes.FileMode(0o640),
			},
			want: "set -e\numask 077\nmkdir -p -- '/var/secrets'\n" +
				"cat > '/var/secrets/key'\nchown 1000:100 -- '/var/secrets/key'\nchmod 640 -- '/var/secrets/key'\n",
		},
		{
			name:   "spaces and single quotes stay one shell word",
			source: attributes.TransferSource{RemotePath: "/var/my secrets/it's a key"},
			want: "set -e\numask 077\nmkdir -p -- '/var/my secrets'\n" +
				"cat > '/var/my secrets/it'\\''s a key'\nchmod 700 -- '/var/my secrets/it'\\''s a key'\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, transferCommandScript(tt.source.RemotePath, tt.source))
		})
	}
}

// quotedTrickyPath is shellquote.Quote("/var/my secrets/it's a key"): spaces
// and a single quote stay one literal shell word.
const quotedTrickyPath = `'/var/my secrets/it'\''s a key'`

// TestTransferCommandProbeScript pins the exact probe script: enforcement of
// mode (and owner when set) on the existing target, the sha256sum output, the
// optional owner line and shellquoting.
func TestTransferCommandProbeScript(t *testing.T) {
	t.Parallel()

	uid := uint(1000)
	gid := uint(100)

	tests := []struct {
		name   string
		source attributes.TransferSource
		want   string
	}{
		{
			name:   "no ownership enforces the default mode only",
			source: attributes.TransferSource{RemotePath: "/var/secrets/key"},
			want: "set -e\n" +
				"if [ -e '/var/secrets/key' ]; then\n" +
				"  [ \"$(stat -c '%a' -- '/var/secrets/key')\" = '700' ] || chmod '700' -- '/var/secrets/key'\n" +
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
				"if [ -e '/var/secrets/key' ]; then\n" +
				"  [ \"$(stat -c '%u:%g' -- '/var/secrets/key')\" = '1000:100' ] || chown '1000:100' -- '/var/secrets/key'\n" +
				"  [ \"$(stat -c '%a' -- '/var/secrets/key')\" = '640' ] || chmod '640' -- '/var/secrets/key'\n" +
				"  sha256sum -- '/var/secrets/key'\n" +
				"fi\n",
		},
		{
			name:   "spaces and single quotes stay one shell word",
			source: attributes.TransferSource{RemotePath: "/var/my secrets/it's a key"},
			want: "set -e\n" +
				"if [ -e " + quotedTrickyPath + " ]; then\n" +
				"  [ \"$(stat -c '%a' -- " + quotedTrickyPath + ")\" = '700' ] || chmod '700' -- " + quotedTrickyPath + "\n" +
				"  sha256sum -- " + quotedTrickyPath + "\n" +
				"fi\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, transferCommandProbeScript(tt.source.RemotePath, tt.source))
		})
	}
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
			assert.Contains(t, script, "cat > '/var/secrets/key'")
		})
	}
}

// TestTransferCommandPipeSpecBootstrapping pins the /mnt prefix on the
// destination script: os secrets are prefixed until the machine is
// bootstrapped, plain secrets never are.
func TestTransferCommandPipeSpecBootstrapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		bootstrapped      bool
		transferOSSecrets bool
		wantScript        string
		wantNoScript      string
	}{
		{
			name:              "os secrets get the bootstrapping prefix",
			transferOSSecrets: true,
			wantScript:        "cat > '/mnt/var/secrets/key'",
		},
		{
			name:              "os secrets drop the prefix once bootstrapped",
			bootstrapped:      true,
			transferOSSecrets: true,
			wantScript:        "cat > '/var/secrets/key'",
			wantNoScript:      "/mnt",
		},
		{
			name:         "plain secrets never get the prefix",
			wantScript:   "cat > '/var/secrets/key'",
			wantNoScript: "/mnt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newLocalTransferMachine(t, true)
			mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: true, Bootstrapped: tt.bootstrapped})

			source := attributes.TransferSource{Command: "printf secret", RemotePath: "/var/secrets/key"}
			plan := transferCommandPipeSpec(mach, source, tt.transferOSSecrets)

			script := assertShScriptArgv(t, plan.Write, false)
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
		name              string
		isRoot            bool
		bootstrapped      bool
		transferOSSecrets bool
		wantSudo          bool
		wantScript        string
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
			name:              "os secrets probe the bootstrapping root",
			isRoot:            true,
			transferOSSecrets: true,
			wantScript:        "sha256sum -- '/mnt/var/secrets/key'",
		},
		{
			name:              "bootstrapped os secrets probe the final root",
			isRoot:            true,
			bootstrapped:      true,
			transferOSSecrets: true,
			wantScript:        "sha256sum -- '/var/secrets/key'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newLocalTransferMachine(t, tt.isRoot)
			mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: tt.isRoot, Bootstrapped: tt.bootstrapped})

			source := attributes.TransferSource{Command: "printf secret", RemotePath: "/var/secrets/key"}
			plan := transferCommandPipeSpec(mach, source, tt.transferOSSecrets)

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

	// Metadata-only change: no content rewrite, but the mode is enforced.
	source.Permissions = attributes.FileMode(0o640)
	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))
	assert.Equal(t, payloadA, readCommandTestFile(t, destination))
	assert.Equal(t, backdated, commandTestModTime(t, destination), "metadata enforcement must not rewrite the file")
	assertCommandTestMode(t, destination, 0o640)

	// Changed content: the file is rewritten and keeps the enforced mode.
	source.Command = printfPayloadCommand(payloadB)
	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))
	assert.Equal(t, payloadB, readCommandTestFile(t, destination))
	assert.True(t, commandTestModTime(t, destination).After(backdated), "changed content must rewrite the file")
	assertCommandTestMode(t, destination, 0o640)
}

// TestTransferCommand_DryRunRecordsSourceAndSkipsExecution pins that dry-run
// records the source command and starts neither process.
func TestTransferCommand_DryRunRecordsSourceAndSkipsExecution(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

	destination := filepath.Join(t.TempDir(), "must-not-exist.bin")
	source := attributes.TransferSource{Command: "printf secret", RemotePath: destination}

	require.NoError(t, TransferCommand(exc, mach, source, "secrets", false))

	assert.Equal(t, "sh -u -c printf secret", testutil.LastCommandLine(t, phaseLog))

	_, err := os.Stat(destination)
	require.ErrorIs(t, err, os.ErrNotExist, "dry run must not start the destination")
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

		assert.Equal(t, "sh -u -c printf secret", testutil.LastCommandLine(t, phaseLog))
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
