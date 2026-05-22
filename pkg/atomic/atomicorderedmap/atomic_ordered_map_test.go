package atomicorderedmap

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestNew(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	assert.NotNil(t, orderedMap, "New() returned nil")
	assert.Equal(t, 0, orderedMap.Len(), "new map should be empty")
}

func TestSetGetExists(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	val, ok := orderedMap.Get("a")
	assert.True(t, ok, "Get(a) ok")
	assert.Equal(t, 1, val, "Get(a) value")

	val, ok = orderedMap.Get("b")
	assert.True(t, ok, "Get(b) ok")
	assert.Equal(t, 2, val, "Get(b) value")

	val, ok = orderedMap.Get("c")
	assert.True(t, ok, "Get(c) ok")
	assert.Equal(t, 3, val, "Get(c) value")

	_, ok = orderedMap.Get("d")
	assert.False(t, ok, "Get(d) should not be found")

	assert.True(t, orderedMap.Exists("a"), "Exists(a) should be true")
	assert.False(t, orderedMap.Exists("d"), "Exists(d) should be false")
}

func TestSetOverwrite(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	orderedMap.Set("a", 1)
	orderedMap.Set("a", 2)
	orderedMap.Set("a", 3)

	assert.Equal(t, 1, orderedMap.Len(), "Len()")

	val, ok := orderedMap.Get("a")
	assert.True(t, ok, "Get(a) ok")
	assert.Equal(t, 3, val, "Get(a) value after overwrite")

	assert.Len(t, orderedMap.Pairs(), 1)
}

func TestOrderPreservation(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	keys := []string{"first", "second", "third", "fourth"}
	for idx, k := range keys {
		orderedMap.Set(k, idx+1)
	}

	pairs := orderedMap.Pairs()
	require.Len(t, pairs, len(keys), "Pairs() length")

	for idx, pair := range pairs {
		assert.Equal(t, keys[idx], pair.Key, "Pairs()[%d].Key", idx)
		assert.Equal(t, idx+1, pair.Value, "Pairs()[%d].Value", idx)
	}
}

func TestDel(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	orderedMap.Del("b")

	assert.Equal(t, 2, orderedMap.Len(), "Len() after Del")
	assert.False(t, orderedMap.Exists("b"), "Exists(b) after Del")

	pairs := orderedMap.Pairs()
	require.Len(t, pairs, 2, "Pairs() length")

	assert.Equal(t, "a", pairs[0].Key, "Pairs()[0].Key")
	assert.Equal(t, "c", pairs[1].Key, "Pairs()[1].Key")
}

func TestDelNonExistent(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)

	orderedMap.Del("nonexistent")

	assert.Equal(t, 1, orderedMap.Len(), "Len() after Del non-existent")
}

func TestDelIndexReordering(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)
	orderedMap.Set("d", 4)

	orderedMap.Del("b")

	pairs := orderedMap.Pairs()
	require.Len(t, pairs, 3, "Pairs() length")

	expected := []string{"a", "c", "d"}
	for idx, p := range pairs {
		assert.Equal(t, expected[idx], p.Key, "Pairs()[%d].Key", idx)
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	orderedMap.Clear()

	assert.Equal(t, 0, orderedMap.Len(), "Len() after Clear")
	assert.False(t, orderedMap.Exists("a"), "Exists(a) after Clear")
	assert.Empty(t, orderedMap.Pairs(), "Pairs() after Clear")
}

func TestPairs(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)

	pairs := orderedMap.Pairs()
	require.Len(t, pairs, 2, "Pairs() length")

	assert.Equal(t, Pair[string, int]{Key: "a", Value: 1}, pairs[0], "Pairs()[0]")
	assert.Equal(t, Pair[string, int]{Key: "b", Value: 2}, pairs[1], "Pairs()[1]")
}

func TestRecords(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)

	records := orderedMap.Records()
	assert.Len(t, records, 2, "Records() length")
	assert.Equal(t, 1, records["a"], "Records()[a]")
	assert.Equal(t, 2, records["b"], "Records()[b]")
}

func TestLast(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	pair, ok := orderedMap.Last()
	assert.True(t, ok, "Last() ok")
	assert.Equal(t, "c", pair.Key, "Last().Key")
	assert.Equal(t, 3, pair.Value, "Last().Value")
}

