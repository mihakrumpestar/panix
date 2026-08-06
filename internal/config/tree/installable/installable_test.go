package installable

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyPresetDefaults_TypeLevelFields verifies that type-level fields
// (IsSystemLevel, IsBootstrappable, OmitTypeFromAttrPath) are always taken
// from the type defaults, ignoring any user-provided values. This is the fix
// for the pre-existing bug where `if !x { x = defaults }` silently overwrote
// an explicit `false` when the default was `true`.
func TestApplyPresetDefaults_TypeLevelFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		userPreset Preset
		defaults   Preset
	}{
		{
			name: "user false on true defaults is overwritten (packages-like: omit=true)",
			userPreset: Preset{
				IsSystemLevel:        false, // user tries to override
				IsBootstrappable:     false,
				OmitTypeFromAttrPath: false, // user tries to disable omit
			},
			defaults: Preset{
				IsSystemLevel:        false,
				IsBootstrappable:     false,
				OmitTypeFromAttrPath: true,
			},
		},
		{
			name: "user true on false defaults is overwritten (nixos-like)",
			userPreset: Preset{
				IsSystemLevel:        true, // user tries to set system-level on a user-level type
				IsBootstrappable:     true, // user tries to enable bootstrap
				OmitTypeFromAttrPath: true, // user tries to enable omit
			},
			defaults: Preset{
				IsSystemLevel:        true,
				IsBootstrappable:     true,
				OmitTypeFromAttrPath: false,
			},
		},
		{
			name: "nixosConfigurations defaults: system-level and bootstrappable enforced",
			userPreset: Preset{
				IsSystemLevel:        false,
				IsBootstrappable:     false,
				OmitTypeFromAttrPath: true,
			},
			defaults: presets[FlakeOutputType("nixosConfigurations")],
		},
		{
			name: "packages defaults: omit-type enforced, not system-level, not bootstrappable",
			userPreset: Preset{
				IsSystemLevel:        true,
				IsBootstrappable:     true,
				OmitTypeFromAttrPath: false,
			},
			defaults: presets[FlakeOutputType("packages")],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inst := &Installable{Preset: tt.userPreset}
			inst.applyPresetDefaults(tt.defaults)
			assertTypeLevelFields(t, inst, tt.defaults)
		})
	}
}

func assertTypeLevelFields(t *testing.T, inst *Installable, defaults Preset) {
	t.Helper()
	assert.Equal(t, defaults.IsSystemLevel, inst.Preset.IsSystemLevel, "IsSystemLevel should always come from defaults")
	assert.Equal(t, defaults.IsBootstrappable, inst.Preset.IsBootstrappable, "IsBootstrappable should always come from defaults")
	assert.Equal(t, defaults.OmitTypeFromAttrPath, inst.Preset.OmitTypeFromAttrPath, "OmitTypeFromAttrPath should always come from defaults")
}

// TestApplyPresetDefaults_UserOverridableFields verifies that user-overridable
// fields use the user value when non-zero, and fall back to defaults when zero.
func TestApplyPresetDefaults_UserOverridableFields(t *testing.T) {
	t.Parallel()

	t.Run("user values preserved when non-zero", func(t *testing.T) {
		t.Parallel()

		userPreset := Preset{
			BuildPath:             "custom.build.path",
			ProfilePath:           "/custom/profile",
			ActivationPath:        "bin/custom-activate",
			ActivationModes:       []string{"custom-mode"},
			ActivationDefaultMode: "custom-mode",
		}
		defaults := Preset{
			BuildPath:             "default.build.path",
			ProfilePath:           "/default/profile",
			ActivationPath:        "bin/default-activate",
			ActivationModes:       []string{"switch", "boot"},
			ActivationDefaultMode: "switch",
		}

		inst := &Installable{Preset: userPreset}
		inst.applyPresetDefaults(defaults)

		assert.Equal(t, "custom.build.path", inst.Preset.BuildPath)
		assert.Equal(t, "/custom/profile", inst.Preset.ProfilePath)
		assert.Equal(t, "bin/custom-activate", inst.Preset.ActivationPath)
		assert.Equal(t, []string{"custom-mode"}, inst.Preset.ActivationModes)
		assert.Equal(t, "custom-mode", inst.Preset.ActivationDefaultMode)
	})

	t.Run("zero values fall back to defaults", func(t *testing.T) {
		t.Parallel()

		defaults := Preset{
			BuildPath:             "config.system.build.toplevel",
			ProfilePath:           "/nix/var/nix/profiles/system",
			ActivationPath:        "bin/switch-to-configuration",
			ActivationModes:       []string{"switch", "boot", "test", "dry-activate"},
			ActivationDefaultMode: "switch",
		}

		inst := &Installable{Preset: Preset{}}
		inst.applyPresetDefaults(defaults)

		assert.Equal(t, defaults.BuildPath, inst.Preset.BuildPath)
		assert.Equal(t, defaults.ProfilePath, inst.Preset.ProfilePath)
		assert.Equal(t, defaults.ActivationPath, inst.Preset.ActivationPath)
		assert.Equal(t, defaults.ActivationModes, inst.Preset.ActivationModes)
		assert.Equal(t, defaults.ActivationDefaultMode, inst.Preset.ActivationDefaultMode)
	})
}

