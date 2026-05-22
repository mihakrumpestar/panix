package yamlschema

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type simpleRoot struct {
	Name   string `yaml:"name"`
	Count  int    `yaml:"count"`
	Active bool   `yaml:"active"`
}

type rootWithRequired struct {
	Name  string `yaml:"name,required"`
	Email string `yaml:"email" validate:"required"`
}

type rootWithNested struct {
	Config simpleRoot `yaml:"config"`
	Label  string     `yaml:"label"`
}

type sharedStruct struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

type rootWithDuplicate struct {
	First  sharedStruct `yaml:"first"`
	Second sharedStruct `yaml:"second"`
}

type rootWithSlice struct {
	Items []string `yaml:"items"`
}

type rootWithDuration struct {
	Timeout time.Duration `yaml:"timeout"`
}

type rootWithValidate struct {
	Mode    string `yaml:"mode" validate:"oneof=local remote"`
	Level   int    `yaml:"level" validate:"min=1,max=10"`
	Path    string `yaml:"path" validate:"filepath"`
	AbsPath string `yaml:"abs_path" validate:"abspath"`
	URI     string `yaml:"uri" validate:"uri"`
	URL     string `yaml:"url" validate:"url"`
}

type rootWithDesc struct {
	Name    string `yaml:"name" desc:"The name of the resource"`
	Count   int    `yaml:"count" help:"Number of items"`
	Timeout string `yaml:"timeout" desc:"Max wait time" default:"30s"`
	Port    int    `yaml:"port" desc:"Listen port" default:"8080"`
	Debug   bool   `yaml:"debug" desc:"Enable debug mode" default:"false"`
}

type rootWithOmit struct {
	Name  string `yaml:"name"`
	Value string `yaml:"-"`
}

type innerStruct struct {
	Key string `yaml:"key"`
}

type rootWithPtrAndSlice struct {
	Inner *innerStruct  `yaml:"inner"`
	Items []innerStruct `yaml:"items"`
}

type mapLikeWrapper struct {
	entries map[string]simpleRoot `validate:"dive"` //nolint:unused
}

type rootWithMapLike struct {
	Entries mapLikeWrapper `yaml:"entries"`
}

type rootWithNullableMap struct {
	Entries mapLikeWrapper `yaml:"entries" schema:"nullable_values"`
}

type inlineBase struct {
	BaseField string `yaml:"base_field"`
}

type rootWithInline struct {
	InlineBase inlineBase `yaml:",inline"`
	ExtraField string     `yaml:"extra_field"`
}

type rootWithDependency struct {
	Mode   string `yaml:"mode" validate:"required_with=Config"`
	Config string `yaml:"config"`
}

// mustGetTypeDef extracts a *TypeDefinition from a property map.
// Fatals if the key is missing or the value is not *TypeDefinition.
func mustGetTypeDef(t *testing.T, props map[string]any, key string) *TypeDefinition {
	t.Helper()

	raw, ok := props[key]
	require.Truef(t, ok, "missing property %q", key)

	td, ok := raw.(*TypeDefinition)
	require.Truef(t, ok, "property %q is not *TypeDefinition", key)

	return td
}

func generate(t *testing.T, rootType reflect.Type) *Schema {
	t.Helper()

	gen := NewSchema(SchemaConfig{RootType: rootType})

	schema, err := gen.Generate()
	require.NoError(t, err)

	return schema
}

func TestNewSchema_SimpleStruct(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[simpleRoot]())

	assert.Equal(t, "object", schema.Type, "Type mismatch")
	assert.Equal(t, "http://json-schema.org/draft-07/schema#", schema.Schema, "Schema mismatch")

	nameDef := mustGetTypeDef(t, schema.Properties, "name")
	assert.Equal(t, "string", nameDef.Type, "name type mismatch")

	countDef := mustGetTypeDef(t, schema.Properties, "count")
	assert.Equal(t, "integer", countDef.Type, "count type mismatch")

	activeDef := mustGetTypeDef(t, schema.Properties, "active")
	assert.Equal(t, "boolean", activeDef.Type, "active type mismatch")
}

func TestNewSchema_MetaData(t *testing.T) {
	t.Parallel()

	gen := NewSchema(SchemaConfig{
		RootType: reflect.TypeFor[simpleRoot](),
		SchemaID: "https://example.com/simple",
		Title:    "Simple",
		Version:  "1.0",
	})

	schema, err := gen.Generate()
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/simple", schema.ID, "ID mismatch")
	assert.Equal(t, "Simple", schema.Title, "Title mismatch")
	assert.Equal(t, "1.0", schema.Version, "Version mismatch")
}

