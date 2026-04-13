package orderedmap

import (
	"encoding/json"
	"fmt"
	"reflect"

	yaml "github.com/goccy/go-yaml"
)

type Pair[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

type OrderedMap[K comparable, V any] struct {
	pairs []Pair[K, V]
	index map[K]int
}

func New[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		pairs: make([]Pair[K, V], 0),
		index: make(map[K]int),
	}
}

func (m *OrderedMap[K, V]) buildIndex() {
	if m.index != nil {
		return
	}
	m.index = make(map[K]int, len(m.pairs))
	for i, p := range m.pairs {
		m.index[p.Key] = i
	}
}

func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	m.buildIndex()
	i, ok := m.index[key]
	if !ok {
		var zero V
		return zero, false
	}
	return m.pairs[i].Value, true
}

func (m *OrderedMap[K, V]) Set(key K, value V) {
	m.buildIndex()
	if i, ok := m.index[key]; ok {
		m.pairs[i].Value = value
		return
	}
	m.pairs = append(m.pairs, Pair[K, V]{Key: key, Value: value})
	m.index[key] = len(m.pairs) - 1
}

func (m *OrderedMap[K, V]) Delete(key K) {
	m.buildIndex()
	i, ok := m.index[key]
	if !ok {
		return
	}
	m.pairs = append(m.pairs[:i], m.pairs[i+1:]...)
	delete(m.index, key)
	for k, idx := range m.index {
		if idx > i {
			m.index[k] = idx - 1
		}
	}
}

func (m *OrderedMap[K, V]) DeleteFunc(pred func(K, V) bool) {
	m.buildIndex()
	n := 0
	for _, p := range m.pairs {
		if pred(p.Key, p.Value) {
			delete(m.index, p.Key)
		} else {
			m.pairs[n] = p
			m.index[p.Key] = n
			n++
		}
	}
	m.pairs = m.pairs[:n]
}

func (m *OrderedMap[K, V]) Clear() {
	m.pairs = m.pairs[:0]
	clear(m.index)
}

func (m *OrderedMap[K, V]) Range(yield func(K, V) bool) {
	for _, p := range m.pairs {
		if !yield(p.Key, p.Value) {
			return
		}
	}
}

func (m *OrderedMap[K, V]) Last() (Pair[K, V], bool) {
	if len(m.pairs) == 0 {
		var zero Pair[K, V]
		return zero, false
	}
	return m.pairs[len(m.pairs)-1], true
}

func (m *OrderedMap[K, V]) Pairs() []Pair[K, V] {
	return m.pairs
}

func (m *OrderedMap[K, V]) Len() int {
	return len(m.pairs)
}

func (m *OrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.pairs)
}

func (m *OrderedMap[K, V]) UnmarshalJSON(data []byte) error {
	var pairs []Pair[K, V]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}
	m.pairs = pairs
	m.index = nil
	return nil
}

var _ yaml.InterfaceUnmarshaler = (*OrderedMap[string, any])(nil)

func (m *OrderedMap[K, V]) UnmarshalYAML(unmarshal func(any) error) error {
	var ms yaml.MapSlice
	if err := unmarshal(&ms); err != nil {
		return err
	}
	m.pairs = make([]Pair[K, V], 0, len(ms))
	m.index = make(map[K]int, len(ms))
	for _, item := range ms {
		rawKey, ok := item.Key.(string)
		if !ok {
			return fmt.Errorf("orderedmap: non-string key: %v", item.Key)
		}
		var key K
		b, err := yaml.Marshal(rawKey)
		if err != nil {
			return fmt.Errorf("orderedmap: marshal key: %w", err)
		}
		if err := yaml.Unmarshal(b, &key); err != nil {
			return fmt.Errorf("orderedmap: unmarshal key %q: %w", rawKey, err)
		}
		var val V
		// Skip null/nil values — leave val as zero value for its type
		if item.Value == nil {
			idx := len(m.pairs)
			m.pairs = append(m.pairs, Pair[K, V]{Key: key, Value: val})
			m.index[key] = idx
			continue
		}
		b, err = yaml.Marshal(item.Value)
		if err != nil {
			return fmt.Errorf("orderedmap: marshal value for key %q: %w", rawKey, err)
		}
		// For pointer value types, allocate a zero value before unmarshaling
		// so the YAML decoder can populate the pointed-to struct.
		valType := reflect.TypeOf(val)
		if valType != nil && valType.Kind() == reflect.Pointer {
			val = reflect.New(valType.Elem()).Interface().(V)
			if err := yaml.Unmarshal(b, val); err != nil {
				return fmt.Errorf("orderedmap: unmarshal value for key %q: %w", rawKey, err)
			}
		} else {
			if err := yaml.Unmarshal(b, &val); err != nil {
				return fmt.Errorf("orderedmap: unmarshal value for key %q: %w", rawKey, err)
			}
		}
		idx := len(m.pairs)
		m.pairs = append(m.pairs, Pair[K, V]{Key: key, Value: val})
		m.index[key] = idx
	}

	return nil
}
