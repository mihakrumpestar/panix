package hardwareconfig

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPhaseLeaf builds a leaf with a controlled preset and machine state, the
// inputs ShouldSkip and RunPhase decide on. The machine is local and root, so a
// recorded argv is the command itself: no ssh wrapper and no sudo.
func newPhaseLeaf(t *testing.T, hardwarePath string, bootstrapsNixOS bool, meta *machine.MetaInspect) *fleet.FleetLeaf {
	t.Helper()

	preset := installable.Preset{}
	if bootstrapsNixOS {
		preset.Bootstrap = installable.BootstrapNixOS
	}

	mach := testutil.NewFaker().Machine()
	if meta != nil {
		mach.MetaInspect.Store(meta)
	}

	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.MetaInspect.Update(func(mi *machine.MetaInspect) { mi.IsRoot = true })
	mach.HardwareConfig.Path = hardwarePath

	return &fleet.FleetLeaf{Installable: &installable.Installable{Preset: preset}, Machine: mach}
}

// TestDecide pins the phase ownership matrix: a real run takes only installed
// machines, because Inspect already generated the file when it found the machine
// in the NixOS installer, and a pre-kexec machine is reported as a deploy's job.
// A dry run executes nothing, so it previews the generation for every machine.
func TestDecide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		meta   *machine.MetaInspect
		dryRun bool
		want   phaseAction
	}{
		{
			name: "installed machine",
			meta: &machine.MetaInspect{Bootstrapped: true},
			want: actionGenerate,
		},
		{
			name: "installer machine, Inspect generated it",
			meta: &machine.MetaInspect{Bootstrapped: false},
			want: actionSkipInstallerMachine,
		},
		{
			name: "pre-kexec machine is reported, not skipped",
			meta: &machine.MetaInspect{RequiresKexec: true},
			want: actionKexecRequired,
		},
		{
			name: "no inspect state at all",
			want: actionSkipInstallerMachine,
		},
		{
			name:   "dry run previews the generation regardless of placeholder state",
			meta:   &machine.MetaInspect{Bootstrapped: false, RequiresKexec: true},
			dryRun: true,
			want:   actionGenerate,
		},
		{
			name:   "dry run of an installed machine",
			meta:   &machine.MetaInspect{Bootstrapped: true},
			dryRun: true,
			want:   actionGenerate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := testutil.NewFaker().Machine()
			if tt.meta != nil {
				mach.MetaInspect.Store(tt.meta)
			}

			assert.Equal(t, tt.want, decide(mach, tt.dryRun))
		})
	}
}

// TestShouldSkip pins the machine-independent skips: a machine with no path, or
// one whose output type does not bootstrap NixOS, has nothing to generate for in
// any run mode.
func TestShouldSkip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		hardwarePath string
		bootstraps   bool
		want         bool
	}{
		{
			name:       "no hardware config path",
			bootstraps: true,
			want:       true,
		},
		{
			name:         "non-NixOS output type",
			hardwarePath: "/repo/hardware-configuration.nix",
			want:         true,
		},
		{
			name:         "configured machine",
			hardwarePath: "/repo/hardware-configuration.nix",
			bootstraps:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			leaf := newPhaseLeaf(t, tt.hardwarePath, tt.bootstraps, &machine.MetaInspect{Bootstrapped: true})

			assert.Equal(t, tt.want, Handler{}.ShouldSkip(leaf))
		})
	}
}

// TestRunPhase pins the phase's own guards on a dry-run executioner: the
// write-mode generator is refused before it can rewrite the target's /etc/nixos,
// and a printing generator is previewed instead.
func TestRunPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		generator attributes.Generator
		wantRun   string
		wantErr   string
	}{
		{
			name:      "write-mode nixos-generate-config is refused",
			generator: attributes.Generator{Command: []string{"nixos-generate-config"}, Args: []string{"--no-filesystems"}},
			wantErr:   "runs without --show-hardware-config",
		},
		{
			name:      "printing nixos-generate-config is previewed",
			generator: attributes.Generator{Command: []string{"nixos-generate-config"}, Args: []string{"--show-hardware-config"}},
			wantRun:   "nixos-generate-config --show-hardware-config",
		},
		{
			name:      "another generator is previewed",
			generator: attributes.Generator{Command: []string{"nixos-facter"}, Args: []string{}},
			wantRun:   "nixos-facter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			leaf := newPhaseLeaf(t, "/repo/hardware-configuration.nix", true, &machine.MetaInspect{Bootstrapped: true})
			leaf.Machine.HardwareConfig.Generate = tt.generator

			exc, phaseLog := testutil.NewDryRunExecutioner(t, leaf.Machine, phase.HardwareConfig)

			err := Handler{}.RunPhase(exc, leaf)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Empty(t, testutil.CommandLines(t, phaseLog), "the refused generator must not run")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, []string{tt.wantRun}, testutil.CommandLines(t, phaseLog))
		})
	}
}

type prepareCase struct {
	name         string
	skipPhases   []phase.Phase
	hardwarePath string
	bootstraps   bool
	generator    attributes.Generator
	wantErr      error
}

var prepareCases = []prepareCase{
	{
		name:         "configured machine",
		hardwarePath: "/repo/hardware-configuration.nix",
		bootstraps:   true,
	},
	{
		name:       "own phase skipped",
		skipPhases: []phase.Phase{phase.HardwareConfig},
		wantErr:    ErrPhaseSkipped,
	},
	{
		name:    "nothing to generate for",
		wantErr: ErrNoHardwareConfigTargets,
	},
	{
		name:         "path on a non-NixOS output type",
		hardwarePath: "/repo/hardware-configuration.nix",
		wantErr:      ErrNoHardwareConfigTargets,
	},
	{
		name:         "write-mode generator",
		hardwarePath: "/repo/hardware-configuration.nix",
		bootstraps:   true,
		generator:    attributes.Generator{Command: []string{"nixos-generate-config"}, Args: []string{}},
		wantErr:      phaseops.ErrWriteModeGenerator,
	},
}

// TestPrepare pins the command's pre-flight rejections, so a run that cannot
// generate anything stops before a machine is contacted.
func TestPrepare(t *testing.T) {
	t.Parallel()

	for _, tt := range prepareCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := testutil.NewFaker().Machine()
			mach.HardwareConfig.Path = tt.hardwarePath
			mach.HardwareConfig.Generate = tt.generator

			inst := testutil.NewFaker().Installable(mach)
			if !tt.bootstraps {
				inst.Preset.Bootstrap = installable.BootstrapNone
			}

			fake := testutil.NewFaker()
			conf := &config.Config{
				Flags: flags.Flags{WorkflowFlags: flags.WorkflowFlags{
					EvalFlags: flags.EvalFlags{SkipPhases: tt.skipPhases},
				}},
				Fleet: fake.Fleet(fake.Flake(inst)),
			}

			err := Prepare(conf)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
