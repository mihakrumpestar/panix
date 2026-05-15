package yamlschema

import (
	"reflect"
	"testing"
	"time"
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
	if !ok {
		t.Fatalf("missing property %q", key)
	}

	td, ok := raw.(*TypeDefinition)
	if !ok {
		t.Fatalf("property %q is not *TypeDefinition", key)
	}

	return td
}

func generate(t *testing.T, rootType reflect.Type) *Schema {
	t.Helper()

	gen := NewSchema(SchemaConfig{RootType: rootType})

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	return schema
}

func TestNewSchema_SimpleStruct(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[simpleRoot]())

	if schema.Type != "object" {
		t.Errorf("Type = %q, want %q", schema.Type, "object")
	}

	if schema.Schema != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("Schema = %q, want draft-07", schema.Schema)
	}

	nameDef := mustGetTypeDef(t, schema.Properties, "name")
	if nameDef.Type != "string" {
		t.Errorf("name type = %q, want %q", nameDef.Type, "string")
	}

	countDef := mustGetTypeDef(t, schema.Properties, "count")
	if countDef.Type != "integer" {
		t.Errorf("count type = %q, want %q", countDef.Type, "integer")
	}

	activeDef := mustGetTypeDef(t, schema.Properties, "active")
	if activeDef.Type != "boolean" {
		t.Errorf("active type = %q, want %q", activeDef.Type, "boolean")
	}
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
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if schema.ID != "https://example.com/simple" {
		t.Errorf("ID = %q, want %q", schema.ID, "https://example.com/simple")
	}

	if schema.Title != "Simple" {
		t.Errorf("Title = %q, want %q", schema.Title, "Simple")
	}

	if schema.Version != "1.0" {
		t.Errorf("Version = %q, want %q", schema.Version, "1.0")
	}
}

func TestNewSchema_RequiredFields(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithRequired]())

	if len(schema.Required) != 2 {
		t.Fatalf("len(Required) = %d, want 2", len(schema.Required))
	}

	requiredMap := make(map[string]bool)
	for _, field := range schema.Required {
		requiredMap[field] = true
	}

	if !requiredMap["name"] {
		t.Error("'name' should be required")
	}

	if !requiredMap["email"] {
		t.Error("'email' should be required")
	}
}

func TestNewSchema_NestedStruct(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNested]())

	configDef := mustGetTypeDef(t, schema.Properties, "config")
	if configDef.Type != "object" {
		t.Errorf("config type = %q, want %q", configDef.Type, "object")
	}

	if len(configDef.Properties) != 3 {
		t.Errorf("config properties = %d, want 3", len(configDef.Properties))
	}
}

func TestNewSchema_DuplicateStructUsesRef(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDuplicate]())

	firstDef := mustGetTypeDef(t, schema.Properties, "first")
	if firstDef.Ref != "#/definitions/sharedStruct" {
		t.Errorf("first.Ref = %q, want %q", firstDef.Ref, "#/definitions/sharedStruct")
	}

	secondDef := mustGetTypeDef(t, schema.Properties, "second")
	if secondDef.Ref != "#/definitions/sharedStruct" {
		t.Errorf("second.Ref = %q, want %q", secondDef.Ref, "#/definitions/sharedStruct")
	}

	if len(schema.Definitions) != 1 {
		t.Fatalf("len(Definitions) = %d, want 1", len(schema.Definitions))
	}

	if _, ok := schema.Definitions["sharedStruct"]; !ok {
		t.Error("missing 'sharedStruct' definition")
	}
}

func TestNewSchema_SliceField(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithSlice]())

	itemsDef := mustGetTypeDef(t, schema.Properties, "items")
	if itemsDef.Type != "array" {
		t.Errorf("items type = %q, want %q", itemsDef.Type, "array")
	}

	itemsItem, ok := itemsDef.Items.(*TypeDefinition)
	if !ok {
		t.Fatal("items.Items is not TypeDefinition")
	}

	if itemsItem.Type != "string" {
		t.Errorf("items.Items type = %q, want %q", itemsItem.Type, "string")
	}
}

func TestNewSchema_TimeDuration(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDuration]())

	timeoutDef := mustGetTypeDef(t, schema.Properties, "timeout")
	if timeoutDef.Type != "string" {
		t.Errorf("timeout type = %q, want %q", timeoutDef.Type, "string")
	}

	if timeoutDef.Description == "" {
		t.Error("timeout description should not be empty")
	}
}

func TestNewSchema_ValidateOneOf(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithValidate]())

	modeDef := mustGetTypeDef(t, schema.Properties, "mode")
	if len(modeDef.Enum) != 2 {
		t.Fatalf("mode enum = %v, want 2 values", modeDef.Enum)
	}

	if modeDef.Enum[0] != "local" || modeDef.Enum[1] != "remote" {
		t.Errorf("mode enum = %v, want [local remote]", modeDef.Enum)
	}
}

