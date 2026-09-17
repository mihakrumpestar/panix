package bootstrap

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDiskoLeaf builds a fleet leaf with a LOCAL machine and a controlled
// runtime IsRoot for disko argv assertions.
func newDiskoLeaf(t *testing.T, isRoot bool) *fleet.FleetLeaf {
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

	inst := &installable.Installable{}
	inst.Preset = installable.Preset{}
	inst.Xpath = xpath.New("fleet", "flakes", "inst")
	inst.Type = installable.FlakeOutputType("nixosConfigurations")
	inst.Name = "test-vm"

	fl := &flake.Flake{}
	fl.Name = "test"
	fl.URL = "path:/test"

	return &fleet.FleetLeaf{Flake: fl, Installable: inst, Machine: mach}
}

// Runs the full disko() flow in DryRun: the build placeholder flows
// through transfer into the final exec, elevated via MaybeSudo unless root.
func TestDisko_Elevation(t *testing.T) {
	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{
			name:   "non-root SSH user: sudo prefix",
			isRoot: false,
			want:   "sudo disko_BUILD_OUTPUT_PATH_PLACEHOLDER",
		},
		{
			name:   "root SSH user: bare script",
			isRoot: true,
			want:   "disko_BUILD_OUTPUT_PATH_PLACEHOLDER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: t.Setenv forbids it. Sandbox XDG_CACHE_HOME
			// because the dry-run build mkdirs <cache>/panix/...
			t.Setenv("XDG_CACHE_HOME", t.TempDir())

			leaf := newDiskoLeaf(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, leaf.Machine, phase.Bootstrap)

			require.NoError(t, disko(exc, leaf, ""))

			// Transfer is SkipIfLocal, so disko is the last recorded line.
			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}
