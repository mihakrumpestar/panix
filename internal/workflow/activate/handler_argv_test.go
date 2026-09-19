package activate

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newActivateMachine builds a LOCAL machine with a controlled runtime
// IsRoot, so recorded argv shows privilege prefixes directly.
func newActivateMachine(t *testing.T, isRoot bool) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.Bootstrap.DisableAutomaticReboot = true
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	return mach
}

// MaybeSudo prefix for a non-root SSH user, none for root; the env(1)
// argv rides inside the prefix so variables survive sudo's env_reset.
func TestExecuteBootstrap_NixosInstallElevation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{
			name:   "non-root SSH user: sudo prefix, env argv inside",
			isRoot: false,
			want:   "sudo env NIXOS_INSTALL_ENV=test nixos-install --no-root-passwd --no-channel-copy --system /nix/store/closure --root /mnt",
		},
		{
			name:   "root SSH user: bare command",
			isRoot: true,
			want:   "env NIXOS_INSTALL_ENV=test nixos-install --no-root-passwd --no-channel-copy --system /nix/store/closure --root /mnt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newActivateMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			nixCfg := &nix.NixConfig{Env: map[string]string{"NIXOS_INSTALL_ENV": "test"}}
			require.NoError(t, executeBootstrap(exc, mach, nixCfg, "/nix/store/closure"))

			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}

// The argv is exactly [MaybeSudo...] + ["reboot"]: dropping the command
// leaves ssh with no remote command, which hangs until the timeout.
func TestPerformReboot_Elevation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{"non-root SSH user: sudo prefix", false, "sudo reboot"},
		{"root SSH user: bare reboot", true, "reboot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newActivateMachine(t, tt.isRoot)
			mach.Bootstrap.DisableAutomaticReboot = false
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			require.NoError(t, performReboot(exc, mach))

			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}
