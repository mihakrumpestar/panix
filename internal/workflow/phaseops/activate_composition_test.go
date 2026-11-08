package phaseops

import (
	"strings"
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

			nixCfg := &nix.NixConfig{}
			require.NoError(t, nixCfg.Init(nil))

			require.NoError(t, Activate(exc, mach, tt.preset, "/nix/store/abc-closure", "switch", tt.targetUser, nixCfg, nixver.FlavorNix))

			lines := testutil.CommandLines(t, phaseLog)
			require.Len(t, lines, len(tt.wantLines))

			for i, want := range tt.wantLines {
				assert.Equal(t, want, lines[i])
			}
		})
	}
}

// TestActivate_SuEnvRedactedInCommandLog pins the display redaction for the
// real activatePackage shape: secret token values inside the joined `su -c`
// string must never reach the command log.
func TestActivate_SuEnvRedactedInCommandLog(t *testing.T) {
	t.Parallel()

	mach := newActivateCompositionMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{Env: map[string]string{"NIX_CONFIG": "access-tokens=SUPERSECRET"}}
	require.NoError(t, nixCfg.Init(nil))

	// A package activation (no ActivationPath) is the real activatePackage
	// shape: it injects the nix env pair before asUser joins the argv.
	preset := installable.Preset{
		ProfilePath:   "/nix/var/nix/profiles/system",
		SetProfile:    new(true),
		IsSystemLevel: new(true),
	}

	require.NoError(t, Activate(exc, mach, preset, "/nix/store/abc-closure", "switch", "bob", nixCfg, nixver.FlavorNix))

	lines := testutil.CommandLines(t, phaseLog)
	require.NotEmpty(t, lines)

	joined := strings.Join(lines, "\n")
	assert.NotContains(t, joined, "SUPERSECRET", "secret values inside su -c must be redacted")
	assert.Contains(t, joined, "su -l bob -c")
	assert.Contains(t, joined, "'env' 'NIX_CONFIG=access-tokens=***'")
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

	nixCfg := &nix.NixConfig{}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, Activate(exc, mach, preset, "/nix/store/abc-closure", "test", "", nixCfg, nixver.FlavorNix))

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

// TestActivate_CommandOverrides verifies the activation-side nix invocations
// honor their command prefixes: profile set (nix-env) and package install
// (nix profile add).
func TestActivate_CommandOverrides(t *testing.T) {
	t.Parallel()

	preset := installable.Preset{
		ProfilePath:   "/nix/var/nix/profiles/system",
		SetProfile:    new(true),
		IsSystemLevel: new(true),
	}

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{
		NixEnvCommand:     []string{"/run/wrappers/bin/nix-env"},
		ProfileAddCommand: []string{"/run/wrappers/bin/nix", "profile", "add"},
	}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, Activate(exc, mach, preset, "/nix/store/abc-closure", "switch", "", nixCfg, nixver.FlavorNix))

	assert.Equal(t, []string{
		"/run/wrappers/bin/nix-env --profile /nix/var/nix/profiles/system --set /nix/store/abc-closure",
		"/run/wrappers/bin/nix profile add --extra-experimental-features nix-command flakes /nix/store/abc-closure",
	}, testutil.CommandLines(t, phaseLog))
}

// TestActivate_DefaultFlagOverrides pins that the operands panix would otherwise
// hardcode come from the configured flag lists: a replacement tool can reorder
// or decorate them, while $PANIX_PROFILE_PATH and $PANIX_CLOSURE still carry the
// values.
func TestActivate_DefaultFlagOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		preset    installable.Preset
		nix       nix.NixConfig
		wantLines []string
	}{
		{
			name:   "profile set reordered with an extra flag",
			preset: nixosCompositionPreset(),
			nix: nix.NixConfig{
				NixEnvSetDefaultFlags: []string{"--set", "$PANIX_CLOSURE", "--profile", "$PANIX_PROFILE_PATH", "--keep-going"},
			},
			wantLines: []string{
				"nix-env --set /nix/store/abc-closure --profile /nix/var/nix/profiles/system --keep-going",
				"/nix/store/abc-closure/bin/switch-to-configuration switch",
			},
		},
		{
			name: "package install keeps the closure with a priority operand",
			preset: installable.Preset{
				IsSystemLevel: new(true),
			},
			nix: nix.NixConfig{
				ProfileAddDefaultFlags: []string{"--priority", "10", "${PANIX_CLOSURE}"},
			},
			wantLines: []string{
				"nix profile add --extra-experimental-features nix-command flakes --priority 10 /nix/store/abc-closure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newLocalTransferMachine(t, true)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			nixCfg := tt.nix
			require.NoError(t, nixCfg.Init(nil))

			require.NoError(t, Activate(
				exc, mach, tt.preset, "/nix/store/abc-closure", "switch", "", &nixCfg, nixver.FlavorNix))

			assert.Equal(t, tt.wantLines, testutil.CommandLines(t, phaseLog))
		})
	}
}

