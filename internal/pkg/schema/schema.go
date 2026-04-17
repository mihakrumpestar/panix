package schema

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/pkg/errors"
)

type generator struct {
	definitions map[string]any
	defTypes    map[reflect.Type]string
	orderedDefs map[reflect.Type]orderedMapDef
}

type orderedMapDef struct {
	valueType reflect.Type
	allowNull bool
}

type requiredList []string

func (r requiredList) MarshalYAML() (any, error) {
	if len(r) == 0 {
		return []string{}, nil
	}

	return []string(r), nil
}

type additionalPropertiesWrapper struct {
	Value any
}

func (ap *additionalPropertiesWrapper) MarshalYAML() (any, error) {
	return ap.Value, nil
}

func FalseAdditionalProperties() *additionalPropertiesWrapper {
	return &additionalPropertiesWrapper{Value: false}
}

type TypeDefinition struct {
	Type                 string                       `yaml:"type,omitempty"`
	Description          string                       `yaml:"description,omitempty"`
	Default              any                          `yaml:"default,omitempty"`
	Properties           map[string]any               `yaml:"properties,omitempty"`
	Items                any                          `yaml:"items,omitempty"`
	Required             requiredList                 `yaml:"required,omitempty"`
	Enum                 []string                     `yaml:"enum,omitempty"`
	Pattern              string                       `yaml:"pattern,omitempty"`
	AdditionalProperties *additionalPropertiesWrapper `yaml:"additionalProperties,omitempty"`
	Ref                  string                       `yaml:"$ref,omitempty"`
	Format               string                       `yaml:"format,omitempty"`
}

type Schema struct {
	Schema               string         `yaml:"$schema"`
	ID                   string         `yaml:"$id,omitempty"`
	Version              string         `yaml:"version,omitempty"`
	Title                string         `yaml:"title,omitempty"`
	Description          string         `yaml:"description,omitempty"`
	Type                 string         `yaml:"type,omitempty"`
	Properties           map[string]any `yaml:"properties,omitempty"`
	Required             requiredList   `yaml:"required,omitempty"`
	AdditionalProperties any            `yaml:"additionalProperties,omitempty"`
	Definitions          map[string]any `yaml:"definitions,omitempty"`
}

var formatConstraints = map[string]struct {
	format  string
	pattern string
}{
	"filepath": {format: "file-path"},
	"abspath":  {format: "uri-reference", pattern: "^/.*"},
	"uri":      {format: "uri"},
	"url":      {format: "uri"},
}

func newGenerator() *generator {
	generator := &generator{
		definitions: make(map[string]any),
		defTypes:    make(map[reflect.Type]string),
		orderedDefs: make(map[reflect.Type]orderedMapDef),
	}

	generator.initDefinitionTypes()

	return generator
}

func (g *generator) initDefinitionTypes() {
	g.defTypes[reflect.TypeFor[attributes.Bootstrap]()] = "Bootstrap"
	g.defTypes[reflect.TypeFor[attributes.NixConfig]()] = "NixConfig"
	g.defTypes[reflect.TypeFor[attributes.PlainFileOrDirToTransfer]()] = "FileTransfer"
	g.defTypes[reflect.TypeFor[attributes.KexecConfig]()] = "KexecConfig"
	g.defTypes[reflect.TypeFor[ssh.SSHClient]()] = "SSH"

	g.orderedDefs[reflect.TypeFor[atomicorderedmap.AtomicOrderedMap[string, *flake.Flake]]()] = orderedMapDef{
		valueType: reflect.TypeFor[*flake.Flake](),
	}
	g.orderedDefs[reflect.TypeFor[atomicorderedmap.AtomicOrderedMap[string, *configuration.Configuration]]()] = orderedMapDef{
		valueType: reflect.TypeFor[*configuration.Configuration](),
	}
	g.orderedDefs[reflect.TypeFor[atomicorderedmap.AtomicOrderedMap[string, *machine.Machine]]()] = orderedMapDef{
		valueType: reflect.TypeFor[*machine.Machine](),
		allowNull: true,
	}
}

