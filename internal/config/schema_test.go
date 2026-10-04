package config

import (
	"reflect"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/pkg/yamlschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// definitionOfType extracts a *TypeDefinition from the schema definitions map.
func definitionOfType(t *testing.T, schema *yamlschema.Schema, name string) *yamlschema.TypeDefinition {
	t.Helper()

	raw, ok := schema.Definitions[name]
	require.Truef(t, ok, "missing definition %q", name)

	td, ok := raw.(*yamlschema.TypeDefinition)
	require.Truef(t, ok, "definition %q is not a TypeDefinition", name)

	return td
}

// The real config tree contains two distinct types named NixConfig (nix build
// configuration and the bootstrap Nix installer configuration): the generated
// schema must give each its own definition and reference the right one from
// bootstrap.nix.
func TestSchemaNixConfigDefinitionsDisambiguated(t *testing.T) {
	t.Parallel()

	generator := yamlschema.NewSchema(yamlschema.SchemaConfig{RootType: reflect.TypeFor[Config]()})

	schema, err := generator.Generate()
	require.NoError(t, err)

	bootstrapDef := definitionOfType(t, schema, "Bootstrap")
	nixProp, ok := bootstrapDef.Properties["nix"]
	require.True(t, ok, "Bootstrap should declare the nix property")

	nixRef, ok := nixProp.(*yamlschema.TypeDefinition)
	require.True(t, ok, "nix property should be a TypeDefinition")
	assert.Equal(t, "#/definitions/attributes_NixConfig", nixRef.Ref,
		"bootstrap.nix must reference the installer config definition")

	installerDef := definitionOfType(t, schema, "attributes_NixConfig")
	assert.Contains(t, installerDef.Properties, "url", "installer NixConfig must declare url")
	assert.Contains(t, installerDef.Properties, "args", "installer NixConfig must declare args")

	buildDef := definitionOfType(t, schema, "nix_NixConfig")
	assert.Contains(t, buildDef.Properties, "build_mode", "build NixConfig must declare build_mode")
	assert.NotContains(t, buildDef.Properties, "url", "build NixConfig must not mix in installer fields")
}

// The generated Preset definition must expose bootstrap_mode as an enum whose
// values are exactly the BootstrapMode constants: the schema renders the
// struct-tag oneof, so this pins the tag literal and the constants in sync.
func TestSchemaPresetBootstrapModeEnum(t *testing.T) {
	t.Parallel()

	generator := yamlschema.NewSchema(yamlschema.SchemaConfig{RootType: reflect.TypeFor[Config]()})

	schema, err := generator.Generate()
	require.NoError(t, err)

	presetDef := definitionOfType(t, schema, "Preset")
	rawProp, ok := presetDef.Properties["bootstrap_mode"]
	require.True(t, ok, "Preset must declare the bootstrap_mode property")

	prop, ok := rawProp.(*yamlschema.TypeDefinition)
	require.True(t, ok, "bootstrap_mode property must be a TypeDefinition")

	want := []string{string(installable.BootstrapNixOS), string(installable.BootstrapNixInstall)}
	assert.Equal(t, want, prop.Enum,
		"bootstrap_mode enum must be exactly the BootstrapMode constants (the oneof tag must mirror them)")
}