func TestNewSchema_RequiredFields(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithRequired]())

	require.Len(t, schema.Required, 2)

	requiredMap := make(map[string]bool)
	for _, field := range schema.Required {
		requiredMap[field] = true
	}

	assert.True(t, requiredMap["name"], "'name' should be required")
	assert.True(t, requiredMap["email"], "'email' should be required")
}

func TestNewSchema_NestedStruct(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNested]())

	configDef := mustGetTypeDef(t, schema.Properties, "config")
	assert.Equal(t, "object", configDef.Type, "config type mismatch")
	assert.Len(t, configDef.Properties, 3)
}

func TestNewSchema_DuplicateStructUsesRef(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDuplicate]())

	firstDef := mustGetTypeDef(t, schema.Properties, "first")
	assert.Equal(t, "#/definitions/sharedStruct", firstDef.Ref, "first.Ref mismatch")

	secondDef := mustGetTypeDef(t, schema.Properties, "second")
	assert.Equal(t, "#/definitions/sharedStruct", secondDef.Ref, "second.Ref mismatch")

	require.Len(t, schema.Definitions, 1)
	assert.Contains(t, schema.Definitions, "sharedStruct", "missing 'sharedStruct' definition")
}

func TestNewSchema_SliceField(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithSlice]())

	itemsDef := mustGetTypeDef(t, schema.Properties, "items")
	assert.Equal(t, "array", itemsDef.Type, "items type mismatch")

	itemsItem, ok := itemsDef.Items.(*TypeDefinition)
	require.True(t, ok, "items.Items is not TypeDefinition")

	assert.Equal(t, "string", itemsItem.Type, "items.Items type mismatch")
}

func TestNewSchema_TimeDuration(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDuration]())

	timeoutDef := mustGetTypeDef(t, schema.Properties, "timeout")
	assert.Equal(t, "string", timeoutDef.Type, "timeout type mismatch")
	assert.NotEmpty(t, timeoutDef.Description, "timeout description should not be empty")
}

func TestNewSchema_ValidateOneOf(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithValidate]())

	modeDef := mustGetTypeDef(t, schema.Properties, "mode")
	require.Len(t, modeDef.Enum, 2)
	assert.Equal(t, "local", modeDef.Enum[0], "mode enum[0] mismatch")
	assert.Equal(t, "remote", modeDef.Enum[1], "mode enum[1] mismatch")
}

func TestNewSchema_ValidateFormatConstraints(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithValidate]())

	pathDef := mustGetTypeDef(t, schema.Properties, "path")
	assert.Equal(t, "file-path", pathDef.Format, "path format mismatch")

	absPathDef := mustGetTypeDef(t, schema.Properties, "abs_path")
	assert.Equal(t, "uri-reference", absPathDef.Format, "abs_path format mismatch")
	assert.Equal(t, "^/.*", absPathDef.Pattern, "abs_path pattern mismatch")

	uriDef := mustGetTypeDef(t, schema.Properties, "uri")
	assert.Equal(t, "uri", uriDef.Format, "uri format mismatch")

	urlDef := mustGetTypeDef(t, schema.Properties, "url")
	assert.Equal(t, "uri", urlDef.Format, "url format mismatch")
}

func TestNewSchema_DescTag(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDesc]())

	nameDef := mustGetTypeDef(t, schema.Properties, "name")
	assert.Equal(t, "The name of the resource", nameDef.Description, "name description mismatch")
}

func TestNewSchema_HelpTag(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDesc]())

	countDef := mustGetTypeDef(t, schema.Properties, "count")
	assert.Equal(t, "Number of items", countDef.Description, "count description mismatch")
}

func TestNewSchema_DefaultTag(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDesc]())

	timeoutDef := mustGetTypeDef(t, schema.Properties, "timeout")
	assert.Equal(t, "30s", timeoutDef.Default, "timeout default mismatch")

	portDef := mustGetTypeDef(t, schema.Properties, "port")
	assert.Equal(t, 8080, portDef.Default, "port default mismatch")
	assert.Equal(t, "Listen port (default: 8080)", portDef.Description, "port description mismatch")

	debugDef := mustGetTypeDef(t, schema.Properties, "debug")
	assert.Equal(t, false, debugDef.Default, "debug default mismatch")
}

func TestNewSchema_YamlDashOmitsField(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithOmit]())

	require.Len(t, schema.Properties, 1)
	assert.Contains(t, schema.Properties, "name", "missing 'name' property")
	assert.NotContains(t, schema.Properties, "value", "'value' should be omitted with yaml:\"-\"")
}

