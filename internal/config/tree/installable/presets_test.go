package installable

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// presetExpectedTest pins every preset field of one output type, so unintended
// drift in the presets map cannot silently change behavior.
type presetExpectedTest struct {
	name string
	typ  FlakeOutputType
	want Preset
}

var allPresetsExpectedTests = []presetExpectedTest{
	{
		name: "nixosConfigurations",
		typ:  FlakeOutputType("nixosConfigurations"),
		want: Preset{
			BuildPath:             "config.system.build.toplevel",
			ProfilePath:           "/nix/var/nix/profiles/system",
			SetProfile:            new(true),
			IsSystemLevel:         new(true),
			Bootstrap:             BootstrapNixOS,
			ActivationPath:        "bin/switch-to-configuration",
			ActivationModes:       []string{"switch", "boot", "test", "dry-activate"},
			ProfileSkipModes:      []string{"test", "dry-activate"},
			NonMutatingModes:      []string{"dry-activate"},
			ActivationDefaultMode: "switch",
			OmitTypeFromAttrPath:  false,
		},
	},
	{
		name: "darwinConfigurations",
		typ:  FlakeOutputType("darwinConfigurations"),
		want: Preset{
			BuildPath:             "system",
			ProfilePath:           "/nix/var/nix/profiles/system",
			SetProfile:            new(true),
			IsSystemLevel:         new(true),
			Bootstrap:             BootstrapNixInstall,
			ActivationPath:        "activate",
			ActivationModes:       nil,
			ActivationDefaultMode: "",
			OmitTypeFromAttrPath:  false,
		},
	},
	{
		name: "systemConfigs",
		typ:  FlakeOutputType("systemConfigs"),
		want: Preset{
			BuildPath:             "",
			ProfilePath:           "/nix/var/nix/profiles/system-manager-profiles",
			SetProfile:            new(true),
			IsSystemLevel:         new(true),
			Bootstrap:             BootstrapNixInstall,
			ActivationPath:        "bin/activate",
			ActivationModes:       nil,
			ActivationDefaultMode: "",
			OmitTypeFromAttrPath:  false,
		},
	},
	{
		name: "homeConfigurations",
		typ:  FlakeOutputType("homeConfigurations"),
		want: Preset{
			BuildPath:             "activationPackage",
			ProfilePath:           "~/.local/state/nix/profiles/home-manager",
			SetProfile:            nil,
			IsSystemLevel:         new(false),
			Bootstrap:             BootstrapNixInstall,
			ActivationPath:        "activate",
			ActivationModes:       nil,
			ActivationDefaultMode: "",
			OmitTypeFromAttrPath:  false,
		},
	},
	{
		name: "nixOnDroidConfigurations",
		typ:  FlakeOutputType("nixOnDroidConfigurations"),
		want: Preset{
			BuildPath:             "build.activationPackage",
			ProfilePath:           "~/.local/state/nix/profiles/nix-on-droid",
			SetProfile:            nil,
			IsSystemLevel:         new(false),
			Bootstrap:             BootstrapNixInstall,
			ActivationPath:        "activate",
			ActivationModes:       nil,
			ActivationDefaultMode: "",
			OmitTypeFromAttrPath:  false,
		},
	},
	{
		name: "packages",
		typ:  FlakeOutputType("packages"),
		want: Preset{
			BuildPath:             "",
			ProfilePath:           "",
			SetProfile:            nil,
			IsSystemLevel:         new(false),
			Bootstrap:             BootstrapNixInstall,
			ActivationPath:        "",
			ActivationModes:       nil,
			ActivationDefaultMode: "",
			OmitTypeFromAttrPath:  true,
		},
	},
	{
		name: "maidConfigurations",
		typ:  FlakeOutputType("maidConfigurations"),
		want: Preset{
			BuildPath:             "",
			ProfilePath:           "",
			SetProfile:            nil,
			IsSystemLevel:         new(false),
			Bootstrap:             BootstrapNixInstall,
			ActivationPath:        "bin/activate",
			ActivationModes:       nil,
			ActivationDefaultMode: "",
			OmitTypeFromAttrPath:  false,
		},
	},
}

