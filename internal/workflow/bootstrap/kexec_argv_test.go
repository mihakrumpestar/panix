package bootstrap

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newKexecMachine builds a LOCAL machine with a controlled runtime IsRoot,
// so recorded argv shows privilege prefixes directly.
func newKexecMachine(t *testing.T, isRoot bool) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test", Username: "kexec-user"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	return mach
}

// The staging dir must be owned by the ACTIVE connection's user (bootstrap
// during kexec staging), not the regular SSH user, which may differ.
func TestCreateKexecDirectory_BootstrapSSHUser(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, false)

	bootstrapClient := &ssh.SSHClient{Hostname: "10.0.0.9", Port: 22, Username: "bootstrap-admin"}
	require.NoError(t, bootstrapClient.Init("bootstrap-host", "", nixver.Info{}))
	mach.Bootstrap.SSH = *bootstrapClient
	mach.State.Store(&machine.State{ActiveSSH: machine.SSHTypeBootstrap})

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	require.NoError(t, createKexecDirectory(exc, mach))

	// Assert usernames: the remote transport quotes the line.
	line := testutil.LastCommandLine(t, phaseLog)
	assert.Contains(t, line, "bootstrap-admin")
	assert.NotContains(t, line, "kexec-user")
}

// ONE elevated command resets and creates the dir owned by the SSH user
// (mode 700), closing the rm→mkdir TOCTOU window; only the kexec run
// itself elevates.
func TestCreateKexecDirectory_Elevation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{
			name:   "non-root SSH user: elevated reset+install, SSH-user-owned, mode 700",
			isRoot: false,
			want:   "sudo sh -c rm -rf '/tmp/kexec' && install -d -m 700 -o 'kexec-user' '/tmp/kexec'",
		},
		{
			name:   "root SSH user: bare reset+install still owned by the SSH user",
			isRoot: true,
			want:   "sh -c rm -rf '/tmp/kexec' && install -d -m 700 -o 'kexec-user' '/tmp/kexec'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			require.NoError(t, createKexecDirectory(exc, mach))

			lines := testutil.CommandLines(t, phaseLog)
			require.Len(t, lines, 1)
			assert.Equal(t, tt.want, lines[0])
		})
	}
}

// No elevation: the staging dir is SSH-user-owned, so tar needs no prefix.
func TestExtractKexecTarball_NoElevation(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, false)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	require.NoError(t, extractKexecTarball(exc, "https://example.com/file.tar.gz"))

	assert.Equal(t, "tar -xzf /tmp/kexec/kexec.tar -C /tmp/kexec", testutil.LastCommandLine(t, phaseLog))
}

// The staged run script is the one elevated step: kexec needs root.
func TestRunKexecCommand_Elevation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{"non-root SSH user: sudo prefix", false, "sudo /tmp/kexec/kexec/run"},
		{"root SSH user: bare", true, "/tmp/kexec/kexec/run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			require.NoError(t, runKexecCommand(exc, mach))

			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}
