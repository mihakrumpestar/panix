package schema

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	config_attributes "github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/pkg/errors"
)

var (
	ErrExpectedStructType  = errors.New("expected struct type")
	ErrUnsupportedTypeKind = errors.New("unsupported type kind")
	ErrNilValue            = errors.New("nil value")
)

// Generator holds the state for schema generation.
type Generator struct {
	visited     map[reflect.Type]bool
	definitions map[string]any
	defTypes    map[reflect.Type]string
}

// RequiredList is a list of required fields that always serializes properly,
// even when empty (unlike []string with omitempty).
type RequiredList []string

func (r RequiredList) MarshalYAML() (any, error) {
	if len(r) == 0 {
		return []string{}, nil
	}

	return []string(r), nil
}

// AdditionalPropertiesWrapper handles YAML marshaling of additionalProperties
// which can be false, a schema object, or nil.
type AdditionalPropertiesWrapper struct {
	Value any
}

// MarshalYAML implements yaml.Marshaler for AdditionalPropertiesWrapper.
func (ap *AdditionalPropertiesWrapper) MarshalYAML() (any, error) {
	if ap == nil {
		return nil, ErrNilValue
	}

	return ap.Value, nil
}

// FalseAdditionalProperties returns AdditionalProperties set to false.
func FalseAdditionalProperties() *AdditionalPropertiesWrapper {
	return &AdditionalPropertiesWrapper{Value: false}
}

// TypeDefinition represents a schema type definition.
type TypeDefinition struct {
	Type                 string                       `yaml:"type,omitempty"`
	Description          string                       `yaml:"description,omitempty"`
	Default              any                          `yaml:"default,omitempty"`
	Properties           map[string]any               `yaml:"properties,omitempty"`
	Items                any                          `yaml:"items,omitempty"`
	Required             RequiredList                 `yaml:"required,omitempty"`
	Enum                 []string                     `yaml:"enum,omitempty"`
	Pattern              string                       `yaml:"pattern,omitempty"`
	Example              any                          `yaml:"example,omitempty"`
	AdditionalProperties *AdditionalPropertiesWrapper `yaml:"additionalProperties,omitempty"`
	AnyOf                []any                        `yaml:"anyOf,omitempty"`
	AllOf                []any                        `yaml:"allOf,omitempty"`
	Ref                  string                       `yaml:"$ref,omitempty"`
	Format               string                       `yaml:"format,omitempty"`
	Minimum              *int                         `yaml:"minimum,omitempty"`
	Maximum              *int                         `yaml:"maximum,omitempty"`
	MinLength            *int                         `yaml:"minLength,omitempty"`
	MaxLength            *int                         `yaml:"maxLength,omitempty"`
}

// Schema represents the root YAML schema document.
type Schema struct {
	Schema               string         `yaml:"$schema"`
	ID                   string         `yaml:"$id,omitempty"`
	Version              string         `yaml:"version,omitempty"`
	Title                string         `yaml:"title,omitempty"`
	Description          string         `yaml:"description,omitempty"`
	Type                 string         `yaml:"type,omitempty"`
	Properties           map[string]any `yaml:"properties,omitempty"`
	Required             RequiredList   `yaml:"required,omitempty"`
	AdditionalProperties any            `yaml:"additionalProperties,omitempty"`
	Definitions          map[string]any `yaml:"definitions,omitempty"`
	AllOf                []any          `yaml:"allOf,omitempty"`
}

// NewGenerator creates a new schema generator.
func NewGenerator() *Generator {
	generator := &Generator{
		visited:     make(map[reflect.Type]bool),
		definitions: make(map[string]any),
		defTypes:    make(map[reflect.Type]string),
	}

	generator.initDefinitionTypes()

	return generator
}

