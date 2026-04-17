package atomicorderedmap

import (
	"encoding/json"
	"maps"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

var (
	ErrAtomicOrderedMapNil = errors.New("OrderedMap.UnmarshalJSON: nil receiver")
)

type Pair[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

type AtomicOrderedMap[K comparable, V any] struct {
	mu     sync.RWMutex
	keys   []K
	index  map[K]int
	values map[K]V `validate:"dive"`
}

func New[K comparable, V any]() *AtomicOrderedMap[K, V] {
	return &AtomicOrderedMap[K, V]{
		keys:   make([]K, 0),
		index:  make(map[K]int),
		values: make(map[K]V),
	}
}

func (m *AtomicOrderedMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.index == nil {
		m.index = make(map[K]int)
	}

	if m.values == nil {
		m.values = make(map[K]V)
	}

	if _, exists := m.index[key]; exists {
		m.values[key] = value

		return
	}

	m.index[key] = len(m.keys)
	m.keys = append(m.keys, key)
	m.values[key] = value
}

func (m *AtomicOrderedMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.values[key]

	return val, ok
}

func (m *AtomicOrderedMap[K, V]) Exists(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.index[key]

	return ok
}

func (m *AtomicOrderedMap[K, V]) Del(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := m.index[key]
	if !ok {
		return
	}

	delete(m.values, key)
	delete(m.index, key)

	m.keys = append(m.keys[:idx], m.keys[idx+1:]...)

	for k, i := range m.index {
		if i > idx {
			m.index[k] = i - 1
		}
	}
}

func (m *AtomicOrderedMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.keys)
}

func (m *AtomicOrderedMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.keys = m.keys[:0]
	for k := range m.index {
		delete(m.index, k)
	}

	for k := range m.values {
		delete(m.values, k)
	}
}

// Pairs returns all key-value pairs in insertion order.
// Nil-safe: returns nil if the AtomicOrderedMap is nil.
func (m *AtomicOrderedMap[K, V]) Pairs() []Pair[K, V] {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Pair[K, V], len(m.keys))
	for i, k := range m.keys {
		result[i] = Pair[K, V]{Key: k, Value: m.values[k]}
	}

	return result
}

// Records returns a map of all key-value pairs.
// Nil-safe: returns nil if the AtomicOrderedMap is nil.
func (m *AtomicOrderedMap[K, V]) Records() map[K]V {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[K]V, len(m.keys))
	maps.Copy(result, m.values)

	return result
}

// DeleteFunc removes entries matching the predicate.
// Nil-safe: no-op if the AtomicOrderedMap is nil.
func (m *AtomicOrderedMap[K, V]) DeleteFunc(pred func(K, V) bool) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(m.keys) - 1; i >= 0; i-- {
		k := m.keys[i]
		if pred(k, m.values[k]) {
			delete(m.values, k)
			delete(m.index, k)
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
		}
	}

	m.rebuildIndex()
}

// Last returns the last pair in insertion order.
// Nil-safe: returns zero value and false if nil or empty.
func (m *AtomicOrderedMap[K, V]) Last() (Pair[K, V], bool) {
	if m == nil {
		var zero Pair[K, V]

		return zero, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.keys) == 0 {
		var zero Pair[K, V]

		return zero, false
	}

	k := m.keys[len(m.keys)-1]

	return Pair[K, V]{Key: k, Value: m.values[k]}, true
}

func (m *AtomicOrderedMap[K, V]) rebuildIndex() {
	for i, k := range m.keys {
		m.index[k] = i
	}
}

// JSON marshaling/unmarshaling

func (m *AtomicOrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(m.Pairs())

	return b, errors.Wrap(err, "marshal ordered map")
}

func (m *AtomicOrderedMap[K, V]) UnmarshalJSON(data []byte) error {
	if m == nil {
		return ErrAtomicOrderedMapNil
	}

	var pairs []Pair[K, V]

	err := json.Unmarshal(data, &pairs)
	if err != nil {
		return errors.Wrap(err, "unmarshal ordered map")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.index == nil {
		m.index = make(map[K]int)
	}

	if m.values == nil {
		m.values = make(map[K]V)
	}

	m.keys = m.keys[:0]
	for k := range m.index {
		delete(m.index, k)
	}

	for k := range m.values {
		delete(m.values, k)
	}

	for _, p := range pairs {
		m.index[p.Key] = len(m.keys)
		m.keys = append(m.keys, p.Key)
		m.values[p.Key] = p.Value
	}

	return nil
}

// YAML marshaling/unmarshaling

var _ yaml.InterfaceMarshaler = (*AtomicOrderedMap[string, any])(nil)

func (m AtomicOrderedMap[K, V]) MarshalYAML() (any, error) {
	pairs := m.Pairs()

	result := make(yaml.MapSlice, 0, len(pairs))
	for _, p := range pairs {
		result = append(result, yaml.MapItem{Key: p.Key, Value: p.Value})
	}

	return result, nil
}

var _ yaml.InterfaceUnmarshaler = (*AtomicOrderedMap[string, any])(nil)

func (m *AtomicOrderedMap[K, V]) UnmarshalYAML(decode func(any) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.index == nil {
		m.index = make(map[K]int)
	}

	if m.values == nil {
		m.values = make(map[K]V)
	}

	m.keys = m.keys[:0]
	for k := range m.index {
		delete(m.index, k)
	}

	for k := range m.values {
		delete(m.values, k)
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

		m.index[key] = len(m.keys)
		m.keys = append(m.keys, key)
		m.values[key] = value
	}

	return nil
}
