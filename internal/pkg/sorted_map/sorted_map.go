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
