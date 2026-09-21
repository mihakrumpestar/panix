package yamlschema

import (
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type generator struct {
	config      SchemaConfig
	definitions map[string]any
	defTypes    map[reflect.Type]string
	typeCounts  map[reflect.Type]int
}

type SchemaConfig struct {
	RootType    reflect.Type
	SchemaID    string
	Title       string
	Description string
	Version     string
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

type dependencyList []string

func (d dependencyList) MarshalYAML() (any, error) {
	if len(d) == 0 {
		return []string{}, nil
	}

	return []string(d), nil
}

type dependenciesMap map[string]dependencyList

func (d dependenciesMap) MarshalYAML() (any, error) {
	if len(d) == 0 {
		return map[string]dependencyList{}, nil
	}

	return map[string]dependencyList(d), nil
}

// anyOfVariant is one branch of an object-level "at least one of" constraint:
// every field it lists must be present.
type anyOfVariant struct {
	Required requiredList `yaml:"required"`
}

// anyOfGroup is one constraint derived from required_without declarations,
// e.g. [{required: [local_path]}, {required: [command]}].
type anyOfGroup []anyOfVariant

type TypeDefinition struct {
	Type                 string                       `yaml:"type,omitempty"`
	Description          string                       `yaml:"description,omitempty"`
	Default              any                          `yaml:"default,omitempty"`
	Properties           map[string]any               `yaml:"properties,omitempty"`
	Items                any                          `yaml:"items,omitempty"`
	Required             requiredList                 `yaml:"required,omitempty"`
	Dependencies         dependenciesMap              `yaml:"dependencies,omitempty"`
	AnyOf                anyOfGroup                   `yaml:"anyOf,omitempty"`
	AllOf                []*TypeDefinition            `yaml:"allOf,omitempty"`
	Enum                 []string                     `yaml:"enum,omitempty"`
	Pattern              string                       `yaml:"pattern,omitempty"`
	AdditionalProperties *additionalPropertiesWrapper `yaml:"additionalProperties,omitempty"`
	Ref                  string                       `yaml:"$ref,omitempty"`
	Format               string                       `yaml:"format,omitempty"`
}

type Schema struct {
	Schema               string            `yaml:"$schema"`
	ID                   string            `yaml:"$id,omitempty"`
	Version              string            `yaml:"version,omitempty"`
	Title                string            `yaml:"title,omitempty"`
	Description          string            `yaml:"description,omitempty"`
	Type                 string            `yaml:"type,omitempty"`
	Properties           map[string]any    `yaml:"properties,omitempty"`
	Required             requiredList      `yaml:"required,omitempty"`
	AnyOf                anyOfGroup        `yaml:"anyOf,omitempty"`
	AllOf                []*TypeDefinition `yaml:"allOf,omitempty"`
	AdditionalProperties any               `yaml:"additionalProperties,omitempty"`
	Definitions          map[string]any    `yaml:"definitions,omitempty"`
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

func NewSchema(cfg SchemaConfig) *generator {
	gen := &generator{
		config:      cfg,
		definitions: make(map[string]any),
		defTypes:    make(map[reflect.Type]string),
		typeCounts:  make(map[reflect.Type]int),
	}

	gen.countStructOccurrences(cfg.RootType, make(map[reflect.Type]bool))

	for typ, count := range gen.typeCounts {
		if count > 1 {
			gen.defTypes[typ] = typ.Name()
		}
	}

	return gen
}

// findMapValueType detects map-like struct wrappers (no YAML-visible fields,
// one map[K]V payload, possibly reached through embedded fields). The
// validate:"dive" map wins when several exist, as it marks the main content.
func findMapValueType(typ reflect.Type) reflect.Type {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil
	}

	// If the struct has any YAML-visible properties, it's not a map wrapper
	if hasYAMLVisibleFields(typ) {
		return nil
	}

	// No YAML-visible properties; look for a map[K]V field directly.
	if vt := findMapFieldType(typ); vt != nil {
		return vt
	}

	// Recurse into embedded anonymous struct fields (e.g., OutputMap embeds *AtomicOrderedMap).
	for field := range typ.Fields() {
		if !field.Anonymous {
			continue
		}

		if vt := findMapValueType(field.Type); vt != nil {
			return vt
		}
	}

	return nil
}

// hasYAMLVisibleFields reports whether the struct has any exported fields
// not tagged with yaml:"-".
func hasYAMLVisibleFields(typ reflect.Type) bool {
	for field := range typ.Fields() {
		if !field.IsExported() || field.Tag.Get("yaml") == "-" {
			continue
		}

		return true
	}

	return false
}

// findMapFieldType finds the value type of a map field in the struct,
// preferring fields with validate:"dive".
func findMapFieldType(typ reflect.Type) reflect.Type {
	// Prefer the field with validate:"dive" as it marks the main content.
	for field := range typ.Fields() {
		if field.Type.Kind() != reflect.Map {
			continue
		}

		if strings.Contains(field.Tag.Get("validate"), "dive") {
			return field.Type.Elem()
		}
	}

	// Fallback: any map field
	for field := range typ.Fields() {
		if field.Type.Kind() == reflect.Map {
			return field.Type.Elem()
		}
	}

	return nil
}

func (g *generator) Generate() (*Schema, error) {
	properties, required, _, anyOf, err := g.processStruct(g.config.RootType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to process root struct")
	}

	schema := &Schema{
		Schema:               "http://json-schema.org/draft-07/schema#",
		ID:                   g.config.SchemaID,
		Version:              g.config.Version,
		Title:                g.config.Title,
		Description:          g.config.Description,
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: true,
	}

	schema.AnyOf, schema.AllOf = requiredWithoutSchema(anyOf)

	if len(g.definitions) > 0 {
		schema.Definitions = g.definitions
	}

	return schema, nil
}

// countStructOccurrences counts how often each struct type appears as a field
// type. Inline fields count as the parent's; stack-based cycle detection keeps
// infinite recursion out while diamond patterns still count fully.
func (g *generator) countStructOccurrences(typ reflect.Type, onStack map[reflect.Type]bool) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return
	}

	if onStack[typ] {
		return
	}

	// Check if this struct is a map-like wrapper (no YAML-visible properties, has map[K]V field)
	if mapValueType := findMapValueType(typ); mapValueType != nil {
		g.countFieldTypeInfo(mapValueType)

		return
	}

	onStack[typ] = true

	for field := range typ.Fields() {
		if !field.IsExported() || field.Tag.Get("yaml") == "-" {
			continue
		}

		if strings.Contains(field.Tag.Get("yaml"), ",inline") {
			g.countStructOccurrences(field.Type, onStack)

			continue
		}

		g.countFieldType(field.Type)
	}

	delete(onStack, typ)
}

