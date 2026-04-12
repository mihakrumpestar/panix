package orderedmap

import (
	"encoding/json"
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLUnmarshalOrdered(t *testing.T) {
	yml := `
name: alice
age: "30"
city: berlin
`
	var m OrderedMap[string, string]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	var keys []string
	m.Range(func(k string, v string) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []string{"name", "age", "city"}, keys)
}

func TestYAMLUnmarshalDuplicateKeys(t *testing.T) {
	yml := `
a: 1
a: 2
`
	var m OrderedMap[string, int]
	err := yaml.Unmarshal([]byte(yml), &m)
	assert.Error(t, err)
}

func TestYAMLUnmarshalNonStringKey(t *testing.T) {
	yml := `
1: value
`
	var m OrderedMap[string, string]
	err := yaml.Unmarshal([]byte(yml), &m)
	assert.Error(t, err)
}

func TestGet(t *testing.T) {
	yml := `
x: hello
y: world
`
	var m OrderedMap[string, string]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	v, ok := m.Get("x")
	assert.True(t, ok)
	assert.Equal(t, "hello", v)

	v, ok = m.Get("y")
	assert.True(t, ok)
	assert.Equal(t, "world", v)

	_, ok = m.Get("z")
	assert.False(t, ok)
}

func TestDelete(t *testing.T) {
	yml := `
a: 1
b: 2
c: 3
`
	var m OrderedMap[string, int]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	m.Delete("b")

	var keys []string
	m.Range(func(k string, v int) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []string{"a", "c"}, keys)

	_, ok := m.Get("b")
	assert.False(t, ok)
}

func TestDeleteNonExistent(t *testing.T) {
	m := New[string, int]()
	m.Delete("nonexistent")
	assert.Equal(t, 0, m.Len())
}

func TestSetNew(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	assert.Equal(t, 3, m.Len())

	v, ok := m.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 2, v)

	var keys []string
	m.Range(func(k string, v int) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []string{"a", "b", "c"}, keys)
}

func TestSetExisting(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("a", 10)

	assert.Equal(t, 2, m.Len())

	v, ok := m.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 10, v)

	var keys []string
	m.Range(func(k string, v int) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []string{"a", "b"}, keys)
}

func TestSetOnUnmarshaled(t *testing.T) {
	yml := `
x: hello
y: world
`
	var m OrderedMap[string, string]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	m.Set("z", "!")
	m.Set("x", "updated")

	assert.Equal(t, 3, m.Len())

	v, ok := m.Get("x")
	assert.True(t, ok)
	assert.Equal(t, "updated", v)

	v, ok = m.Get("z")
	assert.True(t, ok)
	assert.Equal(t, "!", v)

	var keys []string
	m.Range(func(k string, v string) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []string{"x", "y", "z"}, keys)
}

func TestClear(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	require.Equal(t, 2, m.Len())

	m.Clear()
	assert.Equal(t, 0, m.Len())

	_, ok := m.Get("a")
	assert.False(t, ok)

	m.Set("c", 3)
	assert.Equal(t, 1, m.Len())
	v, ok := m.Get("c")
	assert.True(t, ok)
	assert.Equal(t, 3, v)
}

func TestLast(t *testing.T) {
	m := New[string, int]()
	_, ok := m.Last()
	assert.False(t, ok)

	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	p, ok := m.Last()
	assert.True(t, ok)
	assert.Equal(t, Pair[string, int]{Key: "c", Value: 3}, p)
}

func TestJSONMarshal(t *testing.T) {
	yml := `
first: alpha
second: beta
`
	var m OrderedMap[string, string]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	data, err := json.Marshal(&m)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"key":"first","value":"alpha"},{"key":"second","value":"beta"}]`, string(data))
}

func TestJSONUnmarshal(t *testing.T) {
	input := `[{"key":"foo","value":"bar"},{"key":"baz","value":"qux"}]`

	var m OrderedMap[string, string]
	require.NoError(t, json.Unmarshal([]byte(input), &m))

	v, ok := m.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", v)

	v, ok = m.Get("baz")
	assert.True(t, ok)
	assert.Equal(t, "qux", v)
}

func TestJSONRoundTrip(t *testing.T) {
	yml := `
x: hello
y: world
`
	var m OrderedMap[string, string]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	data, err := json.Marshal(&m)
	require.NoError(t, err)

	var m2 OrderedMap[string, string]
	require.NoError(t, json.Unmarshal(data, &m2))

	var keys1, keys2 []string
	m.Range(func(k string, v string) bool { keys1 = append(keys1, k); return true })
	m2.Range(func(k string, v string) bool { keys2 = append(keys2, k); return true })
	assert.Equal(t, keys1, keys2)
}

func TestComplexValue(t *testing.T) {
	type Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	yml := `
web:
  host: localhost
  port: 8080
db:
  host: db.local
  port: 5432
`
	var m OrderedMap[string, Server]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	s, ok := m.Get("web")
	assert.True(t, ok)
	assert.Equal(t, Server{Host: "localhost", Port: 8080}, s)

	s, ok = m.Get("db")
	assert.True(t, ok)
	assert.Equal(t, Server{Host: "db.local", Port: 5432}, s)
}

func TestNewAndLen(t *testing.T) {
	m := New[string, int]()
	assert.Equal(t, 0, m.Len())

	yml := `
a: 1
b: 2
`
	var m2 OrderedMap[string, int]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m2))
	assert.Equal(t, 2, m2.Len())
}

func TestPairs(t *testing.T) {
	yml := `
x: one
y: two
`
	var m OrderedMap[string, string]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	pairs := m.Pairs()
	assert.Equal(t, []Pair[string, string]{
		{Key: "x", Value: "one"},
		{Key: "y", Value: "two"},
	}, pairs)
}

func TestDeleteFunc(t *testing.T) {
	yml := `
a: one
b: two
c: three
d: four
`
	var m OrderedMap[string, string]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	m.DeleteFunc(func(k string, v string) bool {
		return k == "a" || k == "c"
	})

	var keys []string
	m.Range(func(k string, v string) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []string{"b", "d"}, keys)

	_, ok := m.Get("a")
	assert.False(t, ok)
	_, ok = m.Get("b")
	assert.True(t, ok)
}

func TestRangeStopEarly(t *testing.T) {
	yml := `
a: 1
b: 2
c: 3
`
	var m OrderedMap[string, int]
	require.NoError(t, yaml.Unmarshal([]byte(yml), &m))

	var visited []string
	m.Range(func(k string, v int) bool {
		visited = append(visited, k)
		return k != "b"
	})
	assert.Equal(t, []string{"a", "b"}, visited)
}

func TestIntKeyJSON(t *testing.T) {
	input := `[{"key":1,"value":"a"},{"key":2,"value":"b"}]`

	var m OrderedMap[int, string]
	require.NoError(t, json.Unmarshal([]byte(input), &m))

	v, ok := m.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "a", v)

	v, ok = m.Get(2)
	assert.True(t, ok)
	assert.Equal(t, "b", v)

	data, err := json.Marshal(&m)
	require.NoError(t, err)
	assert.JSONEq(t, input, string(data))
}
