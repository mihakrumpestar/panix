package jsonerror

import (
	"encoding/json"
	"errors"
	"testing"
)

var (
	errTest         = errors.New("test error")
	errOther        = errors.New("other error")
	errStatic       = errors.New("static")
	errSpecialChars = errors.New(`message with "quotes" and \backslash\`)
)

func TestNew(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)
	if jsonErr == nil {
		t.Fatal("New(non-nil) returned nil")
	}

	if !errors.Is(jsonErr.Err(), errTest) {
		t.Errorf("Err() = %v, want %v", jsonErr.Err(), errTest)
	}
}

func TestNewNil(t *testing.T) {
	t.Parallel()

	jsonErr := New(nil)
	if jsonErr != nil {
		t.Error("New(nil) should return nil")
	}
}

func TestErr(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)
	if jsonErr.Err() == nil {
		t.Error("Err() returned nil, want non-nil")
	}

	if jsonErr.Err().Error() != "test error" {
		t.Errorf("Err().Error() = %q, want %q", jsonErr.Err().Error(), "test error")
	}
}

func TestErrNilReceiver(t *testing.T) {
	t.Parallel()

	var jsonErr *JSONError
	if jsonErr.Err() != nil {
		t.Error("Err() on nil receiver should return nil")
	}
}

func TestError(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)
	if jsonErr.Error() != "test error" {
		t.Errorf("Error() = %q, want %q", jsonErr.Error(), "test error")
	}
}

func TestErrorNilReceiver(t *testing.T) {
	t.Parallel()

	var jsonErr *JSONError
	if jsonErr.Error() != "" {
		t.Errorf("Error() on nil receiver = %q, want empty string", jsonErr.Error())
	}
}

func TestErrorNilInnerErr(t *testing.T) {
	t.Parallel()

	jsonErr := &JSONError{err: nil}
	if jsonErr.Error() != "" {
		t.Errorf("Error() with nil inner err = %q, want empty string", jsonErr.Error())
	}
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)

	data, err := json.Marshal(jsonErr)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	if string(data) != `"test error"` {
		t.Errorf("MarshalJSON() = %s, want %q", string(data), `"test error"`)
	}
}

func TestMarshalJSONNilReceiver(t *testing.T) {
	t.Parallel()

	var jsonErr *JSONError

	data, err := json.Marshal(jsonErr)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	if string(data) != "null" {
		t.Errorf("MarshalJSON() on nil receiver = %s, want null", string(data))
	}
}

func TestMarshalJSONNilInnerErr(t *testing.T) {
	t.Parallel()

	jsonErr := &JSONError{err: nil}

	data, err := json.Marshal(jsonErr)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	if string(data) != "null" {
		t.Errorf("MarshalJSON() with nil inner err = %s, want null", string(data))
	}
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	jsonErr := &JSONError{}

	err := json.Unmarshal([]byte(`"something went wrong"`), jsonErr)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	if jsonErr.Err() == nil {
		t.Fatal("UnmarshalJSON() should set inner error")
	}

	if jsonErr.Err().Error() != "something went wrong" {
		t.Errorf("Err().Error() = %q, want %q", jsonErr.Err().Error(), "something went wrong")
	}
}

func TestUnmarshalJSONNull(t *testing.T) {
	t.Parallel()

	jsonErr := New(errTest)

	err := json.Unmarshal([]byte("null"), jsonErr)
	if err != nil {
		t.Fatalf("UnmarshalJSON(null) error: %v", err)
	}

	if jsonErr.Err() != nil {
		t.Error("UnmarshalJSON(null) should set inner error to nil")
	}
}

func TestUnmarshalJSONInvalid(t *testing.T) {
	t.Parallel()

	jsonErr := &JSONError{}

	err := json.Unmarshal([]byte(`123`), jsonErr)
	if err == nil {
		t.Error("UnmarshalJSON() with non-string should return error")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := New(errOther)

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	restored := &JSONError{}

	err = json.Unmarshal(data, restored)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	if restored.Error() != original.Error() {
		t.Errorf("round trip: got %q, want %q", restored.Error(), original.Error())
	}
}

func TestJSONRoundTripNull(t *testing.T) {
	t.Parallel()

	var original *JSONError

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	if string(data) != "null" {
		t.Fatalf("MarshalJSON(nil) = %s, want null", string(data))
	}

	restored := New(errStatic)

	err = json.Unmarshal(data, restored)
	if err != nil {
		t.Fatalf("UnmarshalJSON(null) error: %v", err)
	}

	if restored.Err() != nil {
		t.Error("UnmarshalJSON(null) should clear inner error")
	}
}

func TestMarshalJSONSpecialChars(t *testing.T) {
	t.Parallel()

	jsonErr := New(errSpecialChars)

	data, err := json.Marshal(jsonErr)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	restored := &JSONError{}

	err = json.Unmarshal(data, restored)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error: %v", err)
	}

	if restored.Error() != jsonErr.Error() {
		t.Errorf("round trip special chars: got %q, want %q", restored.Error(), jsonErr.Error())
	}
}
