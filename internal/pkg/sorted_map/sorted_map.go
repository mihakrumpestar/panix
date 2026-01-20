package sorted_map

import (
	"cmp"
	"maps"
	"slices"
)

type SortedMap[K cmp.Ordered, V any] map[K]V

// Sorted map
func (sm *SortedMap[K, V]) SortedMap() func(yield func(K, V) bool) {
	return func(yield func(K, V) bool) {
		if sm == nil {
			return
		}

		keys := slices.Collect(maps.Keys(*sm))
		slices.Sort(keys)

		for _, k := range keys {
			v := (*sm)[k]

			if !yield(k, v) {
				break
			}
		}
	}
}

// First
func (sm *SortedMap[K, V]) First() V {
	if sm == nil {
		return *new(V)
	}

	keys := slices.Collect(maps.Keys(*sm))
	slices.Sort(keys)

	if len(keys) == 0 {
		return *new(V)
	}

	k := keys[0]
	v := (*sm)[k]

	return v
}