func assertPresetFields(t *testing.T, typ FlakeOutputType, got, want Preset) {
	t.Helper()
	assert.Equal(t, want.OutputTypeAttr, got.OutputTypeAttr, "%s OutputTypeAttr", typ)
	assert.Equal(t, want.BuildPath, got.BuildPath, "%s BuildPath", typ)
	assert.Equal(t, want.ProfilePath, got.ProfilePath, "%s ProfilePath", typ)
	assert.Equal(t, want.ActivationPath, got.ActivationPath, "%s ActivationPath", typ)
	assert.Equal(t, want.ActivationDefaultMode, got.ActivationDefaultMode, "%s ActivationDefaultMode", typ)
	assert.Equal(t, want.ActivationModes, got.ActivationModes, "%s ActivationModes", typ)
	assert.Equal(t, want.NonMutatingModes, got.NonMutatingModes, "%s NonMutatingModes", typ)
	assert.Equal(t, want.ProfileSkipModes, got.ProfileSkipModes, "%s ProfileSkipModes", typ)
	assert.Equal(t, want.IsSystemLevel, got.IsSystemLevel, "%s IsSystemLevel", typ)
	assert.Equal(t, want.Bootstrap, got.Bootstrap, "%s Bootstrap", typ)
	assert.Equal(t, want.OmitTypeFromAttrPath, got.OmitTypeFromAttrPath, "%s OmitTypeFromAttrPath", typ)

	if want.SetProfile == nil {
		assert.Nil(t, got.SetProfile, "%s SetProfile should be nil", typ)
	} else if assert.NotNil(t, got.SetProfile, "%s SetProfile should not be nil", typ) {
		assert.Equal(t, *want.SetProfile, *got.SetProfile, "%s SetProfile value", typ)
	}
}

func TestAllPresetsExpectedValues(t *testing.T) {
	t.Parallel()

	for _, tt := range allPresetsExpectedTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := presets[tt.typ]
			assert.True(t, ok, "preset for %s should exist in the presets map", tt.typ)
			assertPresetFields(t, tt.typ, got, tt.want)

			// Lookup helpers must agree with the map.
			assert.True(t, tt.typ.IsKnown(), "IsKnown() should return true for %s", tt.typ)
		})
	}
}

// Exactly 7 types are known; unknown and empty types are rejected.
func TestKnownOutputTypes(t *testing.T) {
	t.Parallel()

	known := KnownOutputTypes()

	assert.Len(t, known, 7, "exactly 7 output types should be known")

	expected := []FlakeOutputType{
		FlakeOutputType("nixosConfigurations"),
		FlakeOutputType("darwinConfigurations"),
		FlakeOutputType("systemConfigs"),
		FlakeOutputType("homeConfigurations"),
		FlakeOutputType("nixOnDroidConfigurations"),
		FlakeOutputType("packages"),
		FlakeOutputType("maidConfigurations"),
	}

	// Order isn't guaranteed (map iteration), so compare as sets.
	for _, typ := range expected {
		assert.True(t, typ.IsKnown(), "%s should be known", typ)
		assert.True(t, slices.Contains(known, typ), "KnownOutputTypes should contain %s", typ)
	}

	// Unknown types must not be known.
	assert.False(t, FlakeOutputType("unknownType").IsKnown(),
		"unknown type should not be known")
	assert.False(t, FlakeOutputType("").IsKnown(),
		"empty type should not be known")
}

// The mode helpers are the single classification of a preset's bootstrap
// behavior: NixOS bootstrap and Nix install are mutually exclusive, and any
// mode is bootstrappable.
func TestPresetBootstrapModeHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		mode                 BootstrapMode
		wantBootstrapsNixOS  bool
		wantBootstrapsNix    bool
		wantIsBootstrappable bool
	}{
		{"none is not bootstrappable", BootstrapNone, false, false, false},
		{"nixos bootstraps NixOS only", BootstrapNixOS, true, false, true},
		{"nix-install bootstraps Nix only", BootstrapNixInstall, false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			preset := Preset{Bootstrap: tt.mode}
			assert.Equal(t, tt.wantBootstrapsNixOS, preset.BootstrapsNixOS())
			assert.Equal(t, tt.wantBootstrapsNix, preset.BootstrapsNix())
			assert.Equal(t, tt.wantIsBootstrappable, preset.IsBootstrappable())
		})
	}
}

