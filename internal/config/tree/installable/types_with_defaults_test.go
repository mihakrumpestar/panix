package installable

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type resolveFlakeInstallableTest struct {
	name       string
	outputType FlakeOutputType
	attrName   AttributeName
	preset     Preset
	want       string
}

var resolveFlakeInstallableTests = []resolveFlakeInstallableTest{
	{
		name:       "nixosConfigurations with build path",
		outputType: FlakeOutputType("nixosConfigurations"),
		attrName:   AttributeName("server1"),
		preset: Preset{
			BuildPath: "config.system.build.toplevel",
		},
		want: "nixosConfigurations.server1.config.system.build.toplevel",
	},
	{
		name:       "homeConfigurations with build path",
		outputType: FlakeOutputType("homeConfigurations"),
		attrName:   AttributeName("alice"),
		preset: Preset{
			BuildPath: "activationPackage",
		},
		want: "homeConfigurations.alice.activationPackage",
	},
	{
		name:       "systemConfigs with empty build path",
		outputType: FlakeOutputType("systemConfigs"),
		attrName:   AttributeName("myhost"),
		preset:     Preset{},
		want:       "systemConfigs.myhost",
	},
	{
		name:       "packages type omits type prefix - bare name",
		outputType: FlakeOutputType("packages"),
		attrName:   AttributeName("nixvim"),
		preset: Preset{
			OmitTypeFromAttrPath: true,
		},
		want: "nixvim",
	},
	{
		name:       "packages type with build path still omits type prefix",
		outputType: FlakeOutputType("packages"),
		attrName:   AttributeName("my-tool"),
		preset: Preset{
			OmitTypeFromAttrPath: true,
			BuildPath:            "some.subpath",
		},
		want: "my-tool.some.subpath",
	},
	{
		name:       "packages preset from defaults map produces bare name",
		outputType: FlakeOutputType("packages"),
		attrName:   AttributeName("default"),
		preset:     presets[FlakeOutputType("packages")],
		want:       "default",
	},
	{
		name:       "nixosConfigurations preset from defaults map",
		outputType: FlakeOutputType("nixosConfigurations"),
		attrName:   AttributeName("workstation"),
		preset:     presets[FlakeOutputType("nixosConfigurations")],
		want:       "nixosConfigurations.workstation.config.system.build.toplevel",
	},
	// darwinConfigurations — exercises the "system" build path suffix.
	{
		name:       "darwinConfigurations with build path",
		outputType: FlakeOutputType("darwinConfigurations"),
		attrName:   AttributeName("mymac"),
		preset: Preset{
			BuildPath: "system",
		},
		want: "darwinConfigurations.mymac.system",
	},
	// nixOnDroidConfigurations — exercises the dotted "build.activationPackage"
	// build path, which has a period and must be appended verbatim.
	{
		name:       "nixOnDroidConfigurations with dotted build path",
		outputType: FlakeOutputType("nixOnDroidConfigurations"),
		attrName:   AttributeName("myphone"),
		preset: Preset{
			BuildPath: "build.activationPackage",
		},
		want: "nixOnDroidConfigurations.myphone.build.activationPackage",
	},
	// Preset-from-defaults-map cases for the remaining types, mirroring the
	// existing nixosConfigurations/packages rows above. These ensure that
	// the values in the presets map actually resolve to the expected
	// attrpaths, not just the field-level assertions in presets_test.go.
	{
		name:       "darwinConfigurations preset from defaults map",
		outputType: FlakeOutputType("darwinConfigurations"),
		attrName:   AttributeName("mymac"),
		preset:     presets[FlakeOutputType("darwinConfigurations")],
		want:       "darwinConfigurations.mymac.system",
	},
	{
		name:       "homeConfigurations preset from defaults map",
		outputType: FlakeOutputType("homeConfigurations"),
		attrName:   AttributeName("alice"),
		preset:     presets[FlakeOutputType("homeConfigurations")],
		want:       "homeConfigurations.alice.activationPackage",
	},
	{
		name:       "nixOnDroidConfigurations preset from defaults map",
		outputType: FlakeOutputType("nixOnDroidConfigurations"),
		attrName:   AttributeName("myphone"),
		preset:     presets[FlakeOutputType("nixOnDroidConfigurations")],
		want:       "nixOnDroidConfigurations.myphone.build.activationPackage",
	},
}

func TestResolveFlakeInstallable(t *testing.T) {
	t.Parallel()

	for _, tt := range resolveFlakeInstallableTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolveFlakeInstallable(tt.outputType, tt.attrName, tt.preset)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPackagesPresetOmitsType verifies that the packages preset is configured
// to omit the type prefix, which is the fix for the bug where nix searched for
// "packages.<system>.packages.<name>" (doubled "packages").
func TestPackagesPresetOmitsType(t *testing.T) {
	t.Parallel()

	preset, ok := presets[FlakeOutputType("packages")]
	assert.True(t, ok, "packages preset should exist")
	assert.True(t, preset.OmitTypeFromAttrPath,
		"packages preset must set OmitTypeFromAttrPath to avoid doubled 'packages' in attrpath")
	assert.Empty(t, preset.BuildPath, "packages preset should have empty build path")
}

// TestCompositeKey verifies that CompositeKey produces the "type/name" format
// for all 6 known output types. The composite key is used as the flat map key
// in the AtomicOrderedMap so that installables with the same attribute name
// but different types (e.g. nixosConfigurations/server1 vs
// homeConfigurations/server1) don't collide.
func TestCompositeKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		outputType FlakeOutputType
		attrName   AttributeName
		want       string
	}{
		{"nixosConfigurations", FlakeOutputType("nixosConfigurations"), AttributeName("server1"), "nixosConfigurations/server1"},
		{"darwinConfigurations", FlakeOutputType("darwinConfigurations"), AttributeName("mymac"), "darwinConfigurations/mymac"},
		{"systemConfigs", FlakeOutputType("systemConfigs"), AttributeName("myhost"), "systemConfigs/myhost"},
		{"homeConfigurations", FlakeOutputType("homeConfigurations"), AttributeName("alice"), "homeConfigurations/alice"},
		{"nixOnDroidConfigurations", FlakeOutputType("nixOnDroidConfigurations"), AttributeName("myphone"), "nixOnDroidConfigurations/myphone"},
		{"packages", FlakeOutputType("packages"), AttributeName("nixvim"), "packages/nixvim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CompositeKey(tt.outputType, tt.attrName)
			assert.Equal(t, tt.want, got)
		})
	}
}
