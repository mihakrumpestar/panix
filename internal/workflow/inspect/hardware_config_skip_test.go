package inspect

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

// newSkipTestMachine builds a LOCAL machine with a controlled runtime
// IsRoot, RequiresKexec and hardware config path.
func newSkipTestMachine(t *testing.T, requiresKexec bool, hardwarePath string) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	mach.HardwareConfigPath = hardwarePath
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.SudoProgram = attributes.SudoProgram("sudo")
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: false, RequiresKexec: requiresKexec})

	return mach
}

// Pre-kexec the generation is skipped (no nixos-generate-config on the
// original OS); on NixOS it delegates to phaseops.GenerateHardwareConfig.
func TestHandleUnbootstrapped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		requiresKexec bool
		hardwarePath  string
		wantCommand   string
	}{
		{
			name:          "kexec required: skipped",
			requiresKexec: true,
			hardwarePath:  "/tmp/hardware-configuration.nix",
		},
		{
			name:          "kexec required without path: silent no-op",
			requiresKexec: true,
		},
		{
			name:         "no kexec: generation delegated",
			hardwarePath: "/tmp/hardware-configuration.nix",
			wantCommand:  "sudo nixos-generate-config --show-hardware-config --no-filesystems",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newSkipTestMachine(t, tt.requiresKexec, tt.hardwarePath)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

			require.NoError(t, handleUnbootstrapped(exc, mach))

			if tt.wantCommand == "" {
				assert.Empty(t, testutil.CommandLines(t, phaseLog))

				return
			}

			assert.Equal(t, tt.wantCommand, testutil.LastCommandLine(t, phaseLog))
		})
	}
}