// countFieldType increments the occurrence count for a struct field's type.
func (g *generator) countFieldType(typ reflect.Type) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.Slice {
		typ = typ.Elem()
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
	}

	if typ.Kind() == reflect.Struct {
		g.typeCounts[typ]++
		g.countStructOccurrences(typ, make(map[reflect.Type]bool))
	}
}

// countFieldTypeInfo increments the occurrence count for a type that appears
// as the value type of a map-like struct (e.g., AtomicOrderedMap values).
func (g *generator) countFieldTypeInfo(typ reflect.Type) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.Struct {
		g.typeCounts[typ]++
		g.countStructOccurrences(typ, make(map[reflect.Type]bool))
	}
}

//nolint:funlen // one cohesive field scan; the per-feature collectors live in their own helpers
func (g *generator) processStruct(structType reflect.Type) (map[string]any, requiredList, dependenciesMap, []anyOfGroup, error) {
	properties := make(map[string]any)

	var required requiredList

	dependencies := make(dependenciesMap)

	var anyOf []anyOfGroup

	seenAnyOf := make(map[string]bool)

	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	if structType.Kind() != reflect.Struct {
		return nil, nil, nil, nil, errors.Errorf("expected struct type, got %v", structType.Kind())
	}

	for field := range structType.Fields() {
		if !field.IsExported() || field.Tag.Get("yaml") == "-" {
			continue
		}

		yamlTag := field.Tag.Get("yaml")

		if strings.Contains(yamlTag, ",inline") {
			inlineProps, inlineRequired, inlineDeps, inlineAnyOf, err := g.processStruct(field.Type)
			if err != nil {
				return nil, nil, nil, nil, errors.Wrapf(err, "failed to process inline field %s", field.Name)
			}

			maps.Copy(properties, inlineProps)

			required = append(required, inlineRequired...)

			maps.Copy(dependencies, inlineDeps)

			anyOf = append(anyOf, inlineAnyOf...)

			continue
		}

		fieldName := yamlFieldName(field, yamlTag)

		prop, err := g.processType(field.Type, field)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrapf(err, "failed to process field %s", field.Name)
		}

		g.setFieldDescription(prop, field)
		properties[fieldName] = prop

		if isFieldRequired(field, yamlTag) {
			required = append(required, fieldName)
		}

		g.collectDependencies(structType, field, fieldName, dependencies)

		anyOf = appendRequiredWithout(anyOf, seenAnyOf, collectRequiredWithout(structType, field, fieldName))
	}

	return properties, required, dependencies, anyOf, nil
}

