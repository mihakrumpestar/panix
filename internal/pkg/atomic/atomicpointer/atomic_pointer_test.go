package atomicpointer

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string
	Value int
}

func TestNew(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	must := require.New(t)

	ptr := New[int]()
	must.NotNil(ptr)

	val := ptr.Load()
	must.NotNil(val)
	assertion.Equal(0, *val)
}

func TestNewWithString(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	must := require.New(t)

	ptr := New[string]()
	val := ptr.Load()
	must.NotNil(val)
	assertion.Empty(*val)
}

func TestNewWithStruct(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	must := require.New(t)

	ptr := New[testStruct]()
	val := ptr.Load()
	must.NotNil(val)
	assertion.Empty(val.Name)
	assertion.Equal(0, val.Value)
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	ptr := New[int]()
	ptr.Update(func(val *int) { *val = 42 })
	assertion.Equal(42, *ptr.Load())

	ptr.Update(func(val *int) { *val = 100 })
	assertion.Equal(100, *ptr.Load())
}

func TestUpdateStruct(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	ptr := New[testStruct]()
	ptr.Update(func(val *testStruct) {
		val.Name = "test"
		val.Value = 99
	})
	val := ptr.Load()
	assertion.Equal("test", val.Name)
	assertion.Equal(99, val.Value)
}

func TestUpdateConcurrent(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	ptr := New[int]()
	waitGroup := sync.WaitGroup{}

	for i := range 1000 {
		waitGroup.Go(func() {
			ptr.Update(func(val *int) { *val++ })
		})

		_ = i
	}

	waitGroup.Wait()
	assertion.Equal(1000, *ptr.Load())
}

func TestUpdatePanicsOnNilPointer(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	ptr := New[int]()
	ptr.Pointer.Store(nil)

	assertion.Panics(func() {
		ptr.Update(func(val *int) { *val = 1 })
	})
}

func TestClear(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	ptr := New[int]()
	ptr.Update(func(val *int) { *val = 99 })
	assertion.Equal(99, *ptr.Load())

	ptr.Clear()
	assertion.Equal(0, *ptr.Load())
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("non-nil value", func(t *testing.T) {
		t.Parallel()

		assertion := assert.New(t)
		must := require.New(t)

		ptr := New[testStruct]()
		ptr.Update(func(val *testStruct) {
			val.Name = "hello"
			val.Value = 42
		})
		data, err := ptr.MarshalJSON()
		must.NoError(err)
		assertion.JSONEq(`{"Name":"hello","Value":42}`, string(data))
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		assertion := assert.New(t)
		must := require.New(t)

		ptr := New[int]()
		data, err := ptr.MarshalJSON()
		must.NoError(err)
		assertion.Equal("0", string(data))
	})

	t.Run("nil pointer returns null", func(t *testing.T) {
		t.Parallel()

		assertion := assert.New(t)
		must := require.New(t)

		ptr := New[int]()
		ptr.Pointer.Store(nil)
		data, err := ptr.MarshalJSON()
		must.NoError(err)
		assertion.Equal("null", string(data))
	})
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantName string
		wantVal  int
		wantNil  bool
		wantErr  bool
	}{
		{"valid json", `{"Name":"world","Value":7}`, "world", 7, false, false},
		{"null", "null", "", 0, true, false},
		{"invalid json", `not json`, "", 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			ptr := New[testStruct]()
			err := ptr.UnmarshalJSON([]byte(tt.input))

			if tt.wantErr {
				assertion.Error(err)

				return
			}

			require.NoError(t, err)

			if tt.wantNil {
				assertion.Nil(ptr.Load())

				return
			}

			val := ptr.Load()
			assertion.NotNil(val)
			assertion.Equal(tt.wantName, val.Name)
			assertion.Equal(tt.wantVal, val.Value)
		})
	}
}

func TestMarshalUnmarshalRoundtrip(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	must := require.New(t)

	ptr := New[testStruct]()
	ptr.Update(func(val *testStruct) {
		val.Name = "roundtrip"
		val.Value = 123
	})

	data, err := json.Marshal(ptr)
	must.NoError(err)

	ptr2 := New[testStruct]()

	err = json.Unmarshal(data, ptr2)
	must.NoError(err)

	val := ptr2.Load()
	assertion.Equal("roundtrip", val.Name)
	assertion.Equal(123, val.Value)
}

func TestUpdateDoesNotModifyOriginal(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	ptr := New[testStruct]()
	ptr.Update(func(val *testStruct) {
		val.Name = "original"
		val.Value = 1
	})

	snapshot := *ptr.Load()

	ptr.Update(func(val *testStruct) {
		val.Name = "modified"
		val.Value = 2
	})

	assertion.Equal("original", snapshot.Name)
	assertion.Equal(1, snapshot.Value)

	current := ptr.Load()
	assertion.Equal("modified", current.Name)
	assertion.Equal(2, current.Value)
}
