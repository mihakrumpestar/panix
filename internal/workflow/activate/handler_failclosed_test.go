package activate

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBootstrapFleetLeaf(mach *machine.Machine) *fleet.FleetLeaf {
	return &fleet.FleetLeaf{
		Machine: mach,
		Installable: &installable.Installable{
			Preset:    installable.Preset{Bootstrap: installable.BootstrapNixOS},
			MetaBuild: &installable.MetaBuild{Closure: "/nix/store/closure"},
		},
	}
}

// Unknown inspect metadata must never select the destructive NixOS install
// path: no command may run (no copy to /mnt, no nixos-install, no reboot).
func TestRunPhase_UnknownInspectRefusesNixOSInstall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		metaInspect *machine.MetaInspect // nil stores an explicit nil
	}{
		{"nil inspect data", nil},
		{"atomicpointer.New zero value", &machine.MetaInspect{}},
		{"probed rootness but inspect never finished", &machine.MetaInspect{IsRoot: true, IsRootProbed: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newActivateMachine(t, true)
			mach.MetaInspect.Store(tt.metaInspect)

			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			err := Handler{}.RunPhase(exc, newBootstrapFleetLeaf(mach))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "inspect phase did not run")
			assert.Empty(t, testutil.CommandLines(t, phaseLog),
				"refusal must happen before any command, especially nixos-install and reboot")
		})
	}
}

// A live non-NixOS host (RequiresKexec) must never reach nixos-install when
// the bootstrap phase was skipped: kexec/disko must run first. No command may
// run on the refusal path.
func TestRunPhase_RequiresKexecRefusesNixOSInstall(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	mach.MetaInspect.Store(&machine.MetaInspect{
		Inspected:     true,
		Bootstrapped:  false,
		RequiresKexec: true,
		IsRoot:        true,
		IsRootProbed:  true,
	})

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	err := Handler{}.RunPhase(exc, newBootstrapFleetLeaf(mach))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires kexec")
	assert.Empty(t, testutil.CommandLines(t, phaseLog),
		"refusal must happen before any command, especially nixos-install and reboot")
}

// A successfully inspected unbootstrapped machine must still bootstrap.
func TestRunPhase_InspectedUnbootstrappedBootstraps(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	mach.MetaInspect.Store(&machine.MetaInspect{
		Inspected:    true,
		Bootstrapped: false,
		IsRoot:       true,
		IsRootProbed: true,
	})

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	require.NoError(t, Handler{}.RunPhase(exc, newBootstrapFleetLeaf(mach)))

	lines := testutil.CommandLines(t, phaseLog)
	require.NotEmpty(t, lines)

	last := lines[len(lines)-1]
	assert.Contains(t, last, "nixos-install")
	assert.Contains(t, last, "--root /mnt")
	assert.NotContains(t, strings.Join(lines, "\n"), "reboot",
		"automatic reboot is disabled in the test machine")
}
