package config

import (
	"cmp"
	"maps"
	"reflect"
	"slices"
)

// FoCoM
type FoCoM interface {
	Init(name string)

	Disable(msg string)
	IsDisabled() bool
	Msg(msg string)

	Children(skipDisabled bool) []FoCoM
}

type SortableFoCoM[K cmp.Ordered, V FoCoM] map[K]V

// Sorted map
func (m *SortableFoCoM[K, V]) SortedMap(skipChecks, skipDisabled bool) func(yield func(K, V) bool) {
	return func(yield func(K, V) bool) {
		if m == nil {
			return
		}

		keys := slices.Collect(maps.Keys(*m))
		slices.Sort(keys)

		for _, k := range keys {

			v := (*m)[k]

			if !skipChecks {
				// Skip disabled flakes, configurations, machines
				valueIsDisabled := !isNil(v) && v.IsDisabled()
				if valueIsDisabled && skipDisabled {
					continue
				}
			}

			if !yield(k, v) {
				break
			}
		}
	}
}

// Helpers

// isNil checks if the given value is nil using reflection
func isNil[V any](v V) bool {
	// Get the reflect.Value of the input
	rv := reflect.ValueOf(v)

	// If the value is invalid (zero value), it's nil
	if !rv.IsValid() {
		return true
	}

	// If the value is a pointer, interface, map, slice, function, or channel, check if it's nil
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	}

	// For other types, they can't be nil
	return false
}
