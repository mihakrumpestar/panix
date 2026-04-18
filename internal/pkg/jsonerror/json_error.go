package jsonerror

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// JSONError wraps an error with JSON marshal/unmarshal support.
// Go's error interface cannot be unmarshaled from JSON, so we serialize as a string.
// Nil *JSONError means no error, non-nil means error is set.
type JSONError struct {
	err error
}

func New(err error) *JSONError {
	if err == nil {
		return nil
	}

	return &JSONError{err: err}
}

// Error satisfies error interface.
func (e *JSONError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}

	return e.err.Error()
}

func (e *JSONError) MarshalJSON() ([]byte, error) {
	if e == nil || e.err == nil {
		return []byte("null"), nil
	}

	b, err := json.Marshal(e.err.Error())

	return b, errors.Wrap(err, "marshal error json")
}

func (e *JSONError) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		e.err = nil

		return nil
	}

	var val string

	err := json.Unmarshal(data, &val)
	if err != nil {
		return errors.Wrap(err, "unmarshal error json")
	}

	e.err = errors.New(val)

	return nil
}
