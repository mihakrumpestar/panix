package phaseops

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/structdefault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLocalTransferMachine builds a LOCAL machine with a controlled runtime
// IsRoot: the local transport records the bare command line, so the
// elevation form (plain prefix vs --rsync-path) is directly visible.
func newLocalTransferMachine(t *testing.T, isRoot bool) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.SudoProgram = attributes.SudoProgram("sudo")
	structdefault.Apply(&mach.Attributes)
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	return mach
}

// newRemoteTransferMachine builds a remote (SSH) machine: the recorded
// command line shows the remote form, including --rsync-path.
func newRemoteTransferMachine(t *testing.T, isRoot bool) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	mach.SSH = ssh.SSHClient{Hostname: "10.0.0.1", Port: 22, Username: "deploy"}
	mach.SudoProgram = attributes.SudoProgram("sudo")
	structdefault.Apply(&mach.Attributes)
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	return mach
}

// TestTransferFile_Elevation pins the transport-aware elevation cases: sudo via
// --rsync-path remotely, whole-command prefix locally, absent when root.
func TestTransferFile_Elevation(t *testing.T) {
	t.Parallel()

	file := attributes.TransferSource{
		LocalPath:  "/tmp/src.key",
		RemotePath: "/var/secrets/key",
	}

	for _, tt := range transferElevationCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newRemoteTransferMachine(t, tt.isRoot)
			if tt.local {
				mach = newLocalTransferMachine(t, tt.isRoot)
			}

			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

			require.NoError(t, TransferFile(exc, mach, file, "secret", false))

			line := testutil.LastCommandLine(t, phaseLog)
			assert.Contains(t, line, tt.want)

			if tt.noRsyncPath {
				assert.NotContains(t, line, "--rsync-path")
			}

			if tt.noSudo {
				// Root elevation must be absent in both forms; a Contains-only
				// assertion would pass with spurious sudo.
				assert.NotContains(t, line, "sudo")
			}
		})
	}
}

var transferElevationCases = []struct {
	name        string
	isRoot      bool
	local       bool
	want        string
	noRsyncPath bool
	noSudo      bool
}{
	{
		name:   "remote, non-root: --rsync-path carries sudo",
		isRoot: false,
		want:   "--rsync-path=sudo rsync",
	},
	{
		name:        "remote, root: no elevation flags",
		isRoot:      true,
		want:        "10.0.0.1:/var/secrets/key",
		noRsyncPath: true,
		noSudo:      true,
	},
	{
		name:        "local, non-root: whole-command sudo prefix",
		isRoot:      false,
		local:       true,
		want:        "sudo rsync",
		noRsyncPath: true,
	},
	{
		name:        "local, root: bare rsync",
		isRoot:      true,
		local:       true,
		want:        "/var/secrets/key",
		noRsyncPath: true,
		noSudo:      true,
	},
}

// TestRsyncChmodFlag pins the file/directory split: files keep the configured
// mode while directories stay traversable, including for a 0600 secret.
func TestRsyncChmodFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		perms attributes.FileMode
		want  string
	}{
		{name: "0600 files keep private files and traversable directories", perms: 0o600, want: "D700,F600"},
		{name: "0640 adds group traverse when group can read", perms: 0o640, want: "D750,F640"},
		{name: "0644 adds group and other traverse", perms: 0o644, want: "D755,F644"},
		{name: "zero uses the default 0700 file mode", perms: 0, want: "D700,F700"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, rsyncChmodFlag(tt.perms))
		})
	}
}

// TestTransferFile_ChmodKeepsDirectoriesTraversable pins the rendered rsync
// argv: 0600 applies to files only, directories get an owner-traversable mode,
// and --perms stays enabled.
func TestTransferFile_ChmodKeepsDirectoriesTraversable(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

	source := attributes.TransferSource{
		LocalPath:   "/tmp/src.key",
		RemotePath:  "/var/secrets/key",
		Permissions: attributes.FileMode(0o600),
	}

	require.NoError(t, TransferFile(exc, mach, source, "secret", false))

	line := testutil.LastCommandLine(t, phaseLog)
	assert.Contains(t, line, "--chmod=D700,F600")
	assert.Contains(t, line, "--perms")
}

// TestTransferFile_Chown pins the uid/gid rendering: --chown is
// receiver-side, so elevation is what makes it actually apply.
func TestTransferFile_Chown(t *testing.T) {
	t.Parallel()

	uid := uint(1000)
	gid := uint(100)
	fileWithOwner := attributes.TransferSource{
		LocalPath:  "/tmp/src.key",
		RemotePath: "/var/secrets/key",
		UID:        &uid,
		GID:        &gid,
	}

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

	require.NoError(t, TransferFile(exc, mach, fileWithOwner, "secret", false))

	assert.Contains(t, testutil.LastCommandLine(t, phaseLog), "--chown=1000:100")
}

