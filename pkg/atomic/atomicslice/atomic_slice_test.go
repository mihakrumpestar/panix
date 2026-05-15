package atomicslice

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	if slice == nil {
		t.Fatal("New() returned nil")
	}

	if slice.Length() != 0 {
		t.Errorf("new slice should be empty, got length %d", slice.Length())
	}
}

func TestNewFrom(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}
	slice := NewFrom(items)

	if slice.Length() != len(items) {
		t.Errorf("Length() = %d, want %d", slice.Length(), len(items))
	}

	values := slice.Values()
	for idx, val := range values {
		if val != items[idx] {
			t.Errorf("Values()[%d] = %d, want %d", idx, val, items[idx])
		}
	}
}

func TestNewFromEmpty(t *testing.T) {
	t.Parallel()

	items := []int{}
	slice := NewFrom(items)

	if slice.Length() != 0 {
		t.Errorf("Length() = %d, want 0", slice.Length())
	}
}

func TestAppend(t *testing.T) {
	t.Parallel()

	slice := New[string]()

	slice.Append("a")
	slice.Append("b")
	slice.Append("c")

	if slice.Length() != 3 {
		t.Errorf("Length() = %d, want 3", slice.Length())
	}

	values := slice.Values()

	expected := []string{"a", "b", "c"}
	for idx, val := range values {
		if val != expected[idx] {
			t.Errorf("Values()[%d] = %q, want %q", idx, val, expected[idx])
		}
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(10)
	slice.Append(20)
	slice.Append(30)

	if val, ok := slice.Get(0); !ok || val != 10 {
		t.Errorf("Get(0) = (%d, %v), want (10, true)", val, ok)
	}

	if val, ok := slice.Get(1); !ok || val != 20 {
		t.Errorf("Get(1) = (%d, %v), want (20, true)", val, ok)
	}

	if val, ok := slice.Get(2); !ok || val != 30 {
		t.Errorf("Get(2) = (%d, %v), want (30, true)", val, ok)
	}
}

func TestGetOutOfRange(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)

	if val, ok := slice.Get(-1); ok {
		t.Errorf("Get(-1) = (%d, %v), want (_, false)", val, ok)
	}

	if val, ok := slice.Get(100); ok {
		t.Errorf("Get(100) = (%d, %v), want (_, false)", val, ok)
	}
}

func TestLast(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	val, ok := slice.Last()
	if !ok {
		t.Fatal("Last() returned ok=false, want true")
	}

	if val != 3 {
		t.Errorf("Last() = %d, want 3", val)
	}
}

func TestLastEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	val, ok := slice.Last()
	if ok {
		t.Errorf("Last() on empty slice returned ok=true, want false")
	}

	if val != 0 {
		t.Errorf("Last() = %d, want zero value 0", val)
	}
}

func TestValues(t *testing.T) {
	t.Parallel()

	slice := New[string]()
	slice.Append("x")
	slice.Append("y")
	slice.Append("z")

	values := slice.Values()
	if len(values) != 3 {
		t.Fatalf("Values() length = %d, want 3", len(values))
	}

	expected := []string{"x", "y", "z"}
	for idx, val := range values {
		if val != expected[idx] {
			t.Errorf("Values()[%d] = %q, want %q", idx, val, expected[idx])
		}
	}
}

func TestValuesEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	values := slice.Values()

	if values == nil {
		t.Error("Values() returned nil for empty slice")
	}

	if len(values) != 0 {
		t.Errorf("Values() length = %d, want 0", len(values))
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	slice.Clear()

	if slice.Length() != 0 {
		t.Errorf("Length() after Clear = %d, want 0", slice.Length())
	}

	values := slice.Values()
	if len(values) != 0 {
		t.Errorf("Values() length after Clear = %d, want 0", len(values))
	}
}

func TestCopy(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	copySlice := slice.Copy()

	if copySlice.Length() != slice.Length() {
		t.Errorf("Copy().Length() = %d, want %d", copySlice.Length(), slice.Length())
	}

	originalValues := slice.Values()
	copyValues := copySlice.Values()

	for idx := range originalValues {
		if copyValues[idx] != originalValues[idx] {
			t.Errorf("Copy().Values()[%d] = %d, want %d", idx, copyValues[idx], originalValues[idx])
		}
	}
}

func TestCopyIndependence(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)

	copySlice := slice.Copy()
	copySlice.Append(3)

	if slice.Length() == copySlice.Length() {
		t.Error("modifying copy should not affect original")
	}

	if slice.Length() != 2 {
		t.Errorf("original Length() = %d, want 2", slice.Length())
	}

	if copySlice.Length() != 3 {
		t.Errorf("copy Length() = %d, want 3", copySlice.Length())
	}
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	data, err := json.Marshal(slice)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	slice2 := New[int]()

	err = json.Unmarshal(data, slice2)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	values1 := slice.Values()
	values2 := slice2.Values()

	if len(values1) != len(values2) {
		t.Fatalf("unmarshaled length = %d, want %d", len(values2), len(values1))
	}

	for idx := range values1 {
		if values1[idx] != values2[idx] {
			t.Errorf("Values()[%d] = %d, want %d", idx, values2[idx], values1[idx])
		}
	}
}

func TestJSONUnmarshalEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	data := []byte("[]")

	err := json.Unmarshal(data, slice)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	if slice.Length() != 0 {
		t.Errorf("Length() = %d, want 0", slice.Length())
	}
}

func TestJSONUnmarshalInvalid(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	data := []byte(`not valid json`)

	err := json.Unmarshal(data, slice)
	if err == nil {
		t.Error("UnmarshalJSON() should return error for invalid JSON")
	}
}

func TestJSONMarshalEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	data, err := json.Marshal(slice)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	if string(data) != "[]" {
		t.Errorf("MarshalJSON() = %s, want []", string(data))
	}
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
	if slice.Length() != expectedLength {
		t.Errorf("Length() = %d, want %d", slice.Length(), expectedLength)
	}
}

func TestDifferentTypes(t *testing.T) {
	t.Parallel()

	t.Run("strings", func(t *testing.T) {
		t.Parallel()

		slice := New[string]()
		slice.Append("hello")
		slice.Append("world")

		if val, ok := slice.Get(0); !ok || val != "hello" {
			t.Errorf("Get(0) = (%q, %v), want (hello, true)", val, ok)
		}
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
		if !ok || val.Name != "second" || val.Value != 2 {
			t.Errorf("Get(1) = (%v, %v), want ({second 2}, true)", val, ok)
		}
	})

	t.Run("pointers", func(t *testing.T) {
		t.Parallel()

		slice := New[*int]()
		val1, val2 := 10, 20

		slice.Append(&val1)
		slice.Append(&val2)

		ptr, ok := slice.Get(0)
		if !ok || ptr == nil || *ptr != 10 {
			t.Errorf("Get(0) = (%v, %v), want (&10, true)", ptr, ok)
		}
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
	if !ok || val != 20 {
		t.Errorf("Get(1) after Set = (%d, %v), want (20, true)", val, ok)
	}
}

func TestSetOutOfRange(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)

	slice.Set(100, 999)

	val, ok := slice.Get(100)
	if ok {
		t.Errorf("Set at invalid index should not create element, got (%d, %v)", val, ok)
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)
	slice.Append(3)

	slice.Remove(1)

	if slice.Length() != 2 {
		t.Errorf("Length() after Remove = %d, want 2", slice.Length())
	}

	values := slice.Values()

	expected := []int{1, 3}
	for idx, val := range values {
		if val != expected[idx] {
			t.Errorf("Values()[%d] = %d, want %d", idx, val, expected[idx])
		}
	}
}

func TestRemoveLast(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(2)

	slice.Remove(1)

	if slice.Length() != 1 {
		t.Errorf("Length() = %d, want 1", slice.Length())
	}

	val, ok := slice.Last()
	if !ok || val != 1 {
		t.Errorf("Last() = (%d, %v), want (1, true)", val, ok)
	}
}

func TestRemoveOutOfRange(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)

	slice.Remove(100)

	if slice.Length() != 1 {
		t.Errorf("Length() after invalid Remove = %d, want 1", slice.Length())
	}
}

func TestInsert(t *testing.T) {
	t.Parallel()

	slice := New[int]()
	slice.Append(1)
	slice.Append(3)

	slice.Insert(1, 2)

	if slice.Length() != 3 {
		t.Fatalf("Length() = %d, want 3", slice.Length())
	}

	values := slice.Values()

	expected := []int{1, 2, 3}
	for idx, val := range values {
		if val != expected[idx] {
			t.Errorf("Values()[%d] = %d, want %d", idx, val, expected[idx])
		}
	}
}

func TestContains(t *testing.T) {
	t.Parallel()

	slice := New[string]()
	slice.Append("apple")
	slice.Append("banana")
	slice.Append("cherry")

	if !slice.Contains("banana") {
		t.Error("Contains(banana) = false, want true")
	}

	if slice.Contains("grape") {
		t.Error("Contains(grape) = true, want false")
	}
}

func TestContainsEmpty(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	if slice.Contains(1) {
		t.Error("Contains on empty slice should return false")
	}
}

func TestLength(t *testing.T) {
	t.Parallel()

	slice := New[int]()

	if slice.Length() != 0 {
		t.Errorf("Length() = %d, want 0", slice.Length())
	}

	slice.Append(1)

	if slice.Length() != 1 {
		t.Errorf("Length() = %d, want 1", slice.Length())
	}

	slice.Append(2)

	if slice.Length() != 2 {
		t.Errorf("Length() = %d, want 2", slice.Length())
	}

	slice.Remove(0)

	if slice.Length() != 1 {
		t.Errorf("Length() = %d, want 1", slice.Length())
	}

	slice.Clear()

	if slice.Length() != 0 {
		t.Errorf("Length() = %d, want 0", slice.Length())
	}
}
