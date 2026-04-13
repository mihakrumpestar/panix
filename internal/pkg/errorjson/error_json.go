package errorjson

import (
	"encoding/json"
	"errors"
)

// ErrorJSON wraps an error with JSON marshal/unmarshal support.
// Go's error interface cannot be unmarshaled from JSON, so we serialize as a string.
// Nil *ErrorJSON means no error; non-nil means error is set.
type ErrorJSON struct {
	err error
}

func New(err error) *ErrorJSON {
	if err == nil {
		return nil
	}
	return &ErrorJSON{err: err}
}

// Error satisfies error interface
func (e *ErrorJSON) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *ErrorJSON) MarshalJSON() ([]byte, error) {
	if e == nil || e.err == nil {
		return []byte("null"), nil
	}
	return json.Marshal(e.err.Error())
}

func (e *ErrorJSON) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		e.err = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	e.err = errors.New(s)
	return nil
}