func (g *generator) generate() (*Schema, error) {
	properties, required, err := g.processStruct(reflect.TypeFor[config.Config]())
	if err != nil {
		return nil, errors.Wrap(err, "failed to process Config struct")
	}

	schema := &Schema{
		Schema:               "http://json-schema.org/draft-07/schema#",
		ID:                   "https://panix.dev/schema.json",
		Version:              gen.Version(),
		Title:                "Panix Configuration Schema",
		Description:          "Schema for Panix NixOS deployment configuration files",
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: true,
	}

	if len(g.definitions) > 0 {
		schema.Definitions = g.definitions
	}

	return schema, nil
}

func (g *generator) processStruct(structType reflect.Type) (map[string]any, requiredList, error) {
	properties := make(map[string]any)

	var required requiredList

	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	if structType.Kind() != reflect.Struct {
		return nil, nil, errors.Errorf("expected struct type, got %v", structType.Kind())
	}

	for field := range structType.Fields() {
		if !field.IsExported() || field.Tag.Get("yaml") == "-" {
			continue
		}

		yamlTag := field.Tag.Get("yaml")

		if strings.Contains(yamlTag, ",inline") {
			inlineProps, inlineRequired, err := g.processStruct(field.Type)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to process inline field %s", field.Name)
			}

			maps.Copy(properties, inlineProps)

			required = append(required, inlineRequired...)

			continue
		}

		fieldName := yamlFieldName(field, yamlTag)

		prop, err := g.processType(field.Type, field)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to process field %s", field.Name)
		}

		g.setFieldDescription(prop, field)
		properties[fieldName] = prop

		if isFieldRequired(field, yamlTag) {
			required = append(required, fieldName)
		}
	}

	return properties, required, nil
}

func (g *generator) processType(typ reflect.Type, field reflect.StructField) (any, error) {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if defName, ok := g.defTypes[typ]; ok {
		return g.processDefinitionType(typ, defName)
	}

	if typ.String() == "time.Duration" {
		return &TypeDefinition{Type: "string", Description: "Duration string (e.g., '2h', '30m', '1h30m')"}, nil
	}

	return g.processByKind(typ.Kind(), typ, field)
}

func (g *generator) processByKind(kind reflect.Kind, typ reflect.Type, field reflect.StructField) (any, error) {
	switch kind {
	case reflect.Bool:
		return &TypeDefinition{Type: "boolean"}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return g.applyConstraints(&TypeDefinition{Type: "integer"}, field.Tag.Get("validate")), nil

	case reflect.String:
		return g.applyConstraints(&TypeDefinition{Type: "string"}, field.Tag.Get("validate")), nil

	case reflect.Slice:
		itemType, err := g.processType(typ.Elem(), field)
		if err != nil {
			return nil, err
		}

		return g.applyConstraints(&TypeDefinition{Type: "array", Items: itemType}, field.Tag.Get("validate")), nil

	case reflect.Struct:
		if def, ok := g.orderedDefs[typ]; ok {
			return g.processOrderedMap(def)
		}

		td, err := g.buildObjectTypeDef(typ)
		if err != nil {
			return nil, err
		}

		return td, nil

	default:
		return nil, errors.Errorf("unsupported type kind: %v", kind)
	}
}

func (g *generator) applyConstraints(typeDef *TypeDefinition, validateTag string) *TypeDefinition {
	if validateTag == "" {
		return typeDef
	}

	for tag := range strings.SplitSeq(validateTag, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "" || tag == "required" || tag == "omitempty" {
			continue
		}

		if val, ok := strings.CutPrefix(tag, "oneof="); ok {
			if values := strings.Fields(val); len(values) > 0 {
				typeDef.Enum = values
			}

			continue
		}

		if f, ok := formatConstraints[tag]; ok {
			typeDef.Format = f.format
			if f.pattern != "" {
				typeDef.Pattern = f.pattern
			}
		}
	}

	return typeDef
}