// TestTransferFile_RsyncCommandOverride pins that a configured rsync prefix
// replaces the binary everywhere it appears, including --rsync-path on the
// remote side: the receiving command must stay the same executable.
func TestTransferFile_RsyncCommandOverride(t *testing.T) {
	t.Parallel()

	file := attributes.TransferSource{LocalPath: "/tmp/src.key", RemotePath: "/var/secrets/key"}

	tests := []struct {
		name string
		mach func(t *testing.T) *machine.Machine
		want string
	}{
		{
			name: "local",
			mach: func(t *testing.T) *machine.Machine {
				t.Helper()

				return newLocalTransferMachine(t, false)
			},
			want: "sudo rsync-wrapper --quiet -rcEx",
		},
		{
			name: "remote",
			mach: func(t *testing.T) *machine.Machine {
				t.Helper()

				return newRemoteTransferMachine(t, false)
			},
			want: "rsync-wrapper --quiet -rcEx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := tt.mach(t)
			mach.RsyncCommand = []string{"rsync-wrapper", "--quiet"}

			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

			require.NoError(t, TransferFile(exc, mach, file, "secret", false))

			assert.Contains(t, testutil.LastCommandLine(t, phaseLog), tt.want)
		})
	}

	// The remote receiving side must use the same wrapper.
	mach := newRemoteTransferMachine(t, false)
	mach.RsyncCommand = []string{"rsync-wrapper", "--quiet"}

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)
	require.NoError(t, TransferFile(exc, mach, file, "secret", false))

	assert.Contains(t, testutil.LastCommandLine(t, phaseLog), "--rsync-path=sudo rsync-wrapper --quiet")
}

// TestRemoteRsyncPath pins the --rsync-path rendering: shell-inert words pass
// through untouched so the common form is unchanged, while a configured prefix
// containing whitespace is quoted so the remote rsync's own tokenizer cannot
// split it into extra arguments.
func TestRemoteRsyncPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		maybeSudo []string
		rsyncCmd  []string
		want      string
	}{
		{
			name:      "default form is unchanged",
			maybeSudo: []string{"sudo"},
			rsyncCmd:  []string{"rsync"},
			want:      "sudo rsync",
		},
		{
			name:      "prefix with whitespace stays one word",
			maybeSudo: []string{"sudo"},
			rsyncCmd:  []string{"rsync --fake-super", "rsync"},
			want:      "sudo 'rsync --fake-super' rsync",
		},
		{
			name:      "quote in a word cannot break out",
			maybeSudo: []string{"sudo"},
			rsyncCmd:  []string{"rsync'; rm -rf /; echo '"},
			want:      `sudo 'rsync'\''; rm -rf /; echo '\'''`,
		},
		{
			name:      "no elevation still renders",
			maybeSudo: nil,
			rsyncCmd:  []string{"/run/current-system/sw/bin/rsync"},
			want:      "/run/current-system/sw/bin/rsync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, remoteRsyncPath(tt.maybeSudo, tt.rsyncCmd))
		})
	}
}

// A configured prefix must reach the remote side even when the SSH user is root
// and no elevation is needed: --rsync-path is the only channel it travels on, so
// it cannot be emitted only alongside sudo. Asserting on --rsync-path rather
// than on the prefix name keeps the check honest, because the local argv already
// begins with the prefix either way.
func TestTransferFile_RsyncCommandOverrideWithoutElevation(t *testing.T) {
	t.Parallel()

	mach := newRemoteTransferMachine(t, true)
	mach.RsyncCommand = []string{"rsync-wrapper"}

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)
	file := attributes.TransferSource{LocalPath: "/tmp/src.key", RemotePath: "/var/secrets/key"}

	require.NoError(t, TransferFile(exc, mach, file, "secret", false))

	assert.Contains(t, testutil.LastCommandLine(t, phaseLog), "--rsync-path=rsync-wrapper",
		"root SSH user must still pass the configured prefix to the remote rsync")
}

// Locally there is no remote side, so the prefix is the command itself and no
// --rsync-path may be emitted (rsync ignores it locally).
func TestTransferFile_RsyncCommandOverrideLocal(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, true)
	mach.RsyncCommand = []string{"rsync-wrapper"}

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)
	file := attributes.TransferSource{LocalPath: "/tmp/src.key", RemotePath: "/var/secrets/key"}

	require.NoError(t, TransferFile(exc, mach, file, "secret", false))

	line := testutil.LastCommandLine(t, phaseLog)
	assert.True(t, strings.HasPrefix(line, "rsync-wrapper "), "local command must start with the prefix: %s", line)
	assert.NotContains(t, line, "--rsync-path")
}

// An unset prefix with no elevation stays a bare rsync: no stray --rsync-path.
func TestTransferFile_NoPrefixNoElevation(t *testing.T) {
	t.Parallel()

	mach := newRemoteTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Secrets)

	file := attributes.TransferSource{LocalPath: "/tmp/src.key", RemotePath: "/var/secrets/key"}
	require.NoError(t, TransferFile(exc, mach, file, "secret", false))

	assert.NotContains(t, testutil.LastCommandLine(t, phaseLog), "--rsync-path")
}