// appendRequiredWithout deduplicates required_without groups by canonical key
// while preserving declaration order.
func appendRequiredWithout(anyOf []anyOfGroup, seen map[string]bool, groups []anyOfGroup) []anyOfGroup {
	for _, group := range groups {
		key := anyOfGroupKey(group)
		if seen[key] {
			continue
		}

		seen[key] = true

		anyOf = append(anyOf, group)
	}

	return anyOf
}

// collectRequiredWithout translates a required_without=Field declaration into
// an object-level anyOf group: field A declaring required_without=P1 P2
// becomes anyOf: [{required: [P1, P2]}, {required: [A]}], which reads "A is
// required whenever any of the listed fields is absent".
func collectRequiredWithout(structType reflect.Type, field reflect.StructField, fieldName string) []anyOfGroup {
	validateTag := field.Tag.Get("validate")
	if validateTag == "" {
		return nil
	}

	var groups []anyOfGroup

	for tag := range strings.SplitSeq(validateTag, ",") {
		tag = strings.TrimSpace(tag)

		params, ok := strings.CutPrefix(tag, "required_without=")
		if !ok {
			continue
		}

		deps := strings.Fields(params)
		if len(deps) == 0 {
			continue
		}

		requiredDeps := make(requiredList, 0, len(deps))
		for _, dep := range deps {
			requiredDeps = append(requiredDeps, resolveYAMLFieldName(structType, dep))
		}

		slices.Sort(requiredDeps)

		groups = append(groups, anyOfGroup{
			{Required: requiredDeps},
			{Required: requiredList{fieldName}},
		})
	}

	return groups
}

// anyOfGroupKey canonicalizes a group into its set of variants so symmetric
// declarations (a required_without b, b required_without a) deduplicate to a
// single group regardless of declaration order.
func anyOfGroupKey(group anyOfGroup) string {
	variants := make([]string, 0, len(group))

	for _, variant := range group {
		required := slices.Clone(variant.Required)
		slices.Sort(required)

		variants = append(variants, strings.Join(required, " "))
	}

	slices.Sort(variants)

	return strings.Join(variants, "\x00")
}

// requiredWithoutSchema attaches required_without groups to an object schema:
// one group is emitted directly as anyOf, several independent groups become
// allOf of anyOf so each stays its own "at least one of" constraint.
func requiredWithoutSchema(groups []anyOfGroup) (anyOfGroup, []*TypeDefinition) {
	switch len(groups) {
	case 0:
		return nil, nil
	case 1:
		return groups[0], nil
	default:
		allOf := make([]*TypeDefinition, 0, len(groups))
		for _, group := range groups {
			allOf = append(allOf, &TypeDefinition{AnyOf: group})
		}

		return nil, allOf
	}
}

func (g *generator) collectDependencies(structType reflect.Type, field reflect.StructField, fieldName string, dependencies dependenciesMap) {
	validateTag := field.Tag.Get("validate")
	if validateTag == "" {
		return
	}

	for tag := range strings.SplitSeq(validateTag, ",") {
		tag = strings.TrimSpace(tag)

		depFields, ok := strings.CutPrefix(tag, "required_with=")
		if !ok {
			continue
		}

		goField := strings.TrimSpace(depFields)

		yamlName := resolveYAMLFieldName(structType, goField)

		_, ok = dependencies[yamlName]
		if !ok {
			dependencies[yamlName] = dependencyList{}
		}

		dependencies[yamlName] = append(dependencies[yamlName], fieldName)
	}
}

func resolveYAMLFieldName(structType reflect.Type, goFieldName string) string {
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	if structType.Kind() != reflect.Struct {
		return strings.ToLower(goFieldName)
	}

	for f := range structType.Fields() {
		if f.Name == goFieldName {
			yt := f.Tag.Get("yaml")
			if yt == "" {
				return strings.ToLower(goFieldName)
			}

			name, _, _ := strings.Cut(yt, ",")
			if name != "" {
				return name
			}

			return strings.ToLower(goFieldName)
		}
	}

	return strings.ToLower(goFieldName)
}

