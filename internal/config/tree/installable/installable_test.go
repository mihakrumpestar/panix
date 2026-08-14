package installable

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
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
				IsSystemLevel:        new(false), // user tries to override
				IsBootstrappable:     false,
				OmitTypeFromAttrPath: false, // user tries to disable omit
			},
			defaults: Preset{
				IsSystemLevel:        new(false),
				IsBootstrappable:     false,
				OmitTypeFromAttrPath: true,
			},
		},
		{
			name: "user true on false defaults is overwritten (nixos-like)",
			userPreset: Preset{
				IsSystemLevel:        new(true), // user tries to set system-level on a user-level type
				IsBootstrappable:     true,      // user tries to enable bootstrap
				OmitTypeFromAttrPath: true,      // user tries to enable omit
			},
			defaults: Preset{
				IsSystemLevel:        new(true),
				IsBootstrappable:     true,
				OmitTypeFromAttrPath: false,
			},
		},
		{
			name: "nixosConfigurations defaults: system-level and bootstrappable enforced",
			userPreset: Preset{
				IsSystemLevel:        new(false),
				IsBootstrappable:     false,
				OmitTypeFromAttrPath: true,
			},
			defaults: presets[FlakeOutputType("nixosConfigurations")],
		},
		{
			name: "packages defaults: omit-type enforced, not system-level, not bootstrappable",
			userPreset: Preset{
				IsSystemLevel:        new(true),
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
			OutputTypeAttr:        "nixosConf",
			BuildPath:             "custom.build.path",
			ProfilePath:           "/custom/profile",
			ActivationPath:        "bin/custom-activate",
			ActivationModes:       []string{"custom-mode"},
			ActivationDefaultMode: "custom-mode",
		}
		defaults := Preset{
			OutputTypeAttr:        "nixosConfigurations",
			BuildPath:             "default.build.path",
			ProfilePath:           "/default/profile",
			ActivationPath:        "bin/default-activate",
			ActivationModes:       []string{"switch", "boot"},
			ActivationDefaultMode: "switch",
		}

		inst := &Installable{Preset: userPreset}
		inst.applyPresetDefaults(defaults)

		assert.Equal(t, "nixosConf", inst.Preset.OutputTypeAttr)
		assert.Equal(t, "custom.build.path", inst.Preset.BuildPath)
		assert.Equal(t, "/custom/profile", inst.Preset.ProfilePath)
		assert.Equal(t, "bin/custom-activate", inst.Preset.ActivationPath)
		assert.Equal(t, []string{"custom-mode"}, inst.Preset.ActivationModes)
		assert.Equal(t, "custom-mode", inst.Preset.ActivationDefaultMode)
	})

	t.Run("zero values fall back to defaults", func(t *testing.T) {
		t.Parallel()

		defaults := Preset{
			OutputTypeAttr:        "nixosConf",
			BuildPath:             "config.system.build.toplevel",
			ProfilePath:           "/nix/var/nix/profiles/system",
			ActivationPath:        "bin/switch-to-configuration",
			ActivationModes:       []string{"switch", "boot", "test", "dry-activate"},
			ActivationDefaultMode: "switch",
		}

		inst := &Installable{Preset: Preset{}}
		inst.applyPresetDefaults(defaults)

		assert.Equal(t, "nixosConf", inst.Preset.OutputTypeAttr)
		assert.Equal(t, defaults.BuildPath, inst.Preset.BuildPath)
		assert.Equal(t, defaults.ProfilePath, inst.Preset.ProfilePath)
		assert.Equal(t, defaults.ActivationPath, inst.Preset.ActivationPath)
		assert.Equal(t, defaults.ActivationModes, inst.Preset.ActivationModes)
		assert.Equal(t, defaults.ActivationDefaultMode, inst.Preset.ActivationDefaultMode)
	})
}