// An explicit [] is taken as written, and the built-in commands refuse a list
// that dropped an operand they need instead of acting on a default profile or
// installing the working directory's package.
func TestActivate_DroppedOperandsFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		preset  installable.Preset
		nix     nix.NixConfig
		wantErr string
	}{
		{
			name: "nix-env set without the profile",
			preset: installable.Preset{
				ProfilePath: "/nix/var/nix/profiles/system",
				SetProfile:  new(true),
			},
			nix:     nix.NixConfig{NixEnvSetDefaultFlags: []string{"--set", "$PANIX_CLOSURE"}},
			wantErr: "nix_env_set_default_flags dropped $PANIX_PROFILE_PATH",
		},
		{
			name: "nix-env set without the closure",
			preset: installable.Preset{
				ProfilePath: "/nix/var/nix/profiles/system",
				SetProfile:  new(true),
			},
			nix:     nix.NixConfig{NixEnvSetDefaultFlags: []string{"--profile", "$PANIX_PROFILE_PATH"}},
			wantErr: "nix_env_set_default_flags dropped $PANIX_CLOSURE",
		},
		{
			name:    "package install without the closure",
			preset:  installable.Preset{IsSystemLevel: new(true)},
			nix:     nix.NixConfig{ProfileAddDefaultFlags: []string{"--priority", "10"}},
			wantErr: "profile_add_default_flags dropped $PANIX_CLOSURE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newLocalTransferMachine(t, true)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			nixCfg := tt.nix
			require.NoError(t, nixCfg.Init(nil))

			err := Activate(exc, mach, tt.preset, "/nix/store/abc-closure", "switch", "", &nixCfg, nixver.FlavorNix)

			require.ErrorContains(t, err, tt.wantErr)
			assert.Empty(t, testutil.CommandLines(t, phaseLog), "the refused step must not run")
		})
	}
}

// An explicit [] stays valid for a replacement command: panix does not inject
// operands into a tool it did not choose.
func TestActivate_EmptyFlagListRunsReplacementCommand(t *testing.T) {
	t.Parallel()

	preset := installable.Preset{
		ProfilePath:   "/nix/var/nix/profiles/system",
		SetProfile:    new(true),
		IsSystemLevel: new(true),
	}

	mach := newLocalTransferMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{NixEnvCommand: []string{"my-nix-env"}, NixEnvSetDefaultFlags: []string{}}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, Activate(exc, mach, preset, "/nix/store/abc-closure", "switch", "", nixCfg, nixver.FlavorNix))

	lines := testutil.CommandLines(t, phaseLog)
	require.NotEmpty(t, lines)
	assert.Equal(t, "my-nix-env", lines[0], "an explicit [] must not be replaced by the built-in operands")
}

// A flag list referencing a value the step does not supply fails the step with
// an error naming the field, rather than running a partially expanded command.
func TestActivate_UnavailableRuntimeVariableFails(t *testing.T) {
	t.Parallel()

	preset := installable.Preset{
		ProfilePath:   "/nix/var/nix/profiles/system",
		SetProfile:    new(true),
		IsSystemLevel: new(true),
	}

	mach := newLocalTransferMachine(t, true)
	exc, _ := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{NixEnvSetDefaultFlags: []string{"--set", "$PANIX_STORE_URL"}}
	require.NoError(t, nixCfg.Init(nil))

	err := Activate(exc, mach, preset, "/nix/store/abc-closure", "switch", "", nixCfg, nixver.FlavorNix)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nix_env_set_default_flags")
	assert.Contains(t, err.Error(), "PANIX_STORE_URL")
}