// Generate creates a YAML schema from the config.Config type.
func (g *Generator) Generate() (*Schema, error) {
	schema := &Schema{
		Schema:               "http://json-schema.org/draft-07/schema#",
		ID:                   "https://panix.dev/schema.json",
		Version:              gen.Version(),
		Title:                "Panix Configuration Schema",
		Description:          "Schema for Panix NixOS deployment configuration files",
		Type:                 "object",
		Properties:           make(map[string]any),
		Required:             RequiredList{},
		AdditionalProperties: true, // Required to allow anchors
	}

	// Generate schema from config.Config
	cfgType := reflect.TypeFor[config.Config]()

	properties, required, err := g.processStruct(cfgType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to process Config struct")
	}

	schema.Properties = properties
	schema.Required = required

	// Add collected definitions
	if len(g.definitions) > 0 {
		schema.Definitions = g.definitions
	}

	return schema, nil
}

// initDefinitionTypes registers types that should be defined once and referenced via $ref.
func (g *Generator) initDefinitionTypes() {
	// Register types that appear multiple times in the schema
	g.defTypes[reflect.TypeFor[config_attributes.Bootstrap]()] = "Bootstrap"
	g.defTypes[reflect.TypeFor[config_attributes.NixConfig]()] = "NixConfig"
	g.defTypes[reflect.TypeFor[config_attributes.PlainFileOrDirToTransfer]()] = "FileTransfer"
	g.defTypes[reflect.TypeFor[config_attributes.KexecConfig]()] = "KexecConfig"
	g.defTypes[reflect.TypeFor[ssh.SSHClient]()] = "SSH"
}

// processStruct processes a struct type and returns its properties and required fields.
func (g *Generator) hasExactValidateTag(validateTag, tag string) bool {
	tags := strings.SplitSeq(validateTag, ",")
	for t := range tags {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}

	return false
}

func (g *Generator) shouldSkipField(field reflect.StructField) bool {
	if !field.IsExported() {
		return true
	}

	if strings.Contains(field.Tag.Get("desc"), "CLI-only") {
		return true
	}

	if strings.Contains(field.Tag.Get("flag"), " ") && field.Name == "Config" {
		return true
	}

	if field.Tag.Get("yaml") == "-" {
		return true
	}

	return false
}

func (g *Generator) processInlineField(field reflect.StructField, properties map[string]any, required *[]string) error {
	inlineProps, inlineRequired, err := g.processStruct(field.Type)
	if err != nil {
		return errors.Wrapf(err, "failed to process inline field %s", field.Name)
	}

	maps.Copy(properties, inlineProps)

	*required = append(*required, inlineRequired...)

	return nil
}

func (g *Generator) isFieldRequired(field reflect.StructField, yamlTag string) bool {
	validateTag := field.Tag.Get("validate")
	yamlHasRequired := strings.Contains(yamlTag, ",required")
	validateHasRequired := g.hasExactValidateTag(validateTag, "required")

	return yamlHasRequired || validateHasRequired
}

func (g *Generator) processStruct(structType reflect.Type) (map[string]any, []string, error) {
	properties := make(map[string]any)
	required := []string{}

	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	if structType.Kind() != reflect.Struct {
		return nil, nil, errors.Wrapf(ErrExpectedStructType, "got %v", structType.Kind())
	}

	for field := range structType.Fields() {
		if g.shouldSkipField(field) {
			continue
		}

		yamlTag := field.Tag.Get("yaml")

		if strings.Contains(yamlTag, ",inline") {
			err := g.processInlineField(field, properties, &required)
			if err != nil {
				return nil, nil, err
			}

			continue
		}

		fieldName := g.getYAMLFieldName(field, yamlTag)

		prop, err := g.processType(field.Type, field)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to process field %s", field.Name)
		}

		g.setFieldDescription(prop, field)
		properties[fieldName] = prop

		if g.isFieldRequired(field, yamlTag) {
			required = append(required, fieldName)
		}
	}

	return properties, required, nil
}

func (g *Generator) setFieldDescription(prop any, field reflect.StructField) {
	desc := field.Tag.Get("desc")
	if desc == "" {
		desc = field.Tag.Get("help")
	}

	defaultVal := field.Tag.Get("default")

	if typeDef, ok := prop.(*TypeDefinition); ok {
		if defaultVal != "" {
			typeDef.Default = g.parseDefaultValue(defaultVal, field.Type)

			if desc != "" {
				desc = desc + " (default: " + defaultVal + ")"
			}
		}

		if desc != "" {
			typeDef.Description = desc
		}
	}
}

