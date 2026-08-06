package installable

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAllPresetsExpectedValues is a table-driven test that asserts every field
// of every preset matches the expected value. This guards against accidental
// drift in the presets map (e.g. someone changing a build path or flipping a
// type-level flag) which would silently change panix's behavior for that
// output type.
//
// Each row also verifies IsKnown() and IsBootstrappableType() so that the
// lookup helpers stay consistent with the map contents.
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
			IsSystemLevel:         true,
			IsBootstrappable:      true,
			ActivationPath:        "bin/switch-to-configuration",
			ActivationModes:       []string{"switch", "boot", "test", "dry-activate"},
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
			IsSystemLevel:         true,
			IsBootstrappable:      false,
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
			IsSystemLevel:         true,
			IsBootstrappable:      false,
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
			IsSystemLevel:         false,
			IsBootstrappable:      false,
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
			IsSystemLevel:         false,
			IsBootstrappable:      false,
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
			IsSystemLevel:         false,
			IsBootstrappable:      false,
			ActivationPath:        "",
			ActivationModes:       nil,
			ActivationDefaultMode: "",
			OmitTypeFromAttrPath:  true,
		},
	},
}

func assertPresetFields(t *testing.T, typ FlakeOutputType, got, want Preset) {
	t.Helper()
	assert.Equal(t, want.BuildPath, got.BuildPath, "%s BuildPath", typ)
	assert.Equal(t, want.ProfilePath, got.ProfilePath, "%s ProfilePath", typ)
	assert.Equal(t, want.ActivationPath, got.ActivationPath, "%s ActivationPath", typ)
	assert.Equal(t, want.ActivationDefaultMode, got.ActivationDefaultMode, "%s ActivationDefaultMode", typ)
	assert.Equal(t, want.ActivationModes, got.ActivationModes, "%s ActivationModes", typ)
	assert.Equal(t, want.IsSystemLevel, got.IsSystemLevel, "%s IsSystemLevel", typ)
	assert.Equal(t, want.IsBootstrappable, got.IsBootstrappable, "%s IsBootstrappable", typ)
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
			assert.Equal(t, tt.want.IsBootstrappable, IsBootstrappableType(tt.typ),
				"IsBootstrappableType() should match preset's IsBootstrappable for %s", tt.typ)
		})
	}
}

// TestKnownOutputTypes verifies that KnownOutputTypes returns exactly the 6
// supported output types, that IsKnown returns true for each, and that an
// unknown type is rejected.
func TestKnownOutputTypes(t *testing.T) {
	t.Parallel()

	known := KnownOutputTypes()

	assert.Len(t, known, 6, "exactly 6 output types should be known")

	expected := []FlakeOutputType{
		FlakeOutputType("nixosConfigurations"),
		FlakeOutputType("darwinConfigurations"),
		FlakeOutputType("systemConfigs"),
		FlakeOutputType("homeConfigurations"),
		FlakeOutputType("nixOnDroidConfigurations"),
		FlakeOutputType("packages"),
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

// TestIsBootstrappableType verifies that only nixosConfigurations is
// bootstrappable. All other known types (and unknown types) return false.
func TestIsBootstrappableType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  FlakeOutputType
		want bool
	}{
		{"nixosConfigurations is bootstrappable", FlakeOutputType("nixosConfigurations"), true},
		{"darwinConfigurations is not bootstrappable", FlakeOutputType("darwinConfigurations"), false},
		{"systemConfigs is not bootstrappable", FlakeOutputType("systemConfigs"), false},
		{"homeConfigurations is not bootstrappable", FlakeOutputType("homeConfigurations"), false},
		{"nixOnDroidConfigurations is not bootstrappable", FlakeOutputType("nixOnDroidConfigurations"), false},
		{"packages is not bootstrappable", FlakeOutputType("packages"), false},
		{"unknown type is not bootstrappable", FlakeOutputType("nope"), false},
		{"empty type is not bootstrappable", FlakeOutputType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsBootstrappableType(tt.typ)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPresetConsistency verifies cross-field logical invariants that must hold
// for every preset. These rules encode the domain semantics:
//
//   - A type with no activation path (packages) has nothing to roll back to,
//     so it must not define a profile path.
//   - Only system-level types can bootstrap (user-level types run as a user,
//     not root, so they can't provision a system from scratch).
//   - Types that omit the output type from the attrpath (packages) use bare
//     names, so a build path suffix would be meaningless and must be empty.
//   - If SetProfile is true, there must be a profile path to set.
//   - If there's no profile path, SetProfile must be nil (nothing to set).
//
// If any of these fail, the preset map has an internal contradiction that
// would cause runtime errors (e.g. trying to set a profile that doesn't
// exist, or activating a package that has no activation script).
func TestPresetConsistency(t *testing.T) {
	t.Parallel()

	for _, typ := range KnownOutputTypes() {
		t.Run(string(typ), func(t *testing.T) {
			t.Parallel()

			preset, ok := presets[typ]
			assert.True(t, ok, "preset should exist for known type %s", typ)

			// No activation path => no rollback => no profile path.
			// packages is the canonical case: it's a bare package, nothing to
			// activate or roll back.
			if preset.ActivationPath == "" {
				assert.Empty(t, preset.ProfilePath,
					"%s: ActivationPath is empty so ProfilePath must also be empty (no rollback target)", typ)
			}

			// Only system-level types can bootstrap.
			if !preset.IsSystemLevel {
				assert.False(t, preset.IsBootstrappable,
					"%s: non-system-level types cannot be bootstrappable (bootstrap needs root)", typ)
			}

			// Omit-type types use bare names; a build path suffix would be
			// appended to a bare name, which is wrong for packages.
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
		})
	}
}
