package phaseops

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newActivateCompositionMachine builds a LOCAL machine for composition tests;
// the local transport records bare command lines.
func newActivateCompositionMachine(t *testing.T, isRoot bool) *machine.Machine {
	t.Helper()

	return newLocalTransferMachine(t, isRoot)
}

// TestActivate_Composition pins that the deploy commands (profile set,
// activation script) ride inside WrapAsTargetUser, each argv shaped by preset
// level and target user: the unit-level net for what e2e covers under QEMU.
func TestActivate_Composition(t *testing.T) {
	t.Parallel()

	// Preset shapes per the built-ins: nixos is system-level profile-setting
	// switch-to-configuration; home-manager is user-level activation that
	// tracks the profile without setting it.

	for _, tt := range activateCompositionCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newActivateCompositionMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			require.NoError(t, Activate(exc, mach, tt.preset, "/nix/store/abc-closure", "switch", tt.targetUser, &nix.NixConfig{}, nixver.FlavorNix))

			lines := testutil.CommandLines(t, phaseLog)
			require.Len(t, lines, len(tt.wantLines))

			for i, want := range tt.wantLines {
				assert.Equal(t, want, lines[i])
			}
		})
	}
}

// TestActivate_ProfileSkipModes pins maybeSetProfile's skip branch: modes
// the preset declares profile-skipping (nixos: test, dry-activate) must
// not touch the profile.
func TestActivate_ProfileSkipModes(t *testing.T) {
	t.Parallel()

	mach := newActivateCompositionMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	preset := installable.Preset{
		ProfilePath:      "/nix/var/nix/profiles/system",
		SetProfile:       new(true),
		IsSystemLevel:    new(true),
		ActivationPath:   "bin/switch-to-configuration",
		ActivationModes:  []string{"switch", "test"},
		ProfileSkipModes: []string{"test", "dry-activate"},
		NonMutatingModes: []string{"dry-activate"},
	}

	require.NoError(t, Activate(exc, mach, preset, "/nix/store/abc-closure", "test", "", &nix.NixConfig{}, nixver.FlavorNix))

	lines := testutil.CommandLines(t, phaseLog)
	require.Len(t, lines, 1, "profile set must be skipped for test mode")
	assert.Equal(t, "/nix/store/abc-closure/bin/switch-to-configuration test", lines[0])
}

var activateCompositionCases = []struct {
	name       string
	isRoot     bool
	preset     installable.Preset
	targetUser string
	wantLines  []string
}{
	{
		name:   "nixos, root SSH, no target user: sudo-less profile set + activation",
		isRoot: true,
		preset: nixosCompositionPreset(),
		wantLines: []string{
			"nix-env --profile /nix/var/nix/profiles/system --set /nix/store/abc-closure",
			"/nix/store/abc-closure/bin/switch-to-configuration switch",
		},
	},
	{
		name:       "nixos, root SSH, target user bob: profile set and activation inside su -l with inner sudo",
		isRoot:     true,
		preset:     nixosCompositionPreset(),
		targetUser: "bob",
		wantLines: []string{
			`su -l bob -c XDG_RUNTIME_DIR=/run/user/$(id -u) 'sudo' 'nix-env' '--profile' '/nix/var/nix/profiles/system' '--set' '/nix/store/abc-closure'`,
			`su -l bob -c XDG_RUNTIME_DIR=/run/user/$(id -u) 'sudo' '/nix/store/abc-closure/bin/switch-to-configuration' 'switch'`,
		},
	},
	{
		name:       "home-manager shape, root SSH, target user alice: profile NOT set (SetProfile nil), activation inside su -l without sudo",
		isRoot:     true,
		preset:     homeCompositionPreset(),
		targetUser: "alice",
		wantLines: []string{
			`su -l alice -c XDG_RUNTIME_DIR=/run/user/$(id -u) '/nix/store/abc-closure/activate'`,
		},
	},
}

// nixosCompositionPreset is the system-level nixos shape: profile set plus a
// mode-bearing switch-to-configuration activation path.
func nixosCompositionPreset() installable.Preset {
	return installable.Preset{
		ProfilePath:           "/nix/var/nix/profiles/system",
		SetProfile:            new(true),
		IsSystemLevel:         new(true),
		ActivationPath:        "bin/switch-to-configuration",
		ActivationModes:       []string{"switch", "boot", "test", "dry-activate"},
		ActivationDefaultMode: "switch",
	}
}

// homeCompositionPreset is the user-level home-manager shape: a tilde profile
// without setting, activation via the package path.
func homeCompositionPreset() installable.Preset {
	return installable.Preset{
		ProfilePath:    "~/.local/state/nix/profiles/home-manager",
		IsSystemLevel:  new(false),
		ActivationPath: "activate",
	}
}