// TestApplyPresetDefaults_AllKnownTypes exercises the full Init() ->
// applyPresetDefaults() flow for every known output type. Starting from an
// empty Preset, Init must populate every field from the type's preset entry in
// the presets map. This catches integration issues that the field-level
// applyPresetDefaults tests above might miss (e.g. Init not calling
// applyPresetDefaults, or passing the wrong type key).
func TestApplyPresetDefaults_AllKnownTypes(t *testing.T) {
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
		{"maidConfigurations", FlakeOutputType("maidConfigurations")},
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
			err := inst.Init(tt.typ, "testname", attributes.New(), nil, nil)
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

	err := inst.Init(FlakeOutputType("nixosConfigurations"), "host1", attributes.New(), nil, nil)
	require.NoError(t, err)
	// BuildMode defaults to local when unset (see NixConfig.Init).
	assert.Equal(t, nix.BuildModeLocal, inst.Nix.BuildMode)
}

// TestInitCustomPresets verifies that Installable.Init applies a declared
// custom preset (from output_types) as defaults with the same merge semantics
// as built-in presets: installable-level YAML overrides win for user-
// overridable fields, and type-level fields are always taken from the custom
// preset.
func TestInitCustomPresets(t *testing.T) {
	t.Parallel()

	customPresets := atomicorderedmap.New[string, Preset]()
	customPresets.Set("colmenaConfigurations", Preset{
		BuildPath:             "config.system.build.toplevel",
		ProfilePath:           "/nix/var/nix/profiles/system",
		SetProfile:            new(true),
		IsSystemLevel:         new(true),
		ActivationPath:        "bin/switch-to-configuration",
		ActivationModes:       []string{"switch", "boot"},
		ActivationDefaultMode: "switch",
	})

	t.Run("empty preset merges all custom defaults", func(t *testing.T) {
		t.Parallel()

		assertCustomPresetDefaultsMerged(t, customPresets)
	})

	t.Run("installable-level overrides win over custom defaults", func(t *testing.T) {
		t.Parallel()

		inst := &Installable{Preset: Preset{
			ActivationDefaultMode: "boot",
		}}

		err := inst.Init(FlakeOutputType("colmenaConfigurations"), "cfg0", attributes.New(), nil, customPresets)
		require.NoError(t, err)

		assert.Equal(t, "boot", inst.Preset.ActivationDefaultMode,
			"installable-level value should override the custom default")
		assert.Equal(t, "config.system.build.toplevel", inst.Preset.BuildPath,
			"unset installable-level fields should fall back to the custom default")
	})

	t.Run("type-level fields always come from the custom preset", func(t *testing.T) {
		t.Parallel()

		// Per-installable YAML tries to flip system_level to false, but it's a
		// type-level field, so the declared custom preset wins.
		inst := &Installable{Preset: Preset{
			IsSystemLevel: new(false),
		}}

		err := inst.Init(FlakeOutputType("colmenaConfigurations"), "cfg0", attributes.New(), nil, customPresets)
		require.NoError(t, err)

		require.NotNil(t, inst.Preset.IsSystemLevel)
		assert.True(t, *inst.Preset.IsSystemLevel,
			"system_level is type-level and must always come from the custom preset")
	})
}

// assertCustomPresetDefaultsMerged verifies that an installable with an empty
// preset gets every user-overridable field filled from the declared custom
// preset defaults, and that type-level fields are taken from the custom preset.
func assertCustomPresetDefaultsMerged(t *testing.T, customPresets CustomOutputTypes) {
	t.Helper()

	inst := &Installable{Preset: Preset{}}

	err := inst.Init(FlakeOutputType("colmenaConfigurations"), "cfg0", attributes.New(), nil, customPresets)
	require.NoError(t, err)

	assert.Equal(t, "config.system.build.toplevel", inst.Preset.BuildPath)
	assert.Equal(t, "/nix/var/nix/profiles/system", inst.Preset.ProfilePath)
	assert.Equal(t, "bin/switch-to-configuration", inst.Preset.ActivationPath)
	assert.Equal(t, []string{"switch", "boot"}, inst.Preset.ActivationModes)
	assert.Equal(t, "switch", inst.Preset.ActivationDefaultMode)
	require.NotNil(t, inst.Preset.SetProfile)
	assert.True(t, *inst.Preset.SetProfile)
	require.NotNil(t, inst.Preset.IsSystemLevel)
	assert.True(t, *inst.Preset.IsSystemLevel)
}