// Pins the docs contract: a new built-in type fails the suite until documented,
// and DocOrder must be positive and distinct so the rendered output-type tables
// are deterministic.
func TestBuiltinPresetsAreDocumented(t *testing.T) {
	t.Parallel()

	orders := make(map[int]FlakeOutputType, len(presets))

	for typ, preset := range presets {
		assert.NotEmpty(t, preset.DocDeploys, "%s DocDeploys", typ)
		assert.NotEmpty(t, preset.DocActivation, "%s DocActivation", typ)
		assert.Positive(t, preset.DocOrder, "%s DocOrder", typ)
		assert.NotContains(t, orders, preset.DocOrder,
			"DocOrder %d is used by more than one built-in type", preset.DocOrder)

		orders[preset.DocOrder] = typ
	}

	rows := OutputTypeTable()
	assert.Len(t, rows, len(presets), "OutputTypeTable must return one row per built-in preset")

	renderedOrders := make([]int, 0, len(rows))
	for _, row := range rows {
		renderedOrders = append(renderedOrders, presets[FlakeOutputType(row.Type)].DocOrder)
	}

	assert.IsIncreasing(t, renderedOrders, "OutputTypeTable must be sorted by DocOrder")
}

// Pins the cross-field invariants that encode domain semantics (rollback needs
// a profile, bootstrap needs root, NixOS bootstrap builds the system toplevel,
// bare-name types have no build path, mode lists may only reference declared
// modes). A violation is an internal preset contradiction that would surface
// as a runtime error.
func TestPresetConsistency(t *testing.T) {
	t.Parallel()

	for _, typ := range KnownOutputTypes() {
		t.Run(string(typ), func(t *testing.T) {
			t.Parallel()

			preset, ok := presets[typ]
			assert.True(t, ok, "preset should exist for known type %s", typ)

			// No activation path means no rollback; packages is the canonical
			// case, a bare package with nothing to activate or roll back.
			if preset.ActivationPath == "" {
				assert.Empty(t, preset.ProfilePath,
					"%s: ActivationPath is empty so ProfilePath must also be empty (no rollback target)", typ)
			}

			// Bootstrap invariants (root and system toplevel) are grouped
			// in their own check.
			assertBootstrapInvariants(t, typ, preset)

			// Bare names (omit-type) have nothing to append a build path suffix to.
			if preset.OmitTypeFromAttrPath {
				assert.Empty(t, preset.BuildPath,
					"%s: OmitTypeFromAttrPath is true so BuildPath must be empty (bare name, no suffix)", typ)
			}

			// SetProfile == true requires a non-empty profile path.
			if preset.SetProfile != nil && *preset.SetProfile {
				assert.NotEmpty(t, preset.ProfilePath,
					"%s: SetProfile is true but ProfilePath is empty (can't set a profile that doesn't exist)", typ)
			}

			// No profile path => SetProfile must be nil (nothing to set).
			if preset.ProfilePath == "" {
				assert.Nil(t, preset.SetProfile,
					"%s: ProfilePath is empty so SetProfile must be nil (no profile to set)", typ)
			}

			// Mode lists may only reference modes the activation script understands.
			for _, mode := range preset.NonMutatingModes {
				assert.Contains(t, preset.ActivationModes, mode,
					"%s: NonMutatingModes entry %q must be in ActivationModes", typ, mode)
			}

			for _, mode := range preset.ProfileSkipModes {
				assert.Contains(t, preset.ActivationModes, mode,
					"%s: ProfileSkipModes entry %q must be in ActivationModes", typ, mode)
			}
		})
	}
}

// assertBootstrapInvariants pins the bootstrap cross-field invariants: only
// system-level types can bootstrap NixOS (kexec, disko and nixos-install all
// need root; nix-install works at any level), and the NixOS bootstrap must
// build the NixOS system toplevel because it installs the built system
// closure and resolves the disko script under the same output (the same rule
// validation enforces for declared bootstrap_mode: nixos types).
func assertBootstrapInvariants(t *testing.T, typ FlakeOutputType, preset Preset) {
	t.Helper()

	if !preset.IsSystemLevelValue() {
		assert.False(t, preset.BootstrapsNixOS(),
			"%s: non-system-level types cannot bootstrap NixOS (bootstrap needs root)", typ)
	}

	if preset.BootstrapsNixOS() {
		assert.Equal(t, NixOSSystemBuildPath, preset.BuildPath,
			"%s: NixOS bootstrap must build the system toplevel (nixos-install installs the built closure)", typ)
	}
}