func (g *Generator) parseDefaultValue(val string, typ reflect.Type) any {
	// Handle time.Duration as string
	if typ.String() == "time.Duration" {
		return val
	}

	kind := typ.Kind()
	switch kind {
	case reflect.Bool:
		return val == "true"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		intVal, err := parseInt(val)
		if err == nil {
			return intVal
		}

		return val
	default:
		return val
	}
}

// processType processes a type and returns its schema definition.
func (g *Generator) processType(typ reflect.Type, field reflect.StructField) (any, error) {
	// Handle pointers
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Check if this type should use a $ref definition
	if defName, ok := g.defTypes[typ]; ok {
		return g.processDefinitionType(typ, defName, field)
	}

	// Check for special types first
	if def := g.getSpecialTypeDefinition(typ); def != nil {
		return def, nil
	}

	validateTag := field.Tag.Get("validate")

	return g.processTypeByKind(typ.Kind(), typ, field, validateTag)
}

// processDefinitionType handles types that should be defined once and referenced.
func (g *Generator) processDefinitionType(typ reflect.Type, defName string, _ reflect.StructField) (any, error) {
	// If definition already exists, return a $ref
	if _, exists := g.definitions[defName]; exists {
		return &TypeDefinition{Ref: "#/definitions/" + defName}, nil
	}

	// Generate the definition
	props, required, err := g.processStruct(typ)
	if err != nil {
		return nil, err
	}

	def := &TypeDefinition{
		Type:                 "object",
		Properties:           props,
		Required:             RequiredList(required),
		AdditionalProperties: FalseAdditionalProperties(),
	}

	// Store in definitions
	g.definitions[defName] = def

	// Return a $ref
	return &TypeDefinition{Ref: "#/definitions/" + defName}, nil
}

// processTypeByKind dispatches type processing based on the kind of type.
func (g *Generator) processTypeByKind(kind reflect.Kind, typ reflect.Type, field reflect.StructField, validateTag string) (any, error) {
	switch kind {
	case reflect.Bool:
		return &TypeDefinition{Type: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return g.processNumericType("integer", validateTag), nil
	case reflect.Float32, reflect.Float64:
		return g.processNumericType("number", validateTag), nil
	case reflect.String:
		return g.processNumericType("string", validateTag), nil
	case reflect.Slice, reflect.Array:
		return g.processSliceType(typ, field, validateTag)
	case reflect.Map:
		// Handle regular maps
		return g.processMapType(typ, field)
	case reflect.Struct:
		return g.processStructOrOrderedMap(typ, field)
	case reflect.Interface:
		// Interfaces can be any type
		return &TypeDefinition{}, nil
	default:
		return nil, errors.Wrapf(ErrUnsupportedTypeKind, "%v", kind)
	}
}

// processStructOrOrderedMap handles struct types, checking for OrderedMap first.
func (g *Generator) processStructOrOrderedMap(typ reflect.Type, field reflect.StructField) (any, error) {
	// Check if this is an OrderedMap
	if g.isOrderedMap(typ) {
		return g.processOrderedMap(field)
	}

	// Process as regular struct
	return g.processStructType(typ)
}

// processNumericType creates a type definition for basic numeric or string types.
func (g *Generator) processNumericType(typeName string, validateTag string) *TypeDefinition {
	typeDef := &TypeDefinition{Type: typeName}
	g.applyValidateConstraints(typeDef, validateTag, typeName)

	return typeDef
}

// processSliceType processes slice/array types and returns their schema definition.
func (g *Generator) processSliceType(typ reflect.Type, field reflect.StructField, validateTag string) (*TypeDefinition, error) {
	itemType, err := g.processType(typ.Elem(), field)
	if err != nil {
		return nil, err
	}

	typeDef := &TypeDefinition{
		Type:  "array",
		Items: itemType,
	}

	g.applyValidateConstraints(typeDef, validateTag, "array")

	return typeDef, nil
}

// processMapType processes map types and returns their schema definition.
func (g *Generator) processMapType(typ reflect.Type, field reflect.StructField) (*TypeDefinition, error) {
	valueType, err := g.processType(typ.Elem(), field)
	if err != nil {
		return nil, err
	}

	return &TypeDefinition{
		Type:                 "object",
		AdditionalProperties: &AdditionalPropertiesWrapper{Value: valueType},
	}, nil
}

// processStructType processes struct types and returns their schema definition.
func (g *Generator) processStructType(typ reflect.Type) (*TypeDefinition, error) {
	// Process as regular struct
	properties, required, err := g.processStruct(typ)
	if err != nil {
		return nil, err
	}

	return &TypeDefinition{
		Type:                 "object",
		Properties:           properties,
		Required:             RequiredList(required),
		AdditionalProperties: FalseAdditionalProperties(),
	}, nil
}

// applyValidateConstraints applies validation tag constraints to the type definition.
func (g *Generator) applyValidateConstraints(typeDef *TypeDefinition, validateTag, baseType string) {
	if validateTag == "" {
		return
	}

	tags := strings.SplitSeq(validateTag, ",")
	for tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || tag == "required" || tag == "omitempty" {
			continue
		}

		g.applyConstraintTag(typeDef, tag, baseType)
	}
}

