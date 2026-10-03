package guard

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Composition fixtures shared by the step-list and gc-root tables.
const (
	testProfile  = "/nix/var/nix/profiles/system"
	testNixEnv   = "/run/current-system/sw/bin/nix-env"
	testNixStore = "/run/current-system/sw/bin/nix-store"
	testOld      = "/nix/store/old"
	testNew      = "/nix/store/new"
)

// presetOf returns the preset row for a built-in output type. The preset table
// (internal/config/tree/installable/presets.go) is the single source of truth
// for the guard tier and both script paths, so the compose tables take their
// inputs from it; a miss is a fixture bug.
func presetOf(t *testing.T, typ installable.FlakeOutputType) installable.Preset {
	t.Helper()

	preset, ok := installable.PresetForType(typ)
	require.True(t, ok, "table references unknown output type %q", typ)

	return preset
}

// TestComposeStepLists pins the commit/revert/activation transaction tables per
// output type and mode (spec 4.2): golden argv for every guarded type x mode,
// composed from the type's preset row.
func TestComposeStepLists(t *testing.T) {
	t.Parallel()

	type want struct {
		activation StepList
		commit     StepList
		revert     StepList
		invariant  string
		gate       bool
	}

	tests := []struct {
		name   string
		preset installable.Preset
		mode   string
		want   want
	}{
		{
			name:   "nixos switch: test-then-commit, revert re-runs OLD switch",
			preset: presetOf(t, "nixosConfigurations"), mode: "switch",
			want: want{
				activation: StepList{{testNew + "/bin/switch-to-configuration", "test"}},
				commit: StepList{
					{testNixEnv, "-p", testProfile, "--set", testNew},
					{testNew + "/bin/switch-to-configuration", "boot"},
				},
				revert:    StepList{{testOld + "/bin/switch-to-configuration", "switch"}},
				invariant: testNew,
				gate:      true,
			},
		},
		{
			name:   "nixos test: no commit, revert re-runs OLD test",
			preset: presetOf(t, "nixosConfigurations"), mode: "test",
			want: want{
				activation: StepList{{testNew + "/bin/switch-to-configuration", "test"}},
				commit:     nil,
				revert:     StepList{{testOld + "/bin/switch-to-configuration", "test"}},
				invariant:  testOld,
				gate:       false,
			},
		},
		{
			name:   "nixos boot: profile-first, revert restores the profile entry",
			preset: presetOf(t, "nixosConfigurations"), mode: "boot",
			want: want{
				activation: StepList{{testNew + "/bin/switch-to-configuration", "boot"}},
				commit:     nil,
				revert: StepList{
					{testNixEnv, "-p", testProfile, "--set", testOld},
					{testOld + "/bin/switch-to-configuration", "boot"},
				},
				invariant: testNew,
				gate:      false,
			},
		},
		{
			name:   "darwin: --set commit, OLD activate revert",
			preset: presetOf(t, "darwinConfigurations"), mode: "switch",
			want: want{
				activation: StepList{{testNew + "/activate"}},
				commit:     StepList{{testNixEnv, "-p", testProfile, "--set", testNew}},
				revert:     StepList{{testOld + "/activate"}},
				invariant:  testNew,
				gate:       true,
			},
		},
		{
			name:   "system-manager: register-profile commit, register+activate revert",
			preset: presetOf(t, "systemConfigs"), mode: "switch",
			want: want{
				activation: StepList{{testNew + "/bin/activate"}},
				commit:     StepList{{testNew + "/bin/register-profile"}},
				revert: StepList{
					{testOld + "/bin/register-profile"},
					{testOld + "/bin/activate"},
				},
				invariant: testNew,
				gate:      true,
			},
		},
		{
			name:   "home-manager: self-setting, OLD activate revert, gate without commit",
			preset: presetOf(t, "homeConfigurations"), mode: "switch",
			want: want{
				activation: StepList{{testNew + "/activate"}},
				commit:     nil,
				revert:     StepList{{testOld + "/activate"}},
				invariant:  testNew,
				gate:       true,
			},
		},
		{
			name:   "nix-on-droid: minimal, activation only",
			preset: presetOf(t, "nixOnDroidConfigurations"), mode: "switch",
			want: want{
				activation: StepList{{testNew + "/activate"}},
				commit:     nil,
				revert:     nil,
				invariant:  "",
				gate:       false,
			},
		},
		{
			name:   "nix-maid: minimal, bin/activate child",
			preset: presetOf(t, "maidConfigurations"), mode: "switch",
			want: want{
				activation: StepList{{testNew + "/bin/activate"}},
				commit:     nil,
				revert:     nil,
				invariant:  "",
				gate:       false,
			},
		},
		{
			name:   "packages: none, no transaction at all",
			preset: presetOf(t, "packages"), mode: "switch",
			want: want{activation: nil, commit: nil, revert: nil, invariant: "", gate: false},
		},
		{
			// Custom types read their own output_types declaration; without a
			// declared tier they stay on the legacy direct-activation path.
			name:   "custom type without a declared tier: none, no transaction at all",
			preset: installable.Preset{}, mode: "switch",
			want: want{activation: nil, commit: nil, revert: nil, invariant: "", gate: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := composeStepLists(tt.preset, tt.mode, tt.preset.ProfilePath, testNixEnv, testOld, testNew)

			assert.Equal(t, tt.want.activation, got.Activation, "activation argv")
			assert.Equal(t, tt.want.commit, got.Commit, "commit argv")
			assert.Equal(t, tt.want.revert, got.Revert, "revert argv")
			assert.Equal(t, tt.want.invariant, got.InvariantTarget, "invariant target")
			assert.Equal(t, tt.want.gate, got.GateEligible, "gate eligibility")
		})
	}
}