func TestNewSchema_ValidateFormatConstraints(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithValidate]())

	pathDef := mustGetTypeDef(t, schema.Properties, "path")
	if pathDef.Format != "file-path" {
		t.Errorf("path format = %q, want %q", pathDef.Format, "file-path")
	}

	absPathDef := mustGetTypeDef(t, schema.Properties, "abs_path")
	if absPathDef.Format != "uri-reference" {
		t.Errorf("abs_path format = %q, want %q", absPathDef.Format, "uri-reference")
	}

	if absPathDef.Pattern != "^/.*" {
		t.Errorf("abs_path pattern = %q, want %q", absPathDef.Pattern, "^/.*")
	}

	uriDef := mustGetTypeDef(t, schema.Properties, "uri")
	if uriDef.Format != "uri" {
		t.Errorf("uri format = %q, want %q", uriDef.Format, "uri")
	}

	urlDef := mustGetTypeDef(t, schema.Properties, "url")
	if urlDef.Format != "uri" {
		t.Errorf("url format = %q, want %q", urlDef.Format, "uri")
	}
}

func TestNewSchema_DescTag(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDesc]())

	nameDef := mustGetTypeDef(t, schema.Properties, "name")
	if nameDef.Description != "The name of the resource" {
		t.Errorf("name description = %q, want %q", nameDef.Description, "The name of the resource")
	}
}

func TestNewSchema_HelpTag(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDesc]())

	countDef := mustGetTypeDef(t, schema.Properties, "count")
	if countDef.Description != "Number of items" {
		t.Errorf("count description = %q, want %q", countDef.Description, "Number of items")
	}
}

func TestNewSchema_DefaultTag(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDesc]())

	timeoutDef := mustGetTypeDef(t, schema.Properties, "timeout")
	if timeoutDef.Default != "30s" {
		t.Errorf("timeout default = %v, want %q", timeoutDef.Default, "30s")
	}

	portDef := mustGetTypeDef(t, schema.Properties, "port")
	if portDef.Default != 8080 {
		t.Errorf("port default = %v, want 8080", portDef.Default)
	}

	if portDef.Description != "Listen port (default: 8080)" {
		t.Errorf("port description = %q, want %q", portDef.Description, "Listen port (default: 8080)")
	}

	debugDef := mustGetTypeDef(t, schema.Properties, "debug")
	if debugDef.Default != false {
		t.Errorf("debug default = %v, want false", debugDef.Default)
	}
}

func TestNewSchema_YamlDashOmitsField(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithOmit]())

	if len(schema.Properties) != 1 {
		t.Fatalf("len(Properties) = %d, want 1", len(schema.Properties))
	}

	if _, ok := schema.Properties["name"]; !ok {
		t.Error("missing 'name' property")
	}

	if _, ok := schema.Properties["value"]; ok {
		t.Error("'value' should be omitted with yaml:\"-\"")
	}
}

func TestNewSchema_PtrAndSlice(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithPtrAndSlice]())

	// innerStruct appears twice (via ptr + slice) so it becomes a $ref
	innerDef := mustGetTypeDef(t, schema.Properties, "inner")
	if innerDef.Ref != "#/definitions/innerStruct" {
		t.Errorf("inner.Ref = %q, want %q", innerDef.Ref, "#/definitions/innerStruct")
	}

	itemsDef := mustGetTypeDef(t, schema.Properties, "items")
	if itemsDef.Type != "array" {
		t.Errorf("items type = %q, want %q", itemsDef.Type, "array")
	}

	itemsItem, ok := itemsDef.Items.(*TypeDefinition)
	if !ok {
		t.Fatal("items.Items is not TypeDefinition")
	}

	if itemsItem.Ref != "#/definitions/innerStruct" {
		t.Errorf("items.Items.Ref = %q, want %q", itemsItem.Ref, "#/definitions/innerStruct")
	}
}

func TestNewSchema_MapLikeWrapper(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithMapLike]())

	entriesDef := mustGetTypeDef(t, schema.Properties, "entries")
	if entriesDef.Type != "object" {
		t.Errorf("entries type = %q, want %q", entriesDef.Type, "object")
	}

	if entriesDef.AdditionalProperties == nil {
		t.Fatal("entries should have additionalProperties")
	}
}

