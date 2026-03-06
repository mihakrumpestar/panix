package config_schema

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
)

// Generator holds the state for schema generation
type Generator struct {
	visited map[reflect.Type]bool
}

// RequiredList is a list of required fields that always serializes properly
// even when empty (unlike []string with omitempty)
type RequiredList []string

func (r RequiredList) MarshalYAML() (interface{}, error) {
	if len(r) == 0 {
		return nil, nil
	}
	return []string(r), nil
}

// TypeDefinition represents a schema type definition
type TypeDefinition struct {
	Type                 string                 `yaml:"type,omitempty"`
	Description          string                 `yaml:"description,omitempty"`
	Properties           map[string]interface{} `yaml:"properties,omitempty"`
	Items                interface{}            `yaml:"items,omitempty"`
	Required             RequiredList           `yaml:"required,omitempty"`
	Enum                 []string               `yaml:"enum,omitempty"`
	Pattern              string                 `yaml:"pattern,omitempty"`
	Example              interface{}            `yaml:"example,omitempty"`
	AdditionalProperties interface{}            `yaml:"additionalProperties,omitempty"`
	AnyOf                []interface{}          `yaml:"anyOf,omitempty"`
	AllOf                []interface{}          `yaml:"allOf,omitempty"`
	Ref                  string                 `yaml:"$ref,omitempty"`
	Format               string                 `yaml:"format,omitempty"`
	Minimum              *int                   `yaml:"minimum,omitempty"`
	Maximum              *int                   `yaml:"maximum,omitempty"`
	MinLength            *int                   `yaml:"minLength,omitempty"`
	MaxLength            *int                   `yaml:"maxLength,omitempty"`
}

// Schema represents the root YAML schema document
type Schema struct {
	Schema               string                 `yaml:"$schema"`
	ID                   string                 `yaml:"$id,omitempty"`
	Version              string                 `yaml:"version,omitempty"`
	Title                string                 `yaml:"title,omitempty"`
	Description          string                 `yaml:"description,omitempty"`
	Type                 string                 `yaml:"type,omitempty"`
	Properties           map[string]interface{} `yaml:"properties,omitempty"`
	Required             RequiredList           `yaml:"required,omitempty"`
	AdditionalProperties interface{}            `yaml:"additionalProperties,omitempty"`
	Definitions          map[string]interface{} `yaml:"definitions,omitempty"`
	AllOf                []interface{}          `yaml:"allOf,omitempty"`
}

// NewGenerator creates a new schema generator
func NewGenerator() *Generator {
	return &Generator{
		visited: make(map[reflect.Type]bool),
	}
}

// Generate creates a YAML schema from the config.Config type
func (g *Generator) Generate() (*Schema, error) {
	schema := &Schema{
		Schema:               "http://json-schema.org/draft-07/schema#",
		ID:                   "https://panix.dev/schema.json",
		Version:              gen.Version(),
		Title:                "Panix Configuration Schema",
		Description:          "Schema for Panix NixOS deployment configuration files",
		Type:                 "object",
		Properties:           make(map[string]interface{}),
		Required:             RequiredList{},
		AdditionalProperties: false,
		Definitions:          make(map[string]interface{}),
	}

	// Generate schema from config.Config
	cfgType := reflect.TypeOf(config.Config{})

	properties, required, err := g.processStruct(cfgType)
	if err != nil {
		return nil, fmt.Errorf("failed to process Config struct: %w", err)
	}

	schema.Properties = properties
	schema.Required = required

	return schema, nil
}

