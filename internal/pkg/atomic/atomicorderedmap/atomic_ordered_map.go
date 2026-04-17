package atomicorderedmap

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/kirill-scherba/omap"
	"github.com/pkg/errors"
)

type Pair[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

type AtomicOrderedMap[K comparable, V any] struct {
	*omap.Omap[K, V] `validate:"-"`
}

func New[K comparable, V any]() *AtomicOrderedMap[K, V] {
	m, err := omap.New[K, V]()
	if err != nil {
		panic(fmt.Sprintf("failed to create omap: %v", err))
	}

	return &AtomicOrderedMap[K, V]{Omap: m}
}

// Pairs returns all key-value pairs in insertion order.
// Nil-safe: returns nil if the OrderedMap or its underlying omap is nil.
// This can happen after JSON unmarshaling creates a zero-value OrderedMap
// whose UnmarshalJSON was never called (e.g., missing JSON key).
func (m *AtomicOrderedMap[K, V]) Pairs() []Pair[K, V] {
	if m == nil || m.Omap == nil {
		return nil
	}

	pairs := m.Omap.Pairs()

	result := make([]Pair[K, V], len(pairs))
	for i, p := range pairs {
		result[i] = Pair[K, V]{Key: p.Key, Value: p.Value}
	}

	return result
}

// DeleteFunc removes entries matching the predicate.
// Nil-safe: no-op if the OrderedMap or its underlying omap is nil.
func (m *AtomicOrderedMap[K, V]) DeleteFunc(pred func(K, V) bool) {
	if m == nil || m.Omap == nil {
		return
	}

	pairs := m.Omap.Pairs()
	for _, p := range pairs {
		if pred(p.Key, p.Value) {
			m.Omap.Del(p.Key)
		}
	}
}

// Last returns the last pair in insertion order.
// Nil-safe: returns zero value and false if the OrderedMap or its underlying omap is nil.
func (m *AtomicOrderedMap[K, V]) Last() (Pair[K, V], bool) {
	if m == nil || m.Omap == nil || m.Omap.Len() == 0 {
		var zero Pair[K, V]

		return zero, false
	}

	pairs := m.Omap.Pairs()
	p := pairs[len(pairs)-1]

	return Pair[K, V]{Key: p.Key, Value: p.Value}, true
}

func (m *AtomicOrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(m.Pairs())

	return b, errors.Wrap(err, "marshal ordered map")
}

func (m *AtomicOrderedMap[K, V]) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("OrderedMap.UnmarshalJSON: nil receiver")
	}

	var pairs []omap.Pair[K, V]

	err := json.Unmarshal(data, &pairs)
	if err != nil {
		return errors.Wrap(err, "unmarshal ordered map")
	}

	if m.Omap == nil {
		var omapInstance *omap.Omap[K, V]

		omapInstance, err = omap.New[K, V]()
		if err != nil {
			return errors.Wrap(err, "failed to create omap")
		}

		m.Omap = omapInstance
	} else {
		m.Omap.Clear()
	}

	for _, p := range pairs {
		err = m.Omap.Set(p.Key, p.Value)
		if err != nil {
			return errors.Wrapf(err, "failed to set key %v", p.Key)
		}
	}

	return nil
}

var _ yaml.InterfaceMarshaler = (*AtomicOrderedMap[string, any])(nil)

func (m AtomicOrderedMap[K, V]) MarshalYAML() (any, error) {
	if m.Omap == nil {
		return yaml.MapSlice{}, nil
	}

	pairs := m.Omap.Pairs()

	result := make(yaml.MapSlice, 0, len(pairs))
	for _, p := range pairs {
		result = append(result, yaml.MapItem{Key: p.Key, Value: p.Value})
	}

	return result, nil
}

var _ yaml.InterfaceUnmarshaler = (*AtomicOrderedMap[string, any])(nil)

func (m *AtomicOrderedMap[K, V]) UnmarshalYAML(decode func(any) error) error {
	if m.Omap == nil {
		omapInstance, err := omap.New[K, V]()
		if err != nil {
			return errors.Wrap(err, "failed to create omap")
		}

		m.Omap = omapInstance
	} else {
		m.Omap.Clear()
	}

	typed := make(map[K]V)

	err := decode(&typed)
	if err != nil {
		return errors.Wrap(err, "failed to decode ordered map")
	}

	var mapSlice yaml.MapSlice

	err = decode(&mapSlice)
	if err != nil {
		return errors.Wrap(err, "failed to decode ordered map keys")
	}

	for _, item := range mapSlice {
		key, ok := item.Key.(K)
		if !ok {
			continue
		}

		value, exists := typed[key]
		if !exists {
			continue
		}

		err = m.Omap.Set(key, value)
		if err != nil {
			return errors.Wrapf(err, "failed to set key %v", key)
		}
	}

	return nil
}
