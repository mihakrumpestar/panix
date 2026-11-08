// Package overrides preserves fields that use override (not append) semantics
// across an append-slice mergo merge.
package overrides

import "github.com/pkg/errors"

// Slices holds the pre-merge values of slice fields whose child value must win
// over the parent's. mergo.WithAppendSlice appends parent values into child
// slices, so callers capture these fields before the merge and restore them
// after it: a non-nil child value overrides the parent, nil inherits it.
type Slices []sliceValue

type sliceValue struct {
	field *[]string
	value []string
}

// Capture records the current value of each field. A nil value is skipped: nil
// means "inherit from the parent", so there is nothing to restore.
func Capture(fields ...*[]string) Slices {
	captured := make(Slices, 0, len(fields))

	for _, field := range fields {
		if field == nil || *field == nil {
			continue
		}

		captured = append(captured, sliceValue{field: field, value: *field})
	}

	return captured
}

// Restore writes the captured values back, undoing any parent append.
func (s Slices) Restore() {
	for _, captured := range s {
		*captured.field = captured.value
	}
}

// InheritEmpty copies a parent value into a child field that is still nil after
// the merge, when the parent value is set.
//
// mergo.WithAppendSlice cannot append an empty parent slice into a nil child, so
// a parent [] never reaches an unset child: it would fall back to the built-in
// default instead of inheriting "nothing". Both lists must hold the same fields
// in the same order, so a mismatch panics rather than copying into the wrong one.
func InheritEmpty(child, parent []*[]string) {
	if len(child) != len(parent) {
		panic(errors.Errorf("override field lists differ in length: %d child, %d parent", len(child), len(parent)))
	}

	for index, field := range child {
		if *field != nil {
			continue
		}

		if *parent[index] != nil {
			*field = *parent[index]
		}
	}
}
