package theme

import "reflect"

// Merge returns the receiver with every field SET in override (non-zero:
// non-empty string/slice/map/pair, non-nil pointer) replaced by the override
// value; zero fields of override inherit from the receiver. This implements
// partial themes: missing fields inherit from the base theme, theme-set values
// win. The base for user and built-in partial themes is defaultTheme().
//
// Pointer fields (bold, pad_y, pad_x) distinguish unset (nil) from an explicit
// zero value: a non-nil pointer to false or 0 deliberately overrides. Empty
// non-nil slices and maps count as unset, matching the documented non-empty
// wording.
//
// Limitation: a deliberate empty string cannot override a non-empty base value,
// because unset and empty are indistinguishable. Pairs are all-or-nothing:
// validation rejects a pair with one empty side, so it never reaches Merge.
//
// Aliasing: set pointer and slice fields are returned from override directly,
// and inherited pointer and slice fields from the receiver directly, so a
// merged theme shares backing memory with its inputs. Callers that need an
// independent value must clone the result.
func (t Theme) Merge(override Theme) Theme {
	merged := mergeValue(reflect.ValueOf(t), reflect.ValueOf(override))

	result := merged.Interface().(Theme) //nolint:forcetypeassert // mergeValue preserves the Theme type

	return result.derive()
}

// mergeValue recursively merges two values of the same type: structs merge
// field-wise (so a partial sub-struct keeps its other fields), everything else
// follows "non-zero override wins, zero inherits". Empty slices and maps count as
// zero even when non-nil, matching the documented "non-empty" wording.
//
// Fields tagged yaml:"-" are derived render values: they are never merged (they
// are recomputed by the single derive pass), so a stale derived field can never
// leak through a merge.
func mergeValue(base, over reflect.Value) reflect.Value {
	if over.Kind() == reflect.Struct {
		typ := over.Type()
		merged := reflect.New(typ).Elem()

		for i := range over.NumField() {
			if isDerived(typ.Field(i)) {
				continue // derived field: recomputed by derive
			}

			merged.Field(i).Set(mergeValue(base.Field(i), over.Field(i)))
		}

		return merged
	}

	if over.Kind() == reflect.Slice || over.Kind() == reflect.Map {
		if over.Len() == 0 {
			return base
		}

		return over
	}

	if over.IsZero() {
		return base
	}

	return over
}
