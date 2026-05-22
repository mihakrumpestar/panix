package atomicslice

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	assert.NotNil(t, slice, "New() returned nil")
	assert.Equal(t, 0, slice.Length(), "new slice should be empty")
}

func TestNewFrom(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}
	slice := NewFrom(items)

	assert.Equal(t, len(items), slice.Length(), "Length()")

	values := slice.Values()
	for idx, val := range values {
		assert.Equal(t, items[idx], val, "Values()[%d]", idx)
	}
}

func TestNewFromEmpty(t *testing.T) {
	t.Parallel()

	items := []int{}
	slice := NewFrom(items)

	assert.Equal(t, 0, slice.Length(), "Length() for empty slice")
}

func TestAppend(t *testing.T) {
	t.Parallel()

	slice := New[string]()

	slice.Append("a")
	slice.Append("b")
	slice.Append("c")

	assert.Equal(t, 3, slice.Length(), "Length()")

	values := slice.Values()

	expected := []string{"a", "b", "c"}
	for idx, val := range values {
		assert.Equal(t, expected[idx], val, "Values()[%d]", idx)
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(10)
	slice.Append(20)
	slice.Append(30)

	val, ok := slice.Get(0)
	assert.True(t, ok, "Get(0) ok")
	assert.Equal(t, 10, val, "Get(0) value")

	val, ok = slice.Get(1)
	assert.True(t, ok, "Get(1) ok")
	assert.Equal(t, 20, val, "Get(1) value")

	val, ok = slice.Get(2)
	assert.True(t, ok, "Get(2) ok")
	assert.Equal(t, 30, val, "Get(2) value")
}

func TestGetOutOfRange(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)

	_, ok := slice.Get(-1)
	assert.False(t, ok, "Get(-1) ok")

	_, ok = slice.Get(100)
	assert.False(t, ok, "Get(100) ok")
}

func TestLast(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	val, ok := slice.Last()
	assert.True(t, ok, "Last() ok")
	assert.Equal(t, 3, val)
}

func TestLastEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	val, ok := slice.Last()
	assert.False(t, ok, "Last() on empty slice ok")
	assert.Equal(t, 0, val, "Last() on empty slice value")
}

func TestValues(t *testing.T) {
	t.Parallel()

	slice := New[string]()
	slice.Append("x")
	slice.Append("y")
	slice.Append("z")

	values := slice.Values()
	require.Len(t, values, 3, "Values() length")

	expected := []string{"x", "y", "z"}
	for idx, val := range values {
		assert.Equal(t, expected[idx], val, "Values()[%d]", idx)
	}
}

func TestValuesEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	values := slice.Values()

	assert.NotNil(t, values, "Values() for empty slice")
	assert.Empty(t, values)
}

func TestClear(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	slice.Clear()

	assert.Equal(t, 0, slice.Length(), "Length() after Clear")

	values := slice.Values()
	assert.Empty(t, values, "Values() length after Clear")
}

func TestCopy(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	copySlice := slice.Copy()

	assert.Equal(t, slice.Length(), copySlice.Length(), "Copy().Length()")

	originalValues := slice.Values()
	copyValues := copySlice.Values()

	for idx := range originalValues {
		assert.Equal(t, originalValues[idx], copyValues[idx], "Copy().Values()[%d]", idx)
	}
}

func TestCopyIndependence(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)

	copySlice := slice.Copy()
	copySlice.Append(3)

	assert.NotEqual(t, slice.Length(), copySlice.Length(), "modifying copy should not affect original")
	assert.Equal(t, 2, slice.Length(), "original Length()")
	assert.Equal(t, 3, copySlice.Length(), "copy Length()")
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	data, err := json.Marshal(slice)
	require.NoError(t, err, "MarshalJSON() error")

	slice2 := New[int]()

	err = json.Unmarshal(data, slice2)
	require.NoError(t, err, "UnmarshalJSON() error")

	values1 := slice.Values()
	values2 := slice2.Values()

	require.Len(t, values2, len(values1), "unmarshaled length")

	for idx := range values1 {
		assert.Equal(t, values1[idx], values2[idx], "Values()[%d]", idx)
	}
}

func TestJSONUnmarshalEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	data := []byte("[]")

	err := json.Unmarshal(data, slice)
	require.NoError(t, err, "UnmarshalJSON() error")

	assert.Equal(t, 0, slice.Length(), "Length() after unmarshal empty")
}

func TestJSONUnmarshalInvalid(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	data := []byte(`not valid json`)

	err := json.Unmarshal(data, slice)
	assert.Error(t, err, "UnmarshalJSON() should return error for invalid JSON")
}

func TestJSONMarshalEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	data, err := json.Marshal(slice)
	require.NoError(t, err, "MarshalJSON() error")

	assert.Equal(t, "[]", string(data), "MarshalJSON() empty slice")
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	var waitGroup sync.WaitGroup

	numWriters := 10
	numReaders := 10
	numOps := 100

	for writerID := range numWriters {
		waitGroup.Add(1)

		go func(wid int) {
			defer waitGroup.Done()

			for idx := range numOps {
				slice.Append(wid*1000 + idx)
			}
		}(writerID)
	}

	for range numReaders {
		waitGroup.Go(func() {
			for range numOps {
				_ = slice.Length()
				_ = slice.Values()
				_, _ = slice.Get(0)
				_, _ = slice.Last()
			}
		})
	}

	waitGroup.Wait()

	expectedLength := numWriters * numOps
	assert.Equal(t, expectedLength, slice.Length(), "Length() after concurrent access")
}

func TestDifferentTypes(t *testing.T) {
	t.Parallel()

	t.Run("strings", func(t *testing.T) {
		t.Parallel()

		slice := New[string]()
		slice.Append("hello")
		slice.Append("world")

		val, ok := slice.Get(0)
		assert.True(t, ok, "Get(0) ok")
		assert.Equal(t, "hello", val, "Get(0) value")
	})

	t.Run("structs", func(t *testing.T) {
		t.Parallel()

		type testStruct struct {
			Name  string
			Value int
		}

		slice := New[testStruct]()
		slice.Append(testStruct{Name: "first", Value: 1})
		slice.Append(testStruct{Name: "second", Value: 2})

		val, ok := slice.Get(1)
		assert.True(t, ok, "Get(1) ok")
		assert.Equal(t, testStruct{Name: "second", Value: 2}, val, "Get(1) value")
	})

	t.Run("pointers", func(t *testing.T) {
		t.Parallel()

		slice := New[*int]()
		val1, val2 := 10, 20

		slice.Append(&val1)
		slice.Append(&val2)

		ptr, ok := slice.Get(0)
		assert.True(t, ok, "Get(0) ok")
		assert.NotNil(t, ptr, "Get(0) ptr")
		assert.Equal(t, 10, *ptr, "Get(0) value")
	})
}

func TestSet(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	slice.Set(1, 20)

	val, ok := slice.Get(1)
	assert.True(t, ok, "Get(1) ok after Set")
	assert.Equal(t, 20, val, "Get(1) value after Set")
}

func TestSetOutOfRange(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)

	slice.Set(100, 999)

	_, ok := slice.Get(100)
	assert.False(t, ok, "Get(100) after Set at invalid index")
}

func TestRemove(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	slice.Remove(1)

	assert.Equal(t, 2, slice.Length(), "Length() after Remove")

	values := slice.Values()

	expected := []int{1, 3}
	for idx, val := range values {
		assert.Equal(t, expected[idx], val, "Values()[%d]", idx)
	}
}

func TestRemoveLast(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)

	slice.Remove(1)

	assert.Equal(t, 1, slice.Length(), "Length()")

	val, ok := slice.Last()
	assert.True(t, ok, "Last() ok")
	assert.Equal(t, 1, val, "Last() value")
}

func TestRemoveOutOfRange(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)

	slice.Remove(100)

	assert.Equal(t, 1, slice.Length(), "Length() after invalid Remove")
}

func TestInsert(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(3)

	slice.Insert(1, 2)

	require.Equal(t, 3, slice.Length(), "Length()")

	values := slice.Values()

	expected := []int{1, 2, 3}
	for idx, val := range values {
		assert.Equal(t, expected[idx], val, "Values()[%d]", idx)
	}
}

func TestContains(t *testing.T) {
	t.Parallel()

	slice := New[string]()
	slice.Append("apple")
	slice.Append("banana")
	slice.Append("cherry")

	assert.True(t, slice.Contains("banana"), "Contains(banana)")
	assert.False(t, slice.Contains("grape"), "Contains(grape)")
}

func TestContainsEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	assert.False(t, slice.Contains(1), "Contains on empty slice")
}

func TestLength(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	assert.Equal(t, 0, slice.Length(), "Length() after New")
	slice.Append(1)
	assert.Equal(t, 1, slice.Length(), "Length() after first Append")
	slice.Append(2)
	assert.Equal(t, 2, slice.Length(), "Length() after second Append")
	slice.Remove(0)
	assert.Equal(t, 1, slice.Length(), "Length() after Remove")
	slice.Clear()
	assert.Equal(t, 0, slice.Length(), "Length() after Clear")
}