func TestNewSchema_NullableMapValues(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNullableMap]())

	entriesDef := mustGetTypeDef(t, schema.Properties, "entries")
	if entriesDef.AdditionalProperties == nil {
		t.Fatal("entries should have additionalProperties")
	}

	addProps, ok := entriesDef.AdditionalProperties.Value.(map[string]any)
	if !ok {
		t.Fatal("additionalProperties.Value is not a map for nullable_values")
	}

	anyOf, ok := addProps["anyOf"].([]any)
	if !ok {
		t.Fatal("missing anyOf in additionalProperties")
	}

	if len(anyOf) != 2 {
		t.Errorf("anyOf length = %d, want 2", len(anyOf))
	}
}

func TestNewSchema_InlineFields(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithInline]())

	if _, ok := schema.Properties["base_field"]; !ok {
		t.Error("inline 'base_field' should be promoted to root")
	}

	if _, ok := schema.Properties["extra_field"]; !ok {
		t.Error("missing 'extra_field' property")
	}
}

func TestNewSchema_DependencyCollection(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithDependency]())

	modeDef := mustGetTypeDef(t, schema.Properties, "mode")
	if modeDef.Type != "string" {
		t.Errorf("mode type = %q, want %q", modeDef.Type, "string")
	}

	configDef := mustGetTypeDef(t, schema.Properties, "config")
	if configDef.Type != "string" {
		t.Errorf("config type = %q, want %q", configDef.Type, "string")
	}
}

func TestNewSchema_AdditionalPropertiesTrue(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[simpleRoot]())

	if schema.AdditionalProperties != true {
		t.Errorf("root AdditionalProperties = %v, want true", schema.AdditionalProperties)
	}
}

func TestNewSchema_NestedAdditionalPropertiesFalse(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNested]())

	configDef := mustGetTypeDef(t, schema.Properties, "config")
	if configDef.AdditionalProperties == nil {
		t.Fatal("nested object should have additionalProperties")
	}

	if configDef.AdditionalProperties.Value != false {
		t.Errorf("nested AdditionalProperties = %v, want false", configDef.AdditionalProperties.Value)
	}
}

func TestNewSchema_NoDefinitionsForSingleUse(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[rootWithNested]())

	if len(schema.Definitions) != 0 {
		t.Errorf("Definitions = %d, want 0 (no reused types)", len(schema.Definitions))
	}
}

func TestNewSchema_EmptyConfig(t *testing.T) {
	t.Parallel()

	schema := generate(t, reflect.TypeFor[simpleRoot]())

	if schema.ID != "" {
		t.Errorf("ID = %q, want empty", schema.ID)
	}

	if schema.Title != "" {
		t.Errorf("Title = %q, want empty", schema.Title)
	}

	if schema.Description != "" {
		t.Errorf("Description = %q, want empty", schema.Description)
	}
}

func TestFalseAdditionalProperties(t *testing.T) {
	t.Parallel()

	wrapper := FalseAdditionalProperties()
	if wrapper.Value != false {
		t.Errorf("Value = %v, want false", wrapper.Value)
	}
}

func TestRequiredList_MarshalYAML(t *testing.T) {
	t.Parallel()

	empty := requiredList{}

	result, err := empty.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	list, ok := result.([]string)
	if !ok {
		t.Fatal("result is not []string")
	}

	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}

	nonEmpty := requiredList{"a", "b"}

	result, err = nonEmpty.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	list, ok = result.([]string)
	if !ok {
		t.Fatal("result is not []string")
	}

	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestDependencyList_MarshalYAML(t *testing.T) {
	t.Parallel()

	empty := dependencyList{}

	result, err := empty.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	list, ok := result.([]string)
	if !ok {
		t.Fatal("result is not []string")
	}

	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}

func TestDependenciesMap_MarshalYAML(t *testing.T) {
	t.Parallel()

	empty := dependenciesMap{}

	result, err := empty.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	depMap, ok := result.(map[string]dependencyList)
	if !ok {
		t.Fatal("result is not map[string]dependencyList")
	}

	if len(depMap) != 0 {
		t.Errorf("len = %d, want 0", len(depMap))
	}
}

func TestAdditionalPropertiesWrapper_MarshalYAML(t *testing.T) {
	t.Parallel()

	wrapper := &additionalPropertiesWrapper{Value: "test"}

	result, err := wrapper.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	if result != "test" {
		t.Errorf("result = %v, want %q", result, "test")
	}
}

func TestFindMapValueType_NonStruct(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[string]())
	if result != nil {
		t.Error("expected nil for non-struct type")
	}
}

func TestFindMapValueType_StructWithVisibleFields(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[simpleRoot]())
	if result != nil {
		t.Error("expected nil for struct with YAML-visible fields")
	}
}

func TestFindMapValueType_MapLikeWrapper(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[mapLikeWrapper]())
	if result == nil {
		t.Fatal("expected non-nil for map-like wrapper")
	}

	if result != reflect.TypeFor[simpleRoot]() {
		t.Errorf("value type = %v, want %v", result, reflect.TypeFor[simpleRoot]())
	}
}