func (g *generator) processDefinitionType(typ reflect.Type, defName string) (any, error) {
	if _, exists := g.definitions[defName]; exists {
		return &TypeDefinition{Ref: "#/definitions/" + defName}, nil
	}

	td, err := g.buildObjectTypeDef(typ)
	if err != nil {
		return nil, err
	}

	g.definitions[defName] = td

	return &TypeDefinition{Ref: "#/definitions/" + defName}, nil
}

func (g *generator) buildObjectTypeDef(typ reflect.Type) (*TypeDefinition, error) {
	properties, required, err := g.processStruct(typ)
	if err != nil {
		return nil, err
	}

	return &TypeDefinition{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: FalseAdditionalProperties(),
	}, nil
}

func (g *generator) processOrderedMap(def orderedMapDef) (*TypeDefinition, error) {
	valueSchema, err := g.processType(def.valueType, reflect.StructField{})
	if err != nil {
		return nil, err
	}

	additionalProps := &additionalPropertiesWrapper{Value: valueSchema}
	if def.allowNull {
		additionalProps = &additionalPropertiesWrapper{
			Value: map[string]any{
				"anyOf": []any{
					valueSchema,
					map[string]any{"type": "null"},
				},
			},
		}
	}

	return &TypeDefinition{
		Type:                 "object",
		AdditionalProperties: additionalProps,
	}, nil
}

func (g *generator) setFieldDescription(prop any, field reflect.StructField) {
	typeDef, ok := prop.(*TypeDefinition)
	if !ok {
		return
	}

	desc := field.Tag.Get("desc")
	if desc == "" {
		desc = field.Tag.Get("help")
	}

	if defaultVal := field.Tag.Get("default"); defaultVal != "" {
		typeDef.Default = parseDefaultValue(defaultVal, field.Type)
		if desc != "" {
			desc += " (default: " + defaultVal + ")"
		}
	}

	if desc != "" {
		typeDef.Description = desc
	}
}

func isFieldRequired(field reflect.StructField, yamlTag string) bool {
	if strings.Contains(yamlTag, ",required") {
		return true
	}

	for t := range strings.SplitSeq(field.Tag.Get("validate"), ",") {
		if strings.TrimSpace(t) == "required" {
			return true
		}
	}

	return false
}

func yamlFieldName(field reflect.StructField, yamlTag string) string {
	if yamlTag == "" {
		return strings.ToLower(field.Name)
	}

	if name := strings.Split(yamlTag, ",")[0]; name != "" {
		return name
	}

	return strings.ToLower(field.Name)
}

func parseDefaultValue(val string, typ reflect.Type) any {
	if typ.String() == "time.Duration" {
		return val
	}

	switch typ.Kind() {
	case reflect.Bool:
		return val == "true"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		intVal, err := strconv.Atoi(val)
		if err == nil {
			return intVal
		}

		return val
	default:
		return val
	}
}

func generateYAML() ([]byte, error) {
	gen := newGenerator()

	schema, err := gen.generate()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate schema")
	}

	data, err := yaml.Marshal(schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal schema to YAML")
	}

	return data, nil
}

func GenerateSchema(outputPath string) error {
	schemaYAML, err := generateYAML()
	if err != nil {
		return errors.Wrap(err, "failed to generate schema")
	}

	if outputPath == "-" {
		fmt.Print(string(schemaYAML))

		return nil
	}

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		err = os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %s", dir)
		}
	}

	err = os.WriteFile(outputPath, schemaYAML, filepermissions.DefaultFilePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write schema to %s", outputPath)
	}

	fmt.Printf("Schema written to: %s\n", outputPath)

	return nil
}