func (g *Generator) applyConstraintTag(typeDef *TypeDefinition, tag, baseType string) {
	switch {
	case strings.HasPrefix(tag, "oneof="):
		g.applyOneofConstraint(typeDef, tag)
	case strings.HasPrefix(tag, "min="):
		g.applyMinConstraint(typeDef, tag, baseType)
	case strings.HasPrefix(tag, "max="):
		g.applyMaxConstraint(typeDef, tag, baseType)
	case strings.HasPrefix(tag, "len="):
		g.applyLenConstraint(typeDef, tag, baseType)
	default:
		g.applyFormatConstraint(typeDef, tag)
	}
}

func (g *Generator) applyOneofConstraint(typeDef *TypeDefinition, tag string) {
	val := strings.TrimPrefix(tag, "oneof=")
	values := strings.Fields(val)

	if len(values) > 0 {
		typeDef.Enum = values
	}
}

func (g *Generator) applyMinConstraint(typeDef *TypeDefinition, tag, baseType string) {
	val := strings.TrimPrefix(tag, "min=")

	intValue, err := parseInt(val)
	if err == nil {
		switch baseType {
		case "integer":
			typeDef.Minimum = &intValue
		case "string":
			typeDef.MinLength = &intValue
		}
	}
}

func (g *Generator) applyMaxConstraint(typeDef *TypeDefinition, tag, baseType string) {
	val := strings.TrimPrefix(tag, "max=")

	intValue, err := parseInt(val)
	if err == nil {
		switch baseType {
		case "integer":
			typeDef.Maximum = &intValue
		case "string":
			typeDef.MaxLength = &intValue
		}
	}
}

func (g *Generator) applyLenConstraint(typeDef *TypeDefinition, tag, baseType string) {
	if baseType != "string" {
		return
	}

	val := strings.TrimPrefix(tag, "len=")

	v, err := parseInt(val)
	if err == nil {
		typeDef.MinLength = &v
		typeDef.MaxLength = &v
	}
}

func (g *Generator) applyFormatConstraint(typeDef *TypeDefinition, tag string) {
	formats := map[string]struct {
		format  string
		pattern string
	}{
		"filepath": {format: "file-path"},
		"abspath":  {format: "uri-reference", pattern: "^/.*"},
		"uri":      {format: "uri"},
		"email":    {format: "email"},
		"url":      {format: "uri"},
		"uuid":     {format: "uuid"},
		"datetime": {format: "date-time"},
	}

	if f, ok := formats[tag]; ok {
		typeDef.Format = f.format
		if f.pattern != "" {
			typeDef.Pattern = f.pattern
		}
	}
}

func parseInt(s string) (int, error) {
	var parsed int

	_, err := fmt.Sscanf(s, "%d", &parsed)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse string '%s' to int", s)
	}

	return parsed, nil
}