// TestApplyPresetDefaults_AllSixTypes exercises the full Init() ->
// applyPresetDefaults() flow for every known output type. Starting from an
// empty Preset, Init must populate every field from the type's preset entry in
// the presets map. This catches integration issues that the field-level
// applyPresetDefaults tests above might miss (e.g. Init not calling
// applyPresetDefaults, or passing the wrong type key).
func TestApplyPresetDefaults_AllSixTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  FlakeOutputType
	}{
		{"nixosConfigurations", FlakeOutputType("nixosConfigurations")},
		{"darwinConfigurations", FlakeOutputType("darwinConfigurations")},
		{"systemConfigs", FlakeOutputType("systemConfigs")},
		{"homeConfigurations", FlakeOutputType("homeConfigurations")},
		{"nixOnDroidConfigurations", FlakeOutputType("nixOnDroidConfigurations")},
		{"packages", FlakeOutputType("packages")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Start with an empty Preset — all fields are zero values,
			// so applyPresetDefaults should fill in every field from the
			// type's preset entry.
			inst := &Installable{Preset: Preset{}}

			// Init needs a non-nil parent Attributes (it merges parent into
			// child and dereferences parent.Xpath). Use a fresh empty
			// Attributes, matching how fleet.Init() calls attributes.New().
			// parentNix can be nil — NixConfig.Init handles nil.
			err := inst.Init(tt.typ, "testname", attributes.New(), nil)
			require.NoError(t, err, "Init should succeed for %s", tt.typ)

			expected := presets[tt.typ]

			// User-overridable fields — all should come from defaults since
			// the input Preset was empty.
			assert.Equal(t, expected.BuildPath, inst.Preset.BuildPath, "BuildPath")
			assert.Equal(t, expected.ProfilePath, inst.Preset.ProfilePath, "ProfilePath")
			assert.Equal(t, expected.ActivationPath, inst.Preset.ActivationPath, "ActivationPath")
			assert.Equal(t, expected.ActivationModes, inst.Preset.ActivationModes, "ActivationModes")
			assert.Equal(t, expected.ActivationDefaultMode, inst.Preset.ActivationDefaultMode, "ActivationDefaultMode")

			// SetProfile is *bool; compare via dereference, handling nil.
			if expected.SetProfile == nil {
				assert.Nil(t, inst.Preset.SetProfile, "SetProfile should be nil")
			} else if assert.NotNil(t, inst.Preset.SetProfile, "SetProfile should not be nil") {
				assert.Equal(t, *expected.SetProfile, *inst.Preset.SetProfile, "SetProfile value")
			}

			// Type-level fields — always from defaults.
			assert.Equal(t, expected.IsSystemLevel, inst.Preset.IsSystemLevel, "IsSystemLevel")
			assert.Equal(t, expected.IsBootstrappable, inst.Preset.IsBootstrappable, "IsBootstrappable")
			assert.Equal(t, expected.OmitTypeFromAttrPath, inst.Preset.OmitTypeFromAttrPath, "OmitTypeFromAttrPath")

			// Init must also set Type and Name from the keys.
			assert.Equal(t, tt.typ, inst.Type, "Type should be set by Init")
			assert.Equal(t, AttributeName("testname"), inst.Name, "Name should be set by Init")
		})
	}
}

// TestInit_NilParentNixIsSafe verifies that Init tolerates a nil parentNix.
// This matches the real call chain where the top-level fleet passes nil to
// NixConfig.Init. A regression here would crash every config load.
func TestInit_NilParentNixIsSafe(t *testing.T) {
	t.Parallel()

	inst := &Installable{Preset: Preset{}}

	err := inst.Init(FlakeOutputType("nixosConfigurations"), "host1", attributes.New(), nil)
	require.NoError(t, err)
	// BuildMode defaults to local when unset (see NixConfig.Init).
	assert.Equal(t, nix.BuildModeLocal, inst.Nix.BuildMode)
}