// processStruct processes a struct type and returns its properties and required fields
func (g *Generator) hasExactValidateTag(validateTag, tag string) bool {
	tags := strings.Split(validateTag, ",")
	for _, t := range tags {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
}

func (g *Generator) processStruct(t reflect.Type) (map[string]interface{}, []string, error) {
	properties := make(map[string]interface{})
	required := []string{}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("expected struct type, got %v", t.Kind())
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Skip CLI-only fields (marked with comment "CLI-only")
		// These should not appear in the YAML schema
		if strings.Contains(field.Tag.Get("desc"), "CLI-only") ||
			strings.Contains(field.Tag.Get("flag"), " ") && field.Name == "Config" {
			continue
		}

		// Skip internal fields (marked with yaml:"-")
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "-" {
			continue
		}

		// Handle inline embedded structs - flatten their properties into parent
		if strings.Contains(yamlTag, ",inline") {
			inlineProps, inlineRequired, err := g.processStruct(field.Type)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to process inline field %s: %w", field.Name, err)
			}

			for name, prop := range inlineProps {
				properties[name] = prop
			}

			required = append(required, inlineRequired...)
			continue
		}

		// Get the field name from yaml tag or use the struct field name
		fieldName := g.getYAMLFieldName(field, yamlTag)

		// Process the field type
		prop, err := g.processType(field.Type, field)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process field %s: %w", field.Name, err)
		}

		// Add description from desc tag, falling back to help tag
		desc := field.Tag.Get("desc")
		if desc == "" {
			desc = field.Tag.Get("help")
		}

		if desc != "" {
			if td, ok := prop.(*TypeDefinition); ok {
				td.Description = desc
			}
		}

		properties[fieldName] = prop

		// Check if field is required based on yaml or validate tags
		validateTag := field.Tag.Get("validate")
		yamlHasRequired := strings.Contains(yamlTag, ",required")
		validateHasRequired := g.hasExactValidateTag(validateTag, "required")

		if yamlHasRequired || validateHasRequired {
			required = append(required, fieldName)
		}
	}

	return properties, required, nil
}

// processType processes a type and returns its schema definition
func (g *Generator) processType(t reflect.Type, field reflect.StructField) (interface{}, error) {
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check for special types first
	if def := g.getSpecialTypeDefinition(t); def != nil {
		return def, nil
	}

	validateTag := field.Tag.Get("validate")

	switch t.Kind() {
	case reflect.Bool:
		return &TypeDefinition{Type: "boolean"}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		td := &TypeDefinition{Type: "integer"}
		g.applyValidateConstraints(td, validateTag, "integer")
		return td, nil

	case reflect.Float32, reflect.Float64:
		td := &TypeDefinition{Type: "number"}
		g.applyValidateConstraints(td, validateTag, "number")
		return td, nil

	case reflect.String:
		td := &TypeDefinition{Type: "string"}
		g.applyValidateConstraints(td, validateTag, "string")
		return td, nil

	case reflect.Slice, reflect.Array:
		itemType, err := g.processType(t.Elem(), field)
		if err != nil {
			return nil, err
		}
		td := &TypeDefinition{
			Type:  "array",
			Items: itemType,
		}
		g.applyValidateConstraints(td, validateTag, "array")
		return td, nil

	case reflect.Map:
		// Handle regular maps
		valueType, err := g.processType(t.Elem(), field)
		if err != nil {
			return nil, err
		}
		return &TypeDefinition{
			Type:                 "object",
			AdditionalProperties: valueType,
		}, nil

	case reflect.Struct:
		// Check if this is an OrderedMap
		if g.isOrderedMap(t) {
			return g.processOrderedMap(t, field)
		}

		// Process as regular struct
		properties, required, err := g.processStruct(t)
		if err != nil {
			return nil, err
		}

		return &TypeDefinition{
			Type:                 "object",
			Properties:           properties,
			Required:             RequiredList(required),
			AdditionalProperties: false,
		}, nil

	case reflect.Interface:
		// Interfaces can be any type
		return &TypeDefinition{}, nil

	default:
		return nil, fmt.Errorf("unsupported type kind: %v", t.Kind())
	}
}

