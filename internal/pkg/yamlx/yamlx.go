package yamlx

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/mattn/go-colorable"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/pkg/errors"
)

func Encode(val any, writer io.Writer) error {
	encoder := yaml.NewEncoder(
		writer,
		yaml.WithSmartAnchor(),
		yaml.UseLiteralStyleIfMultiline(true),
		yaml.Indent(2), //nolint:mnd
		yaml.OmitEmpty(),
		yaml.OmitZero(),
	)

	err := encoder.Encode(val)
	if err != nil {
		return errors.Wrap(err, "YAML encoder")
	}

	return nil
}

func Decode(b []byte, val any) error {
	decoder := yaml.NewDecoder(
		bytes.NewReader(b),
		yaml.Strict(),
		yaml.AllowFieldPrefixes("anchor_"),
		yaml.UseOrderedMap(),
	)

	err := decoder.Decode(val)
	if err != nil {
		return errors.New("YAML decoder: " + yaml.FormatError(err, true, true))
	}

	return nil
}

func WriteTo(val any, path string) error {
	if path == "-" {
		return fprintColorized(colorable.NewColorableStdout(), val)
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		err := os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %s", dir)
		}
	}

	file, err := os.Create(path) //nolint:gosec // User provided
	if err != nil {
		return errors.Wrapf(err, "failed to create output file %s", path)
	}
	defer file.Close() //nolint:errcheck

	return Encode(val, file)
}

func fprintColorized(writer io.Writer, val any) error {
	var buf bytes.Buffer

	err := Encode(val, &buf)
	if err != nil {
		return err
	}

	return printColorized(buf.Bytes(), writer)
}
