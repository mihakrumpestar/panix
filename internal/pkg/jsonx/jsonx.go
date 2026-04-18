package jsonx

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
)

func Decode(b []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewBuffer(b))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(v)
	if err != nil {
		return errors.Wrap(err, "JSON decoder")
	}

	return nil
}

func Encode(v any, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(v)
	if err != nil {
		return errors.Wrap(err, "JSON encoder")
	}

	return nil
}