// applyValidateConstraints applies validation tag constraints to the type definition
func (g *Generator) applyValidateConstraints(td *TypeDefinition, validateTag, baseType string) {
	if validateTag == "" {
		return
	}

	tags := strings.Split(validateTag, ",")
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || tag == "required" || tag == "omitempty" {
			continue
		}

		if strings.HasPrefix(tag, "min=") {
			val := strings.TrimPrefix(tag, "min=")
			if v, err := parseInt(val); err == nil {
				switch baseType {
				case "integer":
					td.Minimum = &v
				case "string":
					td.MinLength = &v
				}
			}
		} else if strings.HasPrefix(tag, "max=") {
			val := strings.TrimPrefix(tag, "max=")
			if v, err := parseInt(val); err == nil {
				switch baseType {
				case "integer":
					td.Maximum = &v
				case "string":
					td.MaxLength = &v
				}
			}
		} else if strings.HasPrefix(tag, "len=") {
			val := strings.TrimPrefix(tag, "len=")
			if v, err := parseInt(val); err == nil {
				switch baseType {
				case "string":
					td.MinLength = &v
					td.MaxLength = &v
				}
			}
		} else {
			// Handle format tags
			switch tag {
			case "filepath":
				td.Format = "file-path"
			case "abspath":
				td.Format = "uri-reference"
				td.Pattern = "^/.*"
			case "uri":
				td.Format = "uri"
			case "email":
				td.Format = "email"
			case "url":
				td.Format = "uri"
			case "uuid":
				td.Format = "uuid"
			case "datetime":
				td.Format = "date-time"
			}
		}
	}
}

// parseInt parses a string to an integer
func parseInt(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// isOrderedMap checks if a type is an OrderedMap
func (g *Generator) isOrderedMap(t reflect.Type) bool {
	// OrderedMap is a struct with an embedded *omap.Omap field
	if t.Kind() != reflect.Struct {
		return false
	}

	// Check if it has the Omap field which is characteristic of OrderedMap
	_, hasOmap := t.FieldByName("Omap")
	return hasOmap
}

// processOrderedMap processes an OrderedMap type and returns its schema definition
// Since Go doesn't provide access to generic type parameters at runtime via reflection,
// we manually handle the known OrderedMap types based on their field names
func (g *Generator) processOrderedMap(t reflect.Type, field reflect.StructField) (*TypeDefinition, error) {
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
		valueType = reflect.TypeOf(&config.Flake{})
		allowNull = false
	case "configurations":
		valueType = reflect.TypeOf(&config.Configuration{})
		allowNull = false
	case "machines":
		valueType = reflect.TypeOf(&config.Machine{})
		allowNull = true
	}

	if valueType != nil {
		valueSchema, err := g.processType(valueType, field)
		if err != nil {
			return nil, err
		}

		var additionalProps interface{}
		if allowNull {
			// Allow null/empty values for machines (key-only is valid)
			additionalProps = map[string]interface{}{
				"anyOf": []interface{}{
					valueSchema,
					map[string]interface{}{"type": "null"},
				},
			}
		} else {
			// Flakes and configurations require a value
			additionalProps = valueSchema
		}

		return &TypeDefinition{
			Type:                 "object",
			AdditionalProperties: additionalProps,
		}, nil
	}

	return &TypeDefinition{
		Type:                 "object",
		AdditionalProperties: &TypeDefinition{Type: "object"},
	}, nil
}

// getSpecialTypeDefinition returns special type definitions for known types
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

// getYAMLFieldName extracts the field name from yaml tag or returns the struct field name
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

// GenerateYAML generates the schema and returns it as YAML bytes
func GenerateYAML() ([]byte, error) {
	gen := NewGenerator()

	schema, err := gen.Generate()
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(schema)
}

// GenerateYAMLString generates the schema and returns it as a YAML string
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
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	if outputPath == "-" {
		fmt.Print(string(schemaYAML))
		return nil
	}

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, config_flags.DefaultDirPermissions); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(outputPath, schemaYAML, config_flags.DefaultLogFilePermissions); err != nil {
		return fmt.Errorf("failed to write schema to %s: %w", outputPath, err)
	}

	fmt.Printf("Schema written to: %s\n", outputPath)
	return nil
}
