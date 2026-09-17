package phaseops

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
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
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	return mach
}

// TestTransferFile_Elevation pins the transport-aware elevation cases: sudo via
// --rsync-path remotely, whole-command prefix locally, absent when root.
func TestTransferFile_Elevation(t *testing.T) {
	t.Parallel()

	file := attributes.PlainFileOrDirToTransfer{
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

// TestTransferFile_Chown pins the uid/gid rendering: --chown is
// receiver-side, so elevation is what makes it actually apply.
func TestTransferFile_Chown(t *testing.T) {
	t.Parallel()

	uid := uint(1000)
	gid := uint(100)
	fileWithOwner := attributes.PlainFileOrDirToTransfer{
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