func TestFindMapValueType_PtrToMapLike(t *testing.T) {
	t.Parallel()

	result := findMapValueType(reflect.TypeFor[*mapLikeWrapper]())
	if result == nil {
		t.Fatal("expected non-nil for ptr to map-like wrapper")
	}

	if result != reflect.TypeFor[simpleRoot]() {
		t.Errorf("value type = %v, want %v", result, reflect.TypeFor[simpleRoot]())
	}
}

func TestHasYAMLVisibleFields_True(t *testing.T) {
	t.Parallel()

	result := hasYAMLVisibleFields(reflect.TypeFor[simpleRoot]())
	if !result {
		t.Error("expected true for struct with exported fields")
	}
}

func TestHasYAMLVisibleFields_False(t *testing.T) {
	t.Parallel()

	result := hasYAMLVisibleFields(reflect.TypeFor[mapLikeWrapper]())
	if result {
		t.Error("expected false for struct with only unexported/yaml:- fields")
	}
}

func TestYamlFieldName_WithTag(t *testing.T) {
	t.Parallel()

	field, _ := reflect.TypeFor[simpleRoot]().FieldByName("Name")

	result := yamlFieldName(field, field.Tag.Get("yaml"))
	if result != "name" {
		t.Errorf("yamlFieldName = %q, want %q", result, "name")
	}
}

func TestYamlFieldName_WithoutTag(t *testing.T) {
	t.Parallel()

	type noTag struct {
		MyField string
	}

	field, _ := reflect.TypeFor[noTag]().FieldByName("MyField")

	result := yamlFieldName(field, "")
	if result != "myfield" {
		t.Errorf("yamlFieldName = %q, want %q", result, "myfield")
	}
}

func TestIsFieldRequired_YamlTag(t *testing.T) {
	t.Parallel()

	type reqYaml struct {
		Name string `yaml:"name,required"`
	}

	field, _ := reflect.TypeFor[reqYaml]().FieldByName("Name")
	if !isFieldRequired(field, field.Tag.Get("yaml")) {
		t.Error("expected field to be required via yaml tag")
	}
}

func TestIsFieldRequired_ValidateTag(t *testing.T) {
	t.Parallel()

	type reqValidate struct {
		Name string `yaml:"name" validate:"required"`
	}

	field, _ := reflect.TypeFor[reqValidate]().FieldByName("Name")
	if !isFieldRequired(field, field.Tag.Get("yaml")) {
		t.Error("expected field to be required via validate tag")
	}
}

func TestIsFieldRequired_NotRequired(t *testing.T) {
	t.Parallel()

	field, _ := reflect.TypeFor[simpleRoot]().FieldByName("Name")
	if isFieldRequired(field, field.Tag.Get("yaml")) {
		t.Error("expected field to not be required")
	}
}

func TestParseDefaultValue_Bool(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("true", reflect.TypeFor[bool]())
	if result != true {
		t.Errorf("result = %v, want true", result)
	}

	result = parseDefaultValue("false", reflect.TypeFor[bool]())
	if result != false {
		t.Errorf("result = %v, want false", result)
	}
}

func TestParseDefaultValue_Int(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("42", reflect.TypeFor[int]())
	if result != 42 {
		t.Errorf("result = %v, want 42", result)
	}
}

func TestParseDefaultValue_String(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("hello", reflect.TypeFor[string]())
	if result != "hello" {
		t.Errorf("result = %v, want %q", result, "hello")
	}
}

func TestParseDefaultValue_Duration(t *testing.T) {
	t.Parallel()

	result := parseDefaultValue("30s", reflect.TypeFor[time.Duration]())
	if result != "30s" {
		t.Errorf("result = %v, want %q", result, "30s")
	}
}

func TestResolveYAMLFieldName_WithTag(t *testing.T) {
	t.Parallel()

	result := resolveYAMLFieldName(reflect.TypeFor[simpleRoot](), "Name")
	if result != "name" {
		t.Errorf("result = %q, want %q", result, "name")
	}
}

func TestResolveYAMLFieldName_WithoutTag(t *testing.T) {
	t.Parallel()

	type noTag struct {
		MyField string
	}

	result := resolveYAMLFieldName(reflect.TypeFor[noTag](), "MyField")
	if result != "myfield" {
		t.Errorf("result = %q, want %q", result, "myfield")
	}
}

func TestNewSchema_ProcessStruct_NonStructError(t *testing.T) {
	t.Parallel()

	gen := NewSchema(SchemaConfig{
		RootType: reflect.TypeFor[string](),
	})

	_, err := gen.Generate()
	if err == nil {
		t.Fatal("Generate() should error for non-struct root type")
	}
}
