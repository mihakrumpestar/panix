package config

import (
	"fmt"
	"reflect"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	omap "github.com/kirill-scherba/omap"
)

// OrderedMap is a wrapper around omap.Omap that provides YAML unmarshaling support
// compatible with github.com/goccy/go-yaml.
type OrderedMap[K comparable, V any] struct {
	*omap.Omap[K, V] `validate:"-"`
}

// NewOrderedMap creates a new OrderedMap.
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	m, _ := omap.New[K, V]()
	return &OrderedMap[K, V]{Omap: m}
}

// UnmarshalYAML implements yaml.BytesUnmarshaler interface.
// It unmarshals a YAML mapping into the ordered map while preserving key order.
// Keys without values are stored with the zero value for type V.
func (om *OrderedMap[K, V]) UnmarshalYAML(data []byte) error {
	// Create a new omap if needed
	if om.Omap == nil {
		m, _ := omap.New[K, V]()
		om.Omap = m
	}

	// Clear existing data
	om.Omap.Clear()

	// Parse the YAML to extract keys and values in order
	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
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
	if mapping, ok := node.(*ast.MappingNode); ok {
		valueType := reflect.TypeOf((*V)(nil)).Elem()
		isPtr := valueType.Kind() == reflect.Ptr
		var elemType reflect.Type
		if isPtr {
			elemType = valueType.Elem()
		}

		for _, val := range mapping.Values {
			var key K
			if err := yaml.NodeToValue(val.Key, &key); err != nil {
				e.err = fmt.Errorf("failed to unmarshal key: %w", err)
				return nil
			}

			var value V

			// Check if value is a NullNode (key without value)
			if _, isNull := val.Value.(*ast.NullNode); isNull {
				// Key has no value - for pointer types, create new instance
				if isPtr && elemType.Kind() == reflect.Struct {
					newPtr := reflect.New(elemType)
					value = newPtr.Interface().(V)
				}
				// For non-pointer types, value remains as zero value
			} else {
				// Value exists in YAML, unmarshal it
				if isPtr {
					// For pointer types, allocate a new struct first, then unmarshal into it
					if elemType.Kind() == reflect.Struct {
						newPtr := reflect.New(elemType)
						if err := yaml.NodeToValue(val.Value, newPtr.Interface()); err != nil {
							e.err = fmt.Errorf("failed to unmarshal %v: %w", key, err)
							return nil
						}
						value = newPtr.Interface().(V)
					} else {
						// Pointer to non-struct (e.g., *string, *int)
						if err := yaml.NodeToValue(val.Value, &value); err != nil {
							e.err = fmt.Errorf("failed to unmarshal %v: %w", key, err)
							return nil
						}
					}
				} else {
					// Non-pointer type, unmarshal directly
					if err := yaml.NodeToValue(val.Value, &value); err != nil {
						e.err = fmt.Errorf("failed to unmarshal %v: %w", key, err)
						return nil
					}
				}
			}

			e.om.Omap.Set(key, value)
		}
		// Don't recurse into values
		return nil
	}
	return e
}
