package config

import (
	"fmt"
	"reflect"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	omap "github.com/kirill-scherba/omap"
	"github.com/pkg/errors"
)

var ErrTypeAssertionFailed = errors.New("type assertion failed")

// OrderedMap is a wrapper around omap.Omap that provides YAML unmarshaling support
// compatible with github.com/goccy/go-yaml.
type OrderedMap[K comparable, V any] struct {
	*omap.Omap[K, V] `validate:"-"`
}

// NewOrderedMap creates a new OrderedMap.
func NewOrderedMap[K comparable, V any]() (*OrderedMap[K, V], error) {
	m, err := omap.New[K, V]()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ordered map")
	}

	return &OrderedMap[K, V]{Omap: m}, nil
}

// UnmarshalYAML implements yaml.BytesUnmarshaler interface.
// It unmarshals a YAML mapping into the ordered map while preserving key order.
// Keys without values are stored with the zero value for type V.
func (om *OrderedMap[K, V]) UnmarshalYAML(data []byte) error {
	// Create a new omap if needed
	if om.Omap == nil {
		m, err := omap.New[K, V]()
		if err != nil {
			return errors.Wrap(err, "failed to create ordered map")
		}

		om.Omap = m
	}

	// Clear existing data
	om.Omap.Clear()

	// Parse the YAML to extract keys and values in order
	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return errors.Wrap(err, "failed to parse YAML")
	}

	// Walk the AST and extract key-value pairs
	if file != nil && file.Docs != nil && len(file.Docs) > 0 && file.Docs[0].Body != nil {
		extractor := &keyValueExtractor[K, V]{om: om}
		ast.Walk(extractor, file.Docs[0].Body)

		if extractor.err != nil {
			return extractor.err
		}
	}

	return nil
}

// keyValueExtractor is an AST visitor that extracts key-value pairs from mapping nodes.
type keyValueExtractor[K comparable, V any] struct {
	om  *OrderedMap[K, V]
	err error
}

func (e *keyValueExtractor[K, V]) Visit(node ast.Node) ast.Visitor {
	mapping, ok := node.(*ast.MappingNode)
	if !ok {
		return e
	}

	valueType := reflect.TypeOf((*V)(nil)).Elem()
	elemType := valueType

	isPtr := valueType.Kind() == reflect.Ptr
	if isPtr {
		elemType = valueType.Elem()
	}

	for _, val := range mapping.Values {
		key, value, err := e.processMappingValue(val, valueType, isPtr, elemType)
		if err != nil {
			e.err = err

			return nil
		}

		err = e.om.Omap.Set(key, value)
		if err != nil {
			e.err = errors.Wrapf(err, "failed to set key %v", key)

			return nil
		}
	}

	return nil
}

func (e *keyValueExtractor[K, V]) processMappingValue(
	val *ast.MappingValueNode,
	valueType reflect.Type, isPtr bool,
	elemType reflect.Type) (K, V, error) {
	var (
		key   K
		value V
	)

	if err := yaml.NodeToValue(val.Key, &key); err != nil {
		return key, value, errors.Wrap(err, "failed to unmarshal key")
	}

	if _, isNull := val.Value.(*ast.NullNode); isNull {
		value = e.createNullValue(isPtr, elemType)

		return key, value, nil
	}

	if err := e.unmarshalValue(val.Value, &value, isPtr, elemType, key); err != nil {
		return key, value, err
	}

	return key, value, nil
}

func (e *keyValueExtractor[K, V]) createNullValue(isPtr bool, elemType reflect.Type) V {
	var value V

	if isPtr && elemType.Kind() == reflect.Struct {
		newPtr := reflect.New(elemType)

		var ok bool

		value, ok = newPtr.Interface().(V)
		if !ok {
			panic(fmt.Sprintf("type assertion failed: expected %T, got %T", value, newPtr.Interface()))
		}
	}

	return value
}

func (e *keyValueExtractor[K, V]) unmarshalValue(node ast.Node, value *V, isPtr bool, elemType reflect.Type, key K) error {
	if !isPtr {
		if err := yaml.NodeToValue(node, value); err != nil {
			return errors.Wrapf(err, "failed to unmarshal %v", key)
		}

		return nil
	}

	if elemType.Kind() == reflect.Struct {
		return e.unmarshalStructPtr(node, value, elemType, key)
	}

	if err := yaml.NodeToValue(node, value); err != nil {
		return errors.Wrapf(err, "failed to unmarshal %v", key)
	}

	return nil
}

func (e *keyValueExtractor[K, V]) unmarshalStructPtr(node ast.Node, value *V, elemType reflect.Type, key K) error {
	newPtr := reflect.New(elemType)
	if err := yaml.NodeToValue(node, newPtr.Interface()); err != nil {
		return errors.Wrapf(err, "failed to unmarshal %v", key)
	}

	v, ok := newPtr.Interface().(V)
	if !ok {
		return errors.Wrapf(ErrTypeAssertionFailed, "expected %T, got %T", value, newPtr.Interface())
	}

	*value = v

	return nil
}
