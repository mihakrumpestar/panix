package orderedmap

import (
	"encoding/json"
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Inner struct {
	Name  string `yaml:"name"`
	Value int    `yaml:"value"`
}

func TestUnmarshalPointerValueNil(t *testing.T) {
	yml := `
first:
  name: alpha
  value: 1
second:
  name: beta
  value: 2
`
	var m OrderedMap[string, *Inner]
	err := yaml.Unmarshal([]byte(yml), &m)
	require.NoError(t, err)

	pairs := m.Pairs()
	require.Len(t, pairs, 2)

	assert.NotNil(t, pairs[0].Value, "first value should not be nil")
	assert.NotNil(t, pairs[1].Value, "second value should not be nil")

	if pairs[0].Value != nil {
		assert.Equal(t, "alpha", pairs[0].Value.Name)
		assert.Equal(t, 1, pairs[0].Value.Value)
	}

	if pairs[1].Value != nil {
		assert.Equal(t, "beta", pairs[1].Value.Name)
		assert.Equal(t, 2, pairs[1].Value.Value)
	}
}

func TestUnmarshalPointerValueEmpty(t *testing.T) {
	yml := `
first:
  name: alpha
  value: 0
second:
  name: beta
  value: 0
`
	var m OrderedMap[string, *Inner]
	err := yaml.Unmarshal([]byte(yml), &m)
	require.NoError(t, err)

	pairs := m.Pairs()
	require.Len(t, pairs, 2)

	assert.NotNil(t, pairs[0].Value, "first value should not be nil")
	assert.NotNil(t, pairs[1].Value, "second value should not be nil")
}

type Outer struct {
	Items OrderedMap[string, *Inner] `yaml:"items"`
}

func TestUnmarshalStructWithOrderedMapPointerValues(t *testing.T) {
	yml := `
items:
  first:
    name: alpha
    value: 1
  second:
    name: beta
    value: 2
`
	var o Outer
	err := yaml.Unmarshal([]byte(yml), &o)
	require.NoError(t, err)

	pairs := o.Items.Pairs()
	require.Len(t, pairs, 2)

	assert.NotNil(t, pairs[0].Value, "first value should not be nil")
	assert.NotNil(t, pairs[1].Value, "second value should not be nil")

	if pairs[0].Value != nil {
		assert.Equal(t, "alpha", pairs[0].Value.Name)
	}

	if pairs[1].Value != nil {
		assert.Equal(t, "beta", pairs[1].Value.Name)
	}
}

func TestUnmarshalPointerValueNilYAML(t *testing.T) {
	yml := `
first:
second:
  name: beta
  value: 2
`
	var m OrderedMap[string, *Inner]
	err := yaml.Unmarshal([]byte(yml), &m)

	t.Logf("error: %v", err)

	if err == nil {
		pairs := m.Pairs()
		for _, p := range pairs {
			t.Logf("key=%s value=%v nil=%v", p.Key, p.Value, p.Value == nil)
		}
	}
}

func TestUnmarshalNilPointerMethodCall(t *testing.T) {
	yml := `
first:
  name: alpha
  value: 1
`
	var m OrderedMap[string, *Inner]
	err := yaml.Unmarshal([]byte(yml), &m)
	require.NoError(t, err)

	pairs := m.Pairs()
	require.Len(t, pairs, 1)

	inner := pairs[0].Value
	require.NotNil(t, inner, "value must not be nil to call methods on it")

	assert.Equal(t, "alpha", inner.Name)
	assert.Equal(t, 1, inner.Value)
}

func TestUnmarshalPointerValueNestedStruct(t *testing.T) {
	type DeepNested struct {
		Field string `yaml:"field"`
	}

	type Nested struct {
		Deep DeepNested `yaml:"deep"`
		Num  int        `yaml:"num"`
	}

	yml := `
a:
  deep:
    field: hello
  num: 42
b:
  deep:
    field: world
  num: 99
`
	var m OrderedMap[string, *Nested]
	err := yaml.Unmarshal([]byte(yml), &m)
	require.NoError(t, err)

	pairs := m.Pairs()
	require.Len(t, pairs, 2)

	assert.NotNil(t, pairs[0].Value, "first value should not be nil")
	assert.NotNil(t, pairs[1].Value, "second value should not be nil")

	if pairs[0].Value != nil {
		assert.Equal(t, "hello", pairs[0].Value.Deep.Field)
		assert.Equal(t, 42, pairs[0].Value.Num)
	}

	if pairs[1].Value != nil {
		assert.Equal(t, "world", pairs[1].Value.Deep.Field)
		assert.Equal(t, 99, pairs[1].Value.Num)
	}
}

func TestJSONUnmarshalPointerValue(t *testing.T) {
	input := `[{"key":"first","value":{"name":"alpha","value":1}},{"key":"second","value":{"name":"beta","value":2}}]`

	var m OrderedMap[string, *Inner]
	err := json.Unmarshal([]byte(input), &m)
	require.NoError(t, err)

	pairs := m.Pairs()
	require.Len(t, pairs, 2)

	assert.NotNil(t, pairs[0].Value, "first value should not be nil")
	assert.NotNil(t, pairs[1].Value, "second value should not be nil")

	if pairs[0].Value != nil {
		assert.Equal(t, "alpha", pairs[0].Value.Name)
		assert.Equal(t, 1, pairs[0].Value.Value)
	}

	if pairs[1].Value != nil {
		assert.Equal(t, "beta", pairs[1].Value.Name)
		assert.Equal(t, 2, pairs[1].Value.Value)
	}
}