// TestGcRootTarget pins the authoritative gc-root table (spec 4.3).
func TestGcRootTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tier installable.GuardTier
		mode string
		want string
	}{
		{name: "nixos switch roots NEW", tier: installable.GuardTierFull, mode: "switch", want: testNew},
		{name: "nixos test roots NEW", tier: installable.GuardTierFull, mode: "test", want: testNew},
		{name: "nixos boot roots OLD", tier: installable.GuardTierFull, mode: "boot", want: testOld},
		{name: "darwin roots NEW", tier: installable.GuardTierStandard, mode: "switch", want: testNew},
		{name: "system-manager roots NEW", tier: installable.GuardTierStandard, mode: "switch", want: testNew},
		{name: "home-manager roots OLD", tier: installable.GuardTierSelfSetting, mode: "switch", want: testOld},
		{name: "home-manager profile-last roots NEW (modern, standard tier)", tier: installable.GuardTierStandard, mode: "switch", want: testNew},
		{name: "minimal roots nothing", tier: installable.GuardTierMinimal, mode: "switch", want: ""},
		{name: "none roots nothing", tier: installable.GuardTierNone, mode: "switch", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, gcRootTarget(tt.tier, tt.mode, testOld, testNew))
		})
	}
}

// TestComposeHMStepLists pins the profile-last home-manager transaction
// shapes (modern HM >= 25.11, resolveHMComposition's ProfileLast branch,
// spec 4.2): the darwin standard shape with the driver-1 flag on the
// activation child and on OLD's revert activation when OLD is modern. The
// legacy self-setting golden above (composeStepLists) pins byte-identical
// legacy behavior.
func TestComposeHMStepLists(t *testing.T) {
	t.Parallel()

	preset := presetOf(t, "homeConfigurations")

	t.Run("OLD modern carries the driver flag on the revert", func(t *testing.T) {
		t.Parallel()

		got := composeHMStepLists(preset, testProfile, testNixEnv, testOld, testNew, true)

		assert.Equal(t, StepList{{testNew + "/activate", "--driver-version", "1"}}, got.Activation)
		assert.Equal(t, StepList{{testNixEnv, "-p", testProfile, "--set", testNew}}, got.Commit)
		assert.Equal(t, StepList{{testOld + "/activate", "--driver-version", "1"}}, got.Revert)
		assert.Equal(t, testNew, got.InvariantTarget)
		assert.True(t, got.GateEligible, "the profile-last commit is non-empty: the gate applies")
	})

	t.Run("OLD legacy keeps the bare revert activate", func(t *testing.T) {
		t.Parallel()

		got := composeHMStepLists(preset, testProfile, testNixEnv, testOld, testNew, false)

		assert.Equal(t, StepList{{testOld + "/activate"}}, got.Revert)
		assert.Equal(t, StepList{{testNew + "/activate", "--driver-version", "1"}}, got.Activation)
		assert.Equal(t, StepList{{testNixEnv, "-p", testProfile, "--set", testNew}}, got.Commit)
		assert.True(t, got.GateEligible)
	})
}

