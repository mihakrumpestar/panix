package atomicorderedmap

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/kirill-scherba/omap"
	"github.com/pkg/errors"
)

type Pair[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

type OrderedMap[K comparable, V any] struct {
	*omap.Omap[K, V] `validate:"-"`
}

func New[K comparable, V any]() *OrderedMap[K, V] {
	m, err := omap.New[K, V]()
	if err != nil {
		panic(fmt.Sprintf("failed to create omap: %v", err))
	}

	return &OrderedMap[K, V]{Omap: m}
}

// Pairs returns all key-value pairs in insertion order.
// Nil-safe: returns nil if the OrderedMap or its underlying omap is nil.
// This can happen after JSON unmarshaling creates a zero-value OrderedMap
// whose UnmarshalJSON was never called (e.g., missing JSON key).
func (m *OrderedMap[K, V]) Pairs() []Pair[K, V] {
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
func (m *OrderedMap[K, V]) DeleteFunc(pred func(K, V) bool) {
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
func (m *OrderedMap[K, V]) Last() (Pair[K, V], bool) {
	if m == nil || m.Omap == nil {
		var zero Pair[K, V]
		return zero, false
	}
	pairs := m.Omap.Pairs()
	if len(pairs) == 0 {
		var zero Pair[K, V]
		return zero, false
	}
	p := pairs[len(pairs)-1]
	return Pair[K, V]{Key: p.Key, Value: p.Value}, true
}

// Get retrieves a value by key.
// Nil-safe: returns zero value and false if the OrderedMap or its underlying omap is nil.
func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	var zero V
	if m == nil || m.Omap == nil {
		return zero, false
	}
	return m.Omap.Get(key)
}

// Set inserts or updates a key-value pair.
// Nil-safe: returns an error if the OrderedMap or its underlying omap is nil.
func (m *OrderedMap[K, V]) Set(key K, value V) error {
	if m == nil || m.Omap == nil {
		return errors.New("ordered map is nil")
	}
	return m.Omap.Set(key, value)
}

// Len returns the number of entries.
// Nil-safe: returns 0 if the OrderedMap or its underlying omap is nil.
func (m *OrderedMap[K, V]) Len() int {
	if m == nil || m.Omap == nil {
		return 0
	}
	return m.Omap.Len()
}

func (m *OrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Pairs())
}

func (m *OrderedMap[K, V]) UnmarshalJSON(data []byte) error {
	var pairs []Pair[K, V]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	if m.Omap == nil {
		omapInstance, err := omap.New[K, V]()
		if err != nil {
			return errors.Wrap(err, "failed to create omap")
		}
		m.Omap = omapInstance
	} else {
		m.Omap.Clear()
	}

	for _, p := range pairs {
		if err := m.Omap.Set(p.Key, p.Value); err != nil {
			return errors.Wrapf(err, "failed to set key %v", p.Key)
		}
	}

	return nil
}

var _ yaml.BytesUnmarshaler = (*OrderedMap[string, any])(nil)

func (m *OrderedMap[K, V]) UnmarshalYAML(data []byte) error {
	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return errors.Wrap(err, "failed to parse YAML")
	}

	if m.Omap == nil {
		omapInstance, err := omap.New[K, V]()
		if err != nil {
			return errors.Wrap(err, "failed to create omap")
		}
		m.Omap = omapInstance
	} else {
		m.Omap.Clear()
	}

	if file == nil || len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return nil
	}

	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil
	}

	valueType := reflect.TypeFor[V]()
	isPtr := valueType.Kind() == reflect.Pointer
	elemType := valueType
	if isPtr {
		elemType = valueType.Elem()
	}

	for _, val := range mapping.Values {
		key, value, processErr := processMappingValue[K, V](val, valueType, isPtr, elemType)
		if processErr != nil {
			return processErr
		}

		if err := m.Omap.Set(key, value); err != nil {
			return errors.Wrapf(err, "failed to set key %v", key)
		}
	}

	return nil
}

func processMappingValue[K comparable, V any](
	val *ast.MappingValueNode,
	valueType reflect.Type, isPtr bool, elemType reflect.Type,
) (K, V, error) {
	var (
		key   K
		value V
	)

	if err := yaml.NodeToValue(val.Key, &key); err != nil {
		return key, value, errors.Wrap(err, "failed to unmarshal key")
	}

	if _, isNull := val.Value.(*ast.NullNode); isNull {
		if isPtr && elemType.Kind() == reflect.Struct {
			newPtr := reflect.New(elemType)
			var ok bool
			value, ok = newPtr.Interface().(V)
			if !ok {
				panic(fmt.Sprintf("type assertion failed: expected %T, got %T", value, newPtr.Interface()))
			}
		}

		return key, value, nil
	}

	if !isPtr {
		if err := yaml.NodeToValue(val.Value, &value); err != nil {
			return key, value, errors.Wrapf(err, "failed to unmarshal %v", key)
		}

		return key, value, nil
	}

	if elemType.Kind() == reflect.Struct {
		newPtr := reflect.New(elemType)
		if err := yaml.NodeToValue(val.Value, newPtr.Interface()); err != nil {
			return key, value, errors.Wrapf(err, "failed to unmarshal %v", key)
		}

		v, ok := newPtr.Interface().(V)
		if !ok {
			return key, value, errors.Errorf("type assertion failed: expected %T, got %T", value, newPtr.Interface())
		}

		value = v
	} else {
		if err := yaml.NodeToValue(val.Value, &value); err != nil {
			return key, value, errors.Wrapf(err, "failed to unmarshal %v", key)
		}
	}

	return key, value, nil
}
