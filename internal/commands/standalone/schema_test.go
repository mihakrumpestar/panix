package commands_standalone

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/yamlschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustGetTypeDef extracts a *yamlschema.TypeDefinition property from an object
// definition, failing the test when the key is missing or mistyped.
func mustGetTypeDef(t *testing.T, props map[string]any, name string) *yamlschema.TypeDefinition {
	t.Helper()

	raw, ok := props[name]
	require.Truef(t, ok, "missing property %q", name)

	td, ok := raw.(*yamlschema.TypeDefinition)
	require.Truef(t, ok, "property %q is not a TypeDefinition", name)

	return td
}

// resolveSchemaType follows $ref links into the schema definitions so leaf
// assertions can walk shared definitions (e.g. ThemeStyle).
func resolveSchemaType(t *testing.T, schema *yamlschema.Schema, typeDef *yamlschema.TypeDefinition) *yamlschema.TypeDefinition {
	t.Helper()

	if typeDef.Ref == "" {
		return typeDef
	}

	name, found := strings.CutPrefix(typeDef.Ref, "#/definitions/")
	require.Truef(t, found, "unsupported $ref %q", typeDef.Ref)

	raw, ok := schema.Definitions[name]
	require.Truef(t, ok, "$ref %q has no definition", typeDef.Ref)

	def, ok := raw.(*yamlschema.TypeDefinition)
	require.Truef(t, ok, "definition %q is not a TypeDefinition", name)

	return def
}

// generateThemesSchema renders the panix-themes.yml schema through the same
// schema config the schema command uses.
func generateThemesSchema(t *testing.T) *yamlschema.Schema {
	t.Helper()

	schema, err := yamlschema.NewSchema(themesSchemaConfig()).Generate()
	require.NoError(t, err)

	return schema
}

// themeObject returns the theme object definition behind the root themes map
// (root property themes, then its additionalProperties value).
func themeObject(t *testing.T, schema *yamlschema.Schema) *yamlschema.TypeDefinition {
	t.Helper()

	themes := mustGetTypeDef(t, schema.Properties, "themes")
	require.Equal(t, "object", themes.Type)
	require.NotNil(t, themes.AdditionalProperties)

	theme, ok := themes.AdditionalProperties.Value.(*yamlschema.TypeDefinition)
	require.Truef(t, ok, "theme entries are not object definitions")

	return theme
}

// The generated themes schema must describe the panix-themes.yml document
// shape: a single root "themes" mapping of theme names to partial theme
// objects, with the metadata naming the file it validates.
func TestSchemaThemesFileRootShape(t *testing.T) {
	t.Parallel()

	schema := generateThemesSchema(t)

	assert.Equal(t, "object", schema.Type)
	assert.Equal(t, []string{"themes"}, slices.Sorted(maps.Keys(schema.Properties)))
	assert.Contains(t, schema.Title, "Themes File Schema")
	assert.Contains(t, schema.Description, theme.ThemesFileName)

	themeObj := themeObject(t, schema)
	assert.Equal(t, "object", themeObj.Type)
	assert.Equal(t, yamlschema.FalseAdditionalProperties(), themeObj.AdditionalProperties)

	for _, group := range []string{"spinner", "status", "chars", "border"} {
		assert.Contains(t, themeObj.Properties, group)
	}
}

// Representative leaf keys of the theme DTO must exist at their documented
// paths (resolving shared definitions such as ThemeStyle), so the published
// schema stays usable as the themes file reference.
func TestSchemaThemesFileLeafKeys(t *testing.T) {
	t.Parallel()

	schema := generateThemesSchema(t)
	themeObj := themeObject(t, schema)

	tests := []struct {
		name     string
		groups   []string
		leaf     string
		wantType string
	}{
		{name: "spinner frames", groups: []string{"spinner"}, leaf: "frames", wantType: "array"},
		{name: "spinner interval duration", groups: []string{"spinner"}, leaf: "interval", wantType: "string"},
		{name: "border table preset", groups: []string{"border"}, leaf: "table", wantType: "string"},
		{name: "chars phase arrow", groups: []string{"chars"}, leaf: "phase_arrow", wantType: "string"},
		{name: "status ok fg color", groups: []string{"status", "ok"}, leaf: "fg", wantType: "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			group := themeObj

			for _, name := range tt.groups {
				group = resolveSchemaType(t, schema, mustGetTypeDef(t, group.Properties, name))
			}

			assert.Equal(t, tt.wantType, mustGetTypeDef(t, group.Properties, tt.leaf).Type)
		})
	}
}

// Both schema outputs must stay byte identical to the committed generated
// artifacts (gen/panix-schema.yaml and gen/panix-themes-schema.yaml): any DTO
// change without a matching task generate run is caught here.
func TestSchemaCmdOutputsMatchGeneratedArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := SchemaCmd{
		Output:       filepath.Join(dir, "panix-schema.yaml"),
		ThemesOutput: filepath.Join(dir, "panix-themes-schema.yaml"),
	}
	require.NoError(t, cmd.Run())

	tests := []struct {
		name   string
		output string
		golden string
	}{
		{name: "config schema", output: cmd.Output, golden: filepath.Join("..", "..", "..", "gen", "panix-schema.yaml")},
		{name: "themes schema", output: cmd.ThemesOutput, golden: filepath.Join("..", "..", "..", "gen", "panix-themes-schema.yaml")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected, err := os.ReadFile(tt.golden)
			require.NoErrorf(t, err, "generated artifact %s must exist (run task generate)", tt.golden)

			actual, err := os.ReadFile(tt.output)
			require.NoError(t, err)

			assert.Equal(t, string(expected), string(actual))
		})
	}
}
