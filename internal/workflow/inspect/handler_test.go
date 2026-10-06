package inspect

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runBootstrapModeInspect dispatch routing: the NixOS mode inspects the
// installer state (bootstrap detection over /etc/os-release), the nix-install
// mode and every other output type run the nix availability probe. Routing is
// asserted through the dry-run command record; the probe-outcome branches
// (missing nix tolerated, bootstrap.disable_nix_install hard error, missing
// nix fatal for other output types) hinge on command exit codes and are
// covered by the e2e suite. The dry-run probe callback also pins the
// MetaInspect.NixAvailable refresh.
func TestRunBootstrapModeInspect_Dispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		mode             installable.BootstrapMode
		wantCommands     []string
		wantNixAvailable bool
	}{
		{
			name:         "nixos: routes to the bootstrap installer inspect",
			mode:         installable.BootstrapNixOS,
			wantCommands: []string{"cat /etc/os-release"},
		},
		{
			name:             "nix-install: nix present proceeds",
			mode:             installable.BootstrapNixInstall,
			wantCommands:     []string{"nix --version"},
			wantNixAvailable: true,
		},
		{
			name:             "none: nix present proceeds",
			mode:             installable.BootstrapNone,
			wantCommands:     []string{"nix --version"},
			wantNixAvailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newSkipTestMachine(t, false, "")

			// Bootstrap SSH satisfies the unbootstrapped SSH-state check of
			// the NixOS path (irrelevant for the other modes).
			bootstrapClient := &ssh.SSHClient{Hostname: "10.0.0.9", Port: 22, Username: "bootstrap-admin"}
			require.NoError(t, bootstrapClient.Init("bootstrap-host", "", nixver.Info{}))
			mach.Bootstrap.SSH = *bootstrapClient

			leaf := &fleet.FleetLeaf{
				Installable: &installable.Installable{Preset: installable.Preset{Bootstrap: tt.mode}},
				Machine:     mach,
			}

			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)

			require.NoError(t, runBootstrapModeInspect(exc, leaf))

			var lines []string

			for _, line := range testutil.CommandLines(t, phaseLog) {
				if line != "" {
					lines = append(lines, line)
				}
			}

			assert.Equal(t, tt.wantCommands, lines)
			assert.Equal(t, tt.wantNixAvailable, mach.MetaInspect.Load().NixAvailable)
		})
	}
}