func TestNewSchema_PtrAndSlice(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithPtrAndSlice]())

	// innerStruct appears twice (via ptr + slice) so it becomes a $ref
	innerDef := mustGetTypeDef(t, schema.Properties, "inner")
	assert.Equal(t, "#/definitions/innerStruct", innerDef.Ref, "inner.Ref mismatch")

	itemsDef := mustGetTypeDef(t, schema.Properties, "items")
	assert.Equal(t, "array", itemsDef.Type, "items type mismatch")

	itemsItem, ok := itemsDef.Items.(*TypeDefinition)
	require.True(t, ok, "items.Items is not TypeDefinition")

	assert.Equal(t, "#/definitions/innerStruct", itemsItem.Ref, "items.Items.Ref mismatch")
}

func TestNewSchema_MapLikeWrapper(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithMapLike]())

	entriesDef := mustGetTypeDef(t, schema.Properties, "entries")
	assert.Equal(t, "object", entriesDef.Type, "entries type mismatch")
	require.NotNil(t, entriesDef.AdditionalProperties, "entries should have additionalProperties")
}

func TestNewSchema_NullableMapValues(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNullableMap]())

	entriesDef := mustGetTypeDef(t, schema.Properties, "entries")
	require.NotNil(t, entriesDef.AdditionalProperties, "entries should have additionalProperties")

	addProps, ok := entriesDef.AdditionalProperties.Value.(map[string]any)
	require.True(t, ok, "additionalProperties.Value is not a map for nullable_values")

	anyOf, ok := addProps["anyOf"].([]any)
	require.True(t, ok, "missing anyOf in additionalProperties")

	assert.Len(t, anyOf, 2)
}

func TestNewSchema_InlineFields(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithInline]())

	assert.Contains(t, schema.Properties, "base_field", "inline 'base_field' should be promoted to root")
	assert.Contains(t, schema.Properties, "extra_field", "missing 'extra_field' property")
}

func TestNewSchema_DependencyCollection(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDependency]())

	modeDef := mustGetTypeDef(t, schema.Properties, "mode")
	assert.Equal(t, "string", modeDef.Type, "mode type mismatch")

	configDef := mustGetTypeDef(t, schema.Properties, "config")
	assert.Equal(t, "string", configDef.Type, "config type mismatch")
}

func TestNewSchema_AdditionalPropertiesTrue(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[simpleRoot]())

	assert.Equal(t, true, schema.AdditionalProperties, "root AdditionalProperties mismatch")
}

func TestNewSchema_NestedAdditionalPropertiesFalse(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNested]())

	configDef := mustGetTypeDef(t, schema.Properties, "config")
	require.NotNil(t, configDef.AdditionalProperties, "nested object should have additionalProperties")
	assert.Equal(t, false, configDef.AdditionalProperties.Value, "nested AdditionalProperties mismatch")
}

func TestNewSchema_NoDefinitionsForSingleUse(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNested]())

	assert.Empty(t, schema.Definitions, "no reused types expected")
}

func TestNewSchema_EmptyConfig(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[simpleRoot]())

	assert.Empty(t, schema.ID, "ID should be empty")
	assert.Empty(t, schema.Title, "Title should be empty")
	assert.Empty(t, schema.Description, "Description should be empty")
}

func TestFalseAdditionalProperties(t *testing.T) {
	t.Parallel()

	wrapper := FalseAdditionalProperties()
	assert.Equal(t, false, wrapper.Value, "Value mismatch")
}

func TestRequiredList_MarshalYAML(t *testing.T) {
	t.Parallel()

	empty := requiredList{}

	result, err := empty.MarshalYAML()
	require.NoError(t, err)

	list, ok := result.([]string)
	require.True(t, ok, "result is not []string")

	assert.Empty(t, list)

	nonEmpty := requiredList{"a", "b"}

	result, err = nonEmpty.MarshalYAML()
	require.NoError(t, err)

	list, ok = result.([]string)
	require.True(t, ok, "result is not []string")

	assert.Len(t, list, 2)
}

func TestDependencyList_MarshalYAML(t *testing.T) {
	t.Parallel()

	empty := dependencyList{}

	result, err := empty.MarshalYAML()
	require.NoError(t, err)

	list, ok := result.([]string)
	require.True(t, ok, "result is not []string")

	assert.Empty(t, list)
}

func TestDependenciesMap_MarshalYAML(t *testing.T) {
	t.Parallel()

	empty := dependenciesMap{}

	result, err := empty.MarshalYAML()
	require.NoError(t, err)

	depMap, ok := result.(map[string]dependencyList)
	require.True(t, ok, "result is not map[string]dependencyList")

	assert.Empty(t, depMap)
}

