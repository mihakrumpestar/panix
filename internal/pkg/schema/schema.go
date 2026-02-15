// Package schema provides functionality to generate YAML schema from Go structs
// using YAML tags. It properly handles special types like OrderedMap.
package schema

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"gopkg.in/yaml.v3"
)

// Generator holds the state for schema generation
type Generator struct {
	definitions map[string]*TypeDefinition
	visited     map[reflect.Type]bool
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
}

// Schema represents the root YAML schema document
type Schema struct {
	Schema               string                 `yaml:"$schema"`
	ID                   string                 `yaml:"$id,omitempty"`
	Title                string                 `yaml:"title,omitempty"`
	Description          string                 `yaml:"description,omitempty"`
	Type                 string                 `yaml:"type,omitempty"`
	Properties           map[string]interface{} `yaml:"properties,omitempty"`
	Required             RequiredList           `yaml:"required,omitempty"`
	AdditionalProperties interface{}            `yaml:"additionalProperties,omitempty"`
	Definitions          map[string]interface{} `yaml:"definitions,omitempty"`
}

// NewGenerator creates a new schema generator
func NewGenerator() *Generator {
	return &Generator{
		definitions: make(map[string]*TypeDefinition),
		visited:     make(map[reflect.Type]bool),
	}
}

// Generate creates a YAML schema from the config.Config type
func (g *Generator) Generate() (*Schema, error) {
	schema := &Schema{
		Schema:               "http://json-schema.org/draft-07/schema#",
		ID:                   "https://panix.dev/schema.json",
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

	// Add collected definitions
	for name, def := range g.definitions {
		schema.Definitions[name] = def
	}

	return schema, nil
}

// processStruct processes a struct type and returns its properties, required fields, and any definitions
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

		// Add description from desc tag if present
		if desc := field.Tag.Get("desc"); desc != "" {
			if td, ok := prop.(*TypeDefinition); ok {
				td.Description = desc
			}
		}

		properties[fieldName] = prop

		// Check if field is required
		// A field is required only if:
		// 1. It has a "required" struct tag
		// OR
		// 2. It's not a pointer, not inline, AND marked with a special "required" yaml option
		// For now, we only mark fields as required if they have explicit "required" tag
		isRequired := field.Tag.Get("required") == "true"

		if isRequired {
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

	switch t.Kind() {
	case reflect.Bool:
		return &TypeDefinition{Type: "boolean"}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &TypeDefinition{Type: "integer"}, nil

	case reflect.Float32, reflect.Float64:
		return &TypeDefinition{Type: "number"}, nil

	case reflect.String:
		return &TypeDefinition{Type: "string"}, nil

	case reflect.Slice, reflect.Array:
		itemType, err := g.processType(t.Elem(), field)
		if err != nil {
			return nil, err
		}
		return &TypeDefinition{
			Type:  "array",
			Items: itemType,
		}, nil

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
	switch fieldName {
	case "flakes":
		valueType = reflect.TypeOf(&config.Flake{})
	case "configurations":
		valueType = reflect.TypeOf(&config.Configuration{})
	case "machines":
		valueType = reflect.TypeOf(&config.Machine{})
	}

	if valueType != nil {
		valueSchema, err := g.processType(valueType, field)
		if err != nil {
			return nil, err
		}
		// Allow null/empty values for OrderedMap entries (key-only is valid)
		return &TypeDefinition{
			Type: "object",
			AdditionalProperties: map[string]interface{}{
				"anyOf": []interface{}{
					valueSchema,
					map[string]interface{}{"type": "null"},
				},
			},
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
