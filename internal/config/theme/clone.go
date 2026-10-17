package theme

import "reflect"

// isDerived reports whether field is a derived render value: all derived fields
// are tagged yaml:"-" and are recomputed by derive instead of copied, merged or
// validated.
func isDerived(field reflect.StructField) bool {
	return field.Tag.Get("yaml") == "-"
}

// clone returns a deep copy of the theme: serializable fields (pointers, slices,
// arrays and nested structs) are copied recursively, derived fields are recomputed
// by derive, so the copy is render-ready and never aliases the source.
func (t Theme) clone() Theme {
	return cloneValue(reflect.ValueOf(t)).Interface().(Theme).derive() //nolint:forcetypeassert // cloneValue preserves the Theme type
}

// cloneValue recursively deep-copies a serializable theme value.
//
//nolint:cyclop // one branch per reflect kind
func cloneValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Struct:
		cloned := reflect.New(value.Type()).Elem()
		typ := value.Type()

		for index := range value.NumField() {
			if isDerived(typ.Field(index)) {
				continue // derive recomputes derived render values
			}

			cloned.Field(index).Set(cloneValue(value.Field(index)))
		}

		return cloned
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}

		cloned := reflect.New(value.Type().Elem())
		cloned.Elem().Set(cloneValue(value.Elem()))

		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}

		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := range value.Len() {
			cloned.Index(index).Set(cloneValue(value.Index(index)))
		}

		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for index := range value.Len() {
			cloned.Index(index).Set(cloneValue(value.Index(index)))
		}

		return cloned
	default:
		return value
	}
}
