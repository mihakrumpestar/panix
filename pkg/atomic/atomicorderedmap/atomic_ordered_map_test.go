package atomicorderedmap

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestNew(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	if orderedMap == nil {
		t.Fatal("New() returned nil")
	}

	if orderedMap.Len() != 0 {
		t.Errorf("new map should be empty, got length %d", orderedMap.Len())
	}
}

func TestSetGetExists(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	if val, ok := orderedMap.Get("a"); !ok || val != 1 {
		t.Errorf("Get(a) = (%d, %v), want (1, true)", val, ok)
	}

	if val, ok := orderedMap.Get("b"); !ok || val != 2 {
		t.Errorf("Get(b) = (%d, %v), want (2, true)", val, ok)
	}

	if val, ok := orderedMap.Get("c"); !ok || val != 3 {
		t.Errorf("Get(c) = (%d, %v), want (3, true)", val, ok)
	}

	if val, ok := orderedMap.Get("d"); ok {
		t.Errorf("Get(d) = (%d, %v), want (_, false)", val, ok)
	}

	if !orderedMap.Exists("a") {
		t.Error("Exists(a) = false, want true")
	}

	if orderedMap.Exists("d") {
		t.Error("Exists(d) = true, want false")
	}
}

func TestSetOverwrite(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	orderedMap.Set("a", 1)
	orderedMap.Set("a", 2)
	orderedMap.Set("a", 3)

	if orderedMap.Len() != 1 {
		t.Errorf("Len() = %d, want 1", orderedMap.Len())
	}

	if val, ok := orderedMap.Get("a"); !ok || val != 3 {
		t.Errorf("Get(a) = (%d, %v), want (3, true)", val, ok)
	}

	pairs := orderedMap.Pairs()
	if len(pairs) != 1 {
		t.Errorf("Pairs() length = %d, want 1", len(pairs))
	}
}

func TestOrderPreservation(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	keys := []string{"first", "second", "third", "fourth"}
	for idx, k := range keys {
		orderedMap.Set(k, idx+1)
	}

	pairs := orderedMap.Pairs()
	if len(pairs) != len(keys) {
		t.Fatalf("Pairs() length = %d, want %d", len(pairs), len(keys))
	}

	for idx, pair := range pairs {
		if pair.Key != keys[idx] {
			t.Errorf("Pairs()[%d].Key = %q, want %q", idx, pair.Key, keys[idx])
		}

		if pair.Value != idx+1 {
			t.Errorf("Pairs()[%d].Value = %d, want %d", idx, pair.Value, idx+1)
		}
	}
}

func TestDel(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	orderedMap.Del("b")

	if orderedMap.Len() != 2 {
		t.Errorf("Len() = %d, want 2", orderedMap.Len())
	}

	if orderedMap.Exists("b") {
		t.Error("Exists(b) = true, want false")
	}

	pairs := orderedMap.Pairs()
	if len(pairs) != 2 {
		t.Fatalf("Pairs() length = %d, want 2", len(pairs))
	}

	if pairs[0].Key != "a" || pairs[1].Key != "c" {
		t.Errorf("order not preserved after Del: got keys %v, want [a c]", []string{pairs[0].Key, pairs[1].Key})
	}
}

func TestDelNonExistent(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)

	orderedMap.Del("nonexistent")

	if orderedMap.Len() != 1 {
		t.Errorf("Len() = %d, want 1", orderedMap.Len())
	}
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
	if len(pairs) != 3 {
		t.Fatalf("Pairs() length = %d, want 3", len(pairs))
	}

	expected := []string{"a", "c", "d"}
	for idx, p := range pairs {
		if p.Key != expected[idx] {
			t.Errorf("Pairs()[%d].Key = %q, want %q", idx, p.Key, expected[idx])
		}
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	orderedMap.Clear()

	if orderedMap.Len() != 0 {
		t.Errorf("Len() = %d, want 0", orderedMap.Len())
	}

	if orderedMap.Exists("a") {
		t.Error("Exists(a) = true after Clear")
	}

	pairs := orderedMap.Pairs()
	if len(pairs) != 0 {
		t.Errorf("Pairs() length = %d, want 0", len(pairs))
	}
}

func TestPairs(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)

	pairs := orderedMap.Pairs()
	if len(pairs) != 2 {
		t.Fatalf("Pairs() length = %d, want 2", len(pairs))
	}

	if pairs[0] != (Pair[string, int]{Key: "a", Value: 1}) {
		t.Errorf("Pairs()[0] = %v, want {a 1}", pairs[0])
	}

	if pairs[1] != (Pair[string, int]{Key: "b", Value: 2}) {
		t.Errorf("Pairs()[1] = %v, want {b 2}", pairs[1])
	}
}

