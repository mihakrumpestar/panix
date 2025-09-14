package config

import (
	"cmp"
	"maps"
	"reflect"
	"slices"
)

type FlakeOrConfigurationOrMachine interface {
	IsDisabled() bool
}

type SortableMap[K cmp.Ordered, V FlakeOrConfigurationOrMachine] map[K]V

// KeyValuePair holds a key-value pair for sorted iteration.
type KeyValuePair[K cmp.Ordered, V FlakeOrConfigurationOrMachine] struct {
	Key   K
	Value V
}

// Range provides a method that allows for key, value := range SortableMap.Range() syntax.
func (m *SortableMap[K, V]) Range(skipDisabled bool) func(yield func(K, V) bool) {
	return func(yield func(K, V) bool) {
		if m == nil {
			return
		}

		keys := slices.Collect(maps.Keys(*m))
		slices.Sort(keys)

		for _, k := range keys {

			v := (*m)[k]

			// Skip disabled flakes, configurations, machines
			valueIsDisabled := !isNil(v) && v.IsDisabled()
			if valueIsDisabled && skipDisabled {
				continue
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