func TestAdditionalPropertiesWrapper_MarshalYAML(t *testing.T) {
	t.Parallel()

	wrapper := &additionalPropertiesWrapper{Value: "test"}

	result, err := wrapper.MarshalYAML()
	require.NoError(t, err)

	assert.Equal(t, "test", result, "result mismatch")
}

func TestFindMapValueType_NonStruct(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[string]())
	assert.Nil(t, result, "expected nil for non-struct type")
}

func TestFindMapValueType_StructWithVisibleFields(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[simpleRoot]())
	assert.Nil(t, result, "expected nil for struct with YAML-visible fields")
}

func TestFindMapValueType_MapLikeWrapper(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[mapLikeWrapper]())
	require.NotNil(t, result, "expected non-nil for map-like wrapper")

	assert.Equal(t, reflect.TypeFor[simpleRoot](), result, "value type mismatch")
}

func TestFindMapValueType_PtrToMapLike(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[*mapLikeWrapper]())
	require.NotNil(t, result, "expected non-nil for ptr to map-like wrapper")

	assert.Equal(t, reflect.TypeFor[simpleRoot](), result, "value type mismatch")
}

func TestHasYAMLVisibleFields_True(t *testing.T) {
	t.Parallel()

	result := hasYAMLVisibleFields(reflect.TypeFor[simpleRoot]())
	assert.True(t, result, "expected true for struct with exported fields")
}

func TestHasYAMLVisibleFields_False(t *testing.T) {
	t.Parallel()

	result := hasYAMLVisibleFields(reflect.TypeFor[mapLikeWrapper]())
	assert.False(t, result, "expected false for struct with only unexported/yaml:- fields")
}

func TestYamlFieldName_WithTag(t *testing.T) {
	t.Parallel()

	field, _ := reflect.TypeFor[simpleRoot]().FieldByName("Name")

	result := yamlFieldName(field, field.Tag.Get("yaml"))
	assert.Equal(t, "name", result, "yamlFieldName mismatch")
}

func TestYamlFieldName_WithoutTag(t *testing.T) {
	t.Parallel()

	type noTag struct {
		MyField string
	}

	field, _ := reflect.TypeFor[noTag]().FieldByName("MyField")

	result := yamlFieldName(field, "")
	assert.Equal(t, "myfield", result, "yamlFieldName mismatch")
}

func TestIsFieldRequired_YamlTag(t *testing.T) {
	t.Parallel()

	type reqYaml struct {
		Name string `yaml:"name,required"`
	}

	field, _ := reflect.TypeFor[reqYaml]().FieldByName("Name")
	assert.True(t, isFieldRequired(field, field.Tag.Get("yaml")), "expected field to be required via yaml tag")
}

func TestIsFieldRequired_ValidateTag(t *testing.T) {
	t.Parallel()

	type reqValidate struct {
		Name string `yaml:"name" validate:"required"`
	}

	field, _ := reflect.TypeFor[reqValidate]().FieldByName("Name")
	assert.True(t, isFieldRequired(field, field.Tag.Get("yaml")), "expected field to be required via validate tag")
}

func TestIsFieldRequired_NotRequired(t *testing.T) {
	t.Parallel()

	field, _ := reflect.TypeFor[simpleRoot]().FieldByName("Name")
	assert.False(t, isFieldRequired(field, field.Tag.Get("yaml")), "expected field to not be required")
}

func TestParseDefaultValue_Bool(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("true", reflect.TypeFor[bool]())
	assert.Equal(t, true, result)

	result = parseDefaultValue("false", reflect.TypeFor[bool]())
	assert.Equal(t, false, result)
}

func TestParseDefaultValue_Int(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("42", reflect.TypeFor[int]())
	assert.Equal(t, 42, result)
}

func TestParseDefaultValue_String(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("hello", reflect.TypeFor[string]())
	assert.Equal(t, "hello", result)
}

func TestParseDefaultValue_Duration(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("30s", reflect.TypeFor[time.Duration]())
	assert.Equal(t, "30s", result)
}

func TestResolveYAMLFieldName_WithTag(t *testing.T) {
	t.Parallel()

	result := resolveYAMLFieldName(reflect.TypeFor[simpleRoot](), "Name")
	assert.Equal(t, "name", result)
}

func TestResolveYAMLFieldName_WithoutTag(t *testing.T) {
	t.Parallel()

	type noTag struct {
		MyField string
	}

	result := resolveYAMLFieldName(reflect.TypeFor[noTag](), "MyField")
	assert.Equal(t, "myfield", result)
}

func TestNewSchema_ProcessStruct_NonStructError(t *testing.T) {
	t.Parallel()

	gen := NewSchema(SchemaConfig{
		RootType: reflect.TypeFor[string](),
	})

	_, err := gen.Generate()
	require.Error(t, err)
}
