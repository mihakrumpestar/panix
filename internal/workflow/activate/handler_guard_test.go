package activate

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/stretchr/testify/assert"
)

// newRoutingMachine builds a machine with a configured rollback tier and a
// probed generation list (the rollback target the guardian protects).
func newRoutingMachine(rollback string, withGenerations bool) *machine.Machine {
	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}

	mach.Rollback = attributes.Rollback(rollback)
	if withGenerations {
		mach.MetaInspect.Store(&machine.MetaInspect{
			Generations: &machine.Generations{Current: 2, Available: []uint{1, 2}},
		})
	}

	return mach
}

// TestShouldRouteToGuard pins the effective routing predicate (spec 2, T3c):
// tier x mode x profile x previous-generation matrix.
func TestShouldRouteToGuard(t *testing.T) {
	t.Parallel()

	nixosInstallable := &installable.Installable{
		Type: "nixosConfigurations",
		Preset: installable.Preset{
			ProfilePath:      "/nix/var/nix/profiles/system",
			NonMutatingModes: []string{"dry-activate"},
			GuardTier:        installable.GuardTierFull,
		},
	}
	packagesInstallable := &installable.Installable{
		Type:   "packages",
		Preset: installable.Preset{},
	}
	darwinInstallable := &installable.Installable{
		Type:   "darwinConfigurations",
		Preset: installable.Preset{ProfilePath: "/nix/var/nix/profiles/system", GuardTier: installable.GuardTierStandard},
	}

	tests := []struct {
		name        string
		machine     *machine.Machine
		installable *installable.Installable
		mode        string
		want        bool
	}{
		{
			name:        "auto tier with a previous generation guards",
			machine:     newRoutingMachine("auto", true),
			installable: nixosInstallable, mode: "switch", want: true,
		},
		{
			name:        "magic tier guards",
			machine:     newRoutingMachine("magic", true),
			installable: nixosInstallable, mode: "switch", want: true,
		},
		{
			name:        "off tier stays legacy",
			machine:     newRoutingMachine("off", true),
			installable: nixosInstallable, mode: "switch", want: false,
		},
		{
			name:        "unset tier stays legacy",
			machine:     newRoutingMachine("", true),
			installable: nixosInstallable, mode: "switch", want: false,
		},
		{
			name:        "non-mutating mode stays legacy",
			machine:     newRoutingMachine("auto", true),
			installable: nixosInstallable, mode: "dry-activate", want: false,
		},
		{
			name:        "first deploy has no rollback target and stays legacy",
			machine:     newRoutingMachine("auto", false),
			installable: nixosInstallable, mode: "switch", want: false,
		},
		{
			name:        "unsupported tier class stays legacy",
			machine:     newRoutingMachine("auto", true),
			installable: packagesInstallable, mode: "switch", want: false,
		},
		{
			name:        "darwin standard tier guards",
			machine:     newRoutingMachine("auto", true),
			installable: darwinInstallable, mode: "switch", want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, shouldRouteToGuard(tt.machine, tt.installable, tt.mode))
		})
	}
}

// TestResolveActivationMode pins the mode precedence (spec 16): installable
// override, preset default, CLI override.
func TestResolveActivationMode(t *testing.T) {
	t.Parallel()

	inst := &installable.Installable{
		Type:           "nixosConfigurations",
		ActivationMode: "test",
		Preset:         installable.Preset{ActivationDefaultMode: "switch"},
	}

	assert.Equal(t, "test", resolveActivationMode(flags.ActivationMode{}, inst), "installable override wins over the preset default")

	cli := flags.ActivationMode{AllTypes: "boot"}
	assert.Equal(t, "boot", resolveActivationMode(cli, inst), "CLI override wins over everything")

	instDefault := &installable.Installable{
		Type:   "nixosConfigurations",
		Preset: installable.Preset{ActivationDefaultMode: "switch"},
	}
	assert.Equal(t, "switch", resolveActivationMode(flags.ActivationMode{}, instDefault), "the preset default applies without overrides")
}
