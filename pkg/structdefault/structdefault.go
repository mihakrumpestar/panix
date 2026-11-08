// Package structdefault materializes the values declared in `default` struct
// tags, so a built-in default is written once next to the field it belongs to.
//
// A list default is a bracket list, for example
// `default:"[--no-link, --print-out-paths]"`. The same tag feeds the generated
// JSON schema (pkg/yamlschema), so a default is never repeated elsewhere.
package structdefault

import (
	"reflect"
	"strings"
)

// configPkgPrefix bounds reflection to configuration types: runtime state
// (atomics, maps, logs) is never walked.
const configPkgPrefix = "github.com/mihakrumpestar/panix/internal/config/"

// valueTag marks a defaulted field. An explicit [] is a deliberate "pass
// nothing" or an empty command, so only a nil slice takes the default.
const valueTag = "default"

// Apply fills every unset string-slice field reachable from dst that declares a
// `default` tag. Only a nil slice takes the default, because an explicit [] is a
// value the operator chose. Apply runs after inheritance is merged, so a
// configured or inherited value always wins. Only configuration packages are
// walked, through struct fields only: no config block sits behind a pointer.
func Apply(dst any) {
	value := reflect.ValueOf(dst)

	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return
		}

		value = value.Elem()
	}

	apply(value)
}

func apply(value reflect.Value) {
	if value.Kind() != reflect.Struct || !isConfigType(value.Type()) {
		return
	}

	for index := range value.NumField() {
		field := value.Field(index)
		if !field.CanSet() {
			continue
		}

		structField := value.Type().Field(index)

		switch field.Kind() {
		case reflect.Slice:
			fillSlice(field, structField.Tag.Get(valueTag), field.IsNil())
		case reflect.Struct:
			apply(field)
		}
	}
}

// fillSlice sets field from tag when fill is true and the field is a string
// slice.
func fillSlice(field reflect.Value, tag string, fill bool) {
	if !fill || tag == "" || field.Type().Elem().Kind() != reflect.String {
		return
	}

	items, ok := ParseList(tag)
	if !ok {
		return
	}

	slice := reflect.MakeSlice(field.Type(), len(items), len(items))
	for index, item := range items {
		slice.Index(index).SetString(item)
	}

	field.Set(slice)
}

// isConfigType reports whether typ belongs to the configuration packages, which
// is where the load-time defaults live.
func isConfigType(typ reflect.Type) bool {
	return strings.HasPrefix(typ.PkgPath(), configPkgPrefix)
}

// String returns the `default` tag of the named field on T, or "" when the
// field declares none. Scalar defaults are read where they are used, because
// their zero value differs per type.
func String[T any](field string) string {
	return tagOf[T](field, valueTag)
}

// List returns the built-in list of the named field on T, or nil when the field
// declares none. Callers use it to compare a configured value against its
// built-in default instead of applying it.
func List[T any](field string) []string {
	items, _ := ParseList(tagOf[T](field, valueTag))

	return items
}

func tagOf[T any](field, tag string) string {
	structField, ok := reflect.TypeFor[T]().FieldByName(field)
	if !ok {
		return ""
	}

	return structField.Tag.Get(tag)
}

// ParseList parses a bracket-list tag value: items are comma separated,
// surrounding spaces are trimmed, and inner spaces are preserved so
// `nix-command flakes` stays one argument. `[]` is a valid empty list.
func ParseList(tag string) ([]string, bool) {
	trimmed := strings.TrimSpace(tag)
	if len(trimmed) < 2 || trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' {
		return nil, false
	}

	inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if inner == "" {
		return []string{}, true
	}

	parts := strings.Split(inner, ",")
	items := make([]string, 0, len(parts))

	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items, true
}