// TestResolveHMComposition pins the per-deploy tier resolution: NEW modern
// composes profile-last (standard tier) with the OLD driver flag per OLD's
// probe; NEW legacy (or a non-HM preset) keeps the preset's static tier with
// no flags, byte-identical to today's composition.
func TestResolveHMComposition(t *testing.T) {
	t.Parallel()

	hm := presetOf(t, "homeConfigurations")
	nixos := presetOf(t, "nixosConfigurations")

	tests := []struct {
		name            string
		preset          installable.Preset
		newModern       bool
		oldModern       bool
		wantProfileLast bool
		wantOldModern   bool
	}{
		{name: "modern NEW and OLD: profile-last with the driver revert", preset: hm, newModern: true, oldModern: true, wantProfileLast: true, wantOldModern: true},
		{name: "modern NEW and legacy OLD: profile-last, bare revert", preset: hm, newModern: true, wantProfileLast: true},
		{name: "legacy NEW keeps self-setting unchanged", preset: hm},
		{name: "non-HM presets never resolve the HM branch", preset: nixos, newModern: true, oldModern: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveHMComposition(tt.preset, tt.newModern, tt.oldModern)

			assert.Equal(t, tt.wantProfileLast, got.ProfileLast, "ProfileLast")
			assert.Equal(t, tt.wantOldModern, got.OldModern, "OldModern")
		})
	}
}

// TestParseHMGenVersion pins the detection contract: integer >= 1 means
// driver-1 support; zero, unparseable, or absent content is legacy.
func TestParseHMGenVersion(t *testing.T) {
	t.Parallel()

	assert.True(t, parseHMGenVersion("1"), "gen-version 1 is modern")
	assert.True(t, parseHMGenVersion("2\n"), "whitespace-tolerant")
	assert.False(t, parseHMGenVersion("0"), "gen-version 0 is legacy")
	assert.False(t, parseHMGenVersion("abc"), "unparseable is legacy")
	assert.False(t, parseHMGenVersion(""), "missing is legacy")
	assert.False(t, parseHMGenVersion("-3"), "negative integers are legacy")
}

// TestBootModeGateIneligible pins the static gate-eligibility verdict the
// config validator consumes against the runtime composition: the static
// predicate and composeStepLists must agree wherever both can answer, so
// config-time validation cannot drift from the runtime gate (spec 2, 5).
func TestBootModeGateIneligible(t *testing.T) {
	t.Parallel()

	// The preset table is the same single source of truth the composition
	// reads, so truth-table cross-checked over every preset x boot-mode shape
	// the transaction can take.
	presets := []installable.Preset{
		presetOf(t, "nixosConfigurations"),  // full tier: boot commit is empty
		presetOf(t, "darwinConfigurations"), // standard tier: boot activation runs the preset's script, no full-tier boot rule
		presetOf(t, "systemConfigs"),        // standard tier
		presetOf(t, "homeConfigurations"),   // self-setting
		presetOf(t, "packages"),             // none tier
		{},                                  // unnamed custom without a declared tier
	}

	for _, preset := range presets {
		assert.False(t, BootModeGateIneligible(preset, "switch"), "the rule is boot-only: %v", preset.GuardTierValue())
		assert.False(t, BootModeGateIneligible(preset, "test"), "the rule is boot-only: %v", preset.GuardTierValue())
	}

	// Full tier boot: statically gate-ineligible, matching the composed
	// (runtime) gate eligibility for the same shape.
	nixPreset := presetOf(t, "nixosConfigurations")
	assert.True(t, BootModeGateIneligible(nixPreset, "boot"))

	steps := composeStepLists(nixPreset, "boot", testProfile, testNixEnv, testOld, testNew)
	assert.False(t, steps.GateEligible, "the runtime composition agrees: boot mode is not gate-worthy")

	// Standard-tier boot keeps its non-empty commit, so it stays eligible.
	steps = composeStepLists(presetOf(t, "darwinConfigurations"), "boot", testProfile, testNixEnv, testOld, testNew)
	assert.True(t, steps.GateEligible)
	assert.False(t, BootModeGateIneligible(presetOf(t, "darwinConfigurations"), "boot"))
}