func TestLastEmpty(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	pair, ok := orderedMap.Last()
	assert.False(t, ok, "Last() on empty map ok")
	assert.Empty(t, pair.Key, "Last().Key on empty map")
	assert.Equal(t, 0, pair.Value, "Last().Value on empty map")
}

func TestDeleteFunc(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)
	orderedMap.Set("d", 4)

	orderedMap.DeleteFunc(func(k string, v int) bool {
		return v%2 == 0
	})

	assert.Equal(t, 2, orderedMap.Len(), "Len() after DeleteFunc")

	pairs := orderedMap.Pairs()

	expected := []string{"a", "c"}
	for idx, p := range pairs {
		assert.Equal(t, expected[idx], p.Key, "Pairs()[%d].Key", idx)
	}
}

func TestDeleteFuncAll(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)

	orderedMap.DeleteFunc(func(k string, v int) bool {
		return true
	})

	assert.Equal(t, 0, orderedMap.Len(), "Len() after DeleteFunc(all)")
}

func TestNilSafety(t *testing.T) {
	t.Parallel()

	var nilMap *AtomicOrderedMap[string, int]

	assert.Equal(t, 0, nilMap.Len(), "nil.Len()")
	assert.Nil(t, nilMap.Pairs(), "nil.Pairs()")
	assert.Nil(t, nilMap.Records(), "nil.Records()")

	_, ok := nilMap.Last()
	assert.False(t, ok, "nil.Last() ok")

	nilMap.DeleteFunc(func(k string, v int) bool {
		return true
	})
}

func testMarshalUnmarshal(t *testing.T, marshalFunc func(any) ([]byte, error), unmarshalFunc func([]byte, any) error) {
	t.Helper()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	data, err := marshalFunc(orderedMap)
	require.NoError(t, err, "marshal error")

	orderedMap2 := New[string, int]()

	err = unmarshalFunc(data, orderedMap2)
	require.NoError(t, err, "unmarshal error")

	pairs1 := orderedMap.Pairs()
	pairs2 := orderedMap2.Pairs()

	require.Len(t, pairs2, len(pairs1), "unmarshaled length")

	for idx := range pairs1 {
		assert.Equal(t, pairs1[idx], pairs2[idx], "pairs[%d]", idx)
	}
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	testMarshalUnmarshal(t, json.Marshal, json.Unmarshal)
}

func TestJSONUnmarshalNil(t *testing.T) {
	t.Parallel()

	var nilMap *AtomicOrderedMap[string, int]

	data := `[{"key":"a","value":1}]`

	err := json.Unmarshal([]byte(data), nilMap)
	assert.Error(t, err, "UnmarshalJSON on nil should return error")
}

func TestYAMLMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	testMarshalUnmarshal(t, yaml.Marshal, yaml.Unmarshal)
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	var waitGroup sync.WaitGroup

	numWriters := 10
	numReaders := 10
	numOps := 100

	for writerID := range numWriters {
		waitGroup.Add(1)

		go func(wid int) {
			defer waitGroup.Done()

			for idx := range numOps {
				key := "key" + string(rune(wid%26+'a'))
				orderedMap.Set(key, idx)
			}
		}(writerID)
	}

	for range numReaders {
		waitGroup.Go(func() {
			for range numOps {
				_ = orderedMap.Exists("key")
				_, _ = orderedMap.Get("key")
				_ = orderedMap.Pairs()
				_ = orderedMap.Len()
			}
		})
	}

	waitGroup.Wait()
}

func TestDifferentTypes(t *testing.T) {
	t.Parallel()

	t.Run("int keys", func(t *testing.T) {
		t.Parallel()

		orderedMap := New[int, string]()
		orderedMap.Set(1, "one")
		orderedMap.Set(2, "two")

		val, ok := orderedMap.Get(1)
		assert.True(t, ok, "Get(1) ok")
		assert.Equal(t, "one", val, "Get(1) value")
	})

	t.Run("struct values", func(t *testing.T) {
		t.Parallel()

		type testStruct struct {
			Field string
		}

		orderedMap := New[string, testStruct]()
		orderedMap.Set("a", testStruct{Field: "value"})

		val, ok := orderedMap.Get("a")
		assert.True(t, ok, "Get(a) ok")
		assert.Equal(t, "value", val.Field, "Get(a).Field")
	})
}