func (g *generator) processType(typ reflect.Type, field reflect.StructField) (any, error) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	// Types seen more than once become $ref definitions.
	defName, ok := g.defTypes[typ]
	if ok {
		return g.processDefinitionType(typ, defName)
	}

	// Handle time.Duration
	if typ.String() == "time.Duration" {
		return &TypeDefinition{Type: "string", Description: "Duration string (e.g., '2h', '30m', '1h30m')"}, nil
	}

	// Check if this struct is a map-like wrapper (no YAML-visible properties, has map[K]V field)
	if typ.Kind() == reflect.Struct {
		if mapValueType := findMapValueType(typ); mapValueType != nil {
			return g.processMapType(mapValueType, field)
		}
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
		return g.processSlice(typ, field)

	case reflect.Map:
		return g.processMap(typ, field)

	case reflect.Struct:
		td, err := g.buildObjectTypeDef(typ)
		if err != nil {
			return nil, err
		}

		return td, nil

	default:
		return nil, errors.Errorf("unsupported type kind: %v", kind)
	}
}

func (g *generator) processSlice(typ reflect.Type, field reflect.StructField) (any, error) {
	itemType, err := g.processType(typ.Elem(), field)
	if err != nil {
		return nil, err
	}

	return g.applyConstraints(&TypeDefinition{Type: "array", Items: itemType}, field.Tag.Get("validate")), nil
}

func (g *generator) processMap(typ reflect.Type, field reflect.StructField) (any, error) {
	valueType, err := g.processType(typ.Elem(), field)
	if err != nil {
		return nil, err
	}

	return &TypeDefinition{
		Type:                 "object",
		AdditionalProperties: &additionalPropertiesWrapper{Value: valueType},
	}, nil
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

		val, ok := strings.CutPrefix(tag, "oneof=")
		if ok {
			values := strings.Fields(val)
			if len(values) > 0 {
				typeDef.Enum = values
			}

			continue
		}

		f, ok := formatConstraints[tag]
		if ok {
			typeDef.Format = f.format
			if f.pattern != "" {
				typeDef.Pattern = f.pattern
			}
		}
	}

	return typeDef
}

func (g *generator) processDefinitionType(typ reflect.Type, defName string) (any, error) {
	_, ok := g.definitions[defName]
	if ok {
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
	properties, required, dependencies, anyOf, err := g.processStruct(typ)
	if err != nil {
		return nil, err
	}

	typeDef := &TypeDefinition{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		Dependencies:         dependencies,
		AdditionalProperties: FalseAdditionalProperties(),
	}

	typeDef.AnyOf, typeDef.AllOf = requiredWithoutSchema(anyOf)

	return typeDef, nil
}

// processMapType emits an object whose additionalProperties is the value
// schema. The schema tag can additionally allow null values or nest a second
// map level.
func (g *generator) processMapType(valueType reflect.Type, field reflect.StructField) (*TypeDefinition, error) {
	valueSchema, err := g.processType(valueType, reflect.StructField{})
	if err != nil {
		return nil, err
	}

	additionalProps := &additionalPropertiesWrapper{Value: valueSchema}

	schemaTag := field.Tag.Get("schema")
	if schemaTag != "" {
		for tag := range strings.SplitSeq(schemaTag, ",") {
			tag = strings.TrimSpace(tag)

			switch tag {
			case "nullable_values":
				additionalProps = &additionalPropertiesWrapper{
					Value: map[string]any{
						"anyOf": []any{
							valueSchema,
							map[string]any{"type": "null"},
						},
					},
				}

			case "two_level_map":
				// Wrap the value schema in another additionalProperties layer,
				// producing: { type: object, additionalProperties: { type: object, additionalProperties: <valueSchema> } }
				innerTypeDef := &TypeDefinition{
					Type:                 "object",
					AdditionalProperties: additionalProps,
				}
				additionalProps = &additionalPropertiesWrapper{Value: innerTypeDef}
			}
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

	defaultVal := field.Tag.Get("default")
	if defaultVal != "" {
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

	name, _, _ := strings.Cut(yamlTag, ",")
	if name != "" {
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