func TestRecords(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)

	records := orderedMap.Records()
	if len(records) != 2 {
		t.Errorf("Records() length = %d, want 2", len(records))
	}

	if records["a"] != 1 {
		t.Errorf("Records()[a] = %d, want 1", records["a"])
	}

	if records["b"] != 2 {
		t.Errorf("Records()[b] = %d, want 2", records["b"])
	}
}

func TestLast(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	pair, ok := orderedMap.Last()
	if !ok {
		t.Fatal("Last() returned ok=false, want true")
	}

	if pair.Key != "c" || pair.Value != 3 {
		t.Errorf("Last() = %v, want {c 3}", pair)
	}
}

func TestLastEmpty(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()

	pair, ok := orderedMap.Last()
	if ok {
		t.Errorf("Last() on empty map returned ok=true, want false")
	}

	if pair.Key != "" || pair.Value != 0 {
		t.Errorf("Last() = %v, want zero value", pair)
	}
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

	if orderedMap.Len() != 2 {
		t.Errorf("Len() = %d, want 2", orderedMap.Len())
	}

	pairs := orderedMap.Pairs()

	expected := []string{"a", "c"}
	for idx, p := range pairs {
		if p.Key != expected[idx] {
			t.Errorf("Pairs()[%d].Key = %q, want %q", idx, p.Key, expected[idx])
		}
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

	if orderedMap.Len() != 0 {
		t.Errorf("Len() = %d, want 0", orderedMap.Len())
	}
}

func TestNilSafety(t *testing.T) {
	t.Parallel()

	var nilMap *AtomicOrderedMap[string, int]

	if nilMap.Len() != 0 {
		t.Error("nil.Len() should return 0")
	}

	if pairs := nilMap.Pairs(); pairs != nil {
		t.Errorf("nil.Pairs() = %v, want nil", pairs)
	}

	if records := nilMap.Records(); records != nil {
		t.Errorf("nil.Records() = %v, want nil", records)
	}

	if pair, ok := nilMap.Last(); ok {
		t.Errorf("nil.Last() = (%v, %v), want (zero, false)", pair, ok)
	}

	nilMap.DeleteFunc(func(k string, v int) bool {
		return true
	})
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	data, err := json.Marshal(orderedMap)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	orderedMap2 := New[string, int]()

	err = json.Unmarshal(data, orderedMap2)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	pairs1 := orderedMap.Pairs()
	pairs2 := orderedMap2.Pairs()

	if len(pairs1) != len(pairs2) {
		t.Fatalf("unmarshaled length = %d, want %d", len(pairs2), len(pairs1))
	}

	for idx := range pairs1 {
		if pairs1[idx] != pairs2[idx] {
			t.Errorf("pairs[%d] = %v, want %v", idx, pairs2[idx], pairs1[idx])
		}
	}
}

func TestJSONUnmarshalNil(t *testing.T) {
	t.Parallel()

	var nilMap *AtomicOrderedMap[string, int]

	data := `[{"key":"a","value":1}]`

	err := json.Unmarshal([]byte(data), nilMap)
	if err == nil {
		t.Error("UnmarshalJSON on nil should return error")
	}
}

func TestYAMLMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	orderedMap := New[string, int]()
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	orderedMap.Set("c", 3)

	data, err := yaml.Marshal(orderedMap)
	if err != nil {
		t.Fatalf("MarshalYAML() error: %v", err)
	}

	orderedMap2 := New[string, int]()

	err = yaml.Unmarshal(data, orderedMap2)
	if err != nil {
		t.Fatalf("UnmarshalYAML() error: %v", err)
	}

	pairs1 := orderedMap.Pairs()
	pairs2 := orderedMap2.Pairs()

	if len(pairs1) != len(pairs2) {
		t.Fatalf("unmarshaled length = %d, want %d", len(pairs2), len(pairs1))
	}

	for idx := range pairs1 {
		if pairs1[idx] != pairs2[idx] {
			t.Errorf("pairs[%d] = %v, want %v", idx, pairs2[idx], pairs1[idx])
		}
	}
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

		if val, ok := orderedMap.Get(1); !ok || val != "one" {
			t.Errorf("Get(1) = (%q, %v), want (one, true)", val, ok)
		}
	})

	t.Run("struct values", func(t *testing.T) {
		t.Parallel()

		type testStruct struct {
			Field string
		}

		orderedMap := New[string, testStruct]()
		orderedMap.Set("a", testStruct{Field: "value"})

		val, ok := orderedMap.Get("a")
		if !ok || val.Field != "value" {
			t.Errorf("Get(a) = (%v, %v), want ({value}, true)", val, ok)
		}
	})
}