// isOrderedMap checks if a type is an OrderedMap.
func (g *Generator) isOrderedMap(typ reflect.Type) bool {
	// OrderedMap is a struct with an embedded *omap.Omap field
	if typ.Kind() != reflect.Struct {
		return false
	}

	// Check if it has the Omap field which is characteristic of OrderedMap
	_, hasOmap := typ.FieldByName("Omap")

	return hasOmap
}

// processOrderedMap processes an OrderedMap type and returns its schema definition.
// Since Go doesn't provide access to generic type parameters at runtime via reflection,
// we manually handle the known OrderedMap types based on their field names.
func (g *Generator) processOrderedMap(field reflect.StructField) (*TypeDefinition, error) {
	// Get the field name from the yaml tag or struct field name
	yamlTag := field.Tag.Get("yaml")
	fieldName := field.Name

	if yamlTag != "" && yamlTag != "-" {
		parts := strings.Split(yamlTag, ",")
		if parts[0] != "" {
			fieldName = parts[0]
		}
	}

	// Get the value type for AdditionalProperties based on field name
	var valueType reflect.Type

	var allowNull bool

	switch fieldName {
	case "flakes":
		valueType = reflect.TypeFor[*config.Flake]()
		allowNull = false
	case "configurations":
		valueType = reflect.TypeFor[*config.Configuration]()
		allowNull = false
	case "machines":
		valueType = reflect.TypeFor[*config.Machine]()
		allowNull = true
	}

	if valueType != nil {
		valueSchema, err := g.processType(valueType, field)
		if err != nil {
			return nil, err
		}

		var additionalProps *AdditionalPropertiesWrapper
		if allowNull {
			// Allow null/empty values for machines (key-only is valid)
			additionalProps = &AdditionalPropertiesWrapper{
				Value: map[string]any{
					"anyOf": []any{
						valueSchema,
						map[string]any{"type": "null"},
					},
				},
			}
		} else {
			// Flakes and configurations require a value
			additionalProps = &AdditionalPropertiesWrapper{Value: valueSchema}
		}

		return &TypeDefinition{
			Type:                 "object",
			AdditionalProperties: additionalProps,
		}, nil
	}

	return &TypeDefinition{
		Type:                 "object",
		AdditionalProperties: &AdditionalPropertiesWrapper{Value: &TypeDefinition{Type: "object"}},
	}, nil
}

// getSpecialTypeDefinition returns special type definitions for known types.
func (g *Generator) getSpecialTypeDefinition(t reflect.Type) *TypeDefinition {
	// Handle time.Duration
	if t.String() == "time.Duration" {
		return &TypeDefinition{
			Type:        "string",
			Description: "Duration string (e.g., '2h', '30m', '1h30m')",
		}
	}

	return nil
}

// getYAMLFieldName extracts the field name from yaml tag or returns the struct field name.
func (g *Generator) getYAMLFieldName(field reflect.StructField, yamlTag string) string {
	if yamlTag == "" {
		return strings.ToLower(field.Name)
	}

	// Parse yaml tag: "name,omitempty" or just "name"
	parts := strings.Split(yamlTag, ",")
	if parts[0] == "" {
		return strings.ToLower(field.Name)
	}

	return parts[0]
}

// GenerateYAML generates the schema and returns it as YAML bytes.
func GenerateYAML() ([]byte, error) {
	gen := NewGenerator()

	schema, err := gen.Generate()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate schema")
	}

	data, err := yaml.Marshal(schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal schema to YAML")
	}

	return data, nil
}

// GenerateYAMLString generates the schema and returns it as a YAML string.
func GenerateYAMLString() (string, error) {
	bytes, err := GenerateYAML()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func GenerateSchema(outputPath string) error {
	schemaYAML, err := GenerateYAML()
	if err != nil {
		return errors.Wrap(err, "failed to generate schema")
	}

	if outputPath == "-" {
		fmt.Print(string(schemaYAML))

		return nil
	}

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		err = os.MkdirAll(dir, flags.DefaultDirPermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %s", dir)
		}
	}

	err = os.WriteFile(outputPath, schemaYAML, flags.DefaultLogFilePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write schema to %s", outputPath)
	}

	fmt.Printf("Schema written to: %s\n", outputPath)

	return nil
}
