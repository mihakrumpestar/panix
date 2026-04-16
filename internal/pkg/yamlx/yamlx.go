package yamlx

import (
	"bytes"
	"io"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

func Decode(b []byte, v any) error {
	decoder := yaml.NewDecoder(
		bytes.NewReader(b),
		yaml.Strict(),
		yaml.AllowFieldPrefixes("anchor_"),
		yaml.UseOrderedMap(),
	)

	err := decoder.Decode(v)
	if err != nil {
		return errors.New("YAML decoder: " + yaml.FormatError(err, true, true))
	}

	return nil
}

func Encode(v any, w io.Writer) error {
	encoder := yaml.NewEncoder(
		w,
		yaml.WithSmartAnchor(),
		yaml.UseLiteralStyleIfMultiline(true),
		yaml.Indent(2),
		yaml.OmitEmpty(),
		yaml.OmitZero(),
	)

	err := encoder.Encode(v)
	if err != nil {
		return errors.Wrap(err, "YAML encoder")
	}

	return nil
}
