package template

import (
	"bytes"
	"text/template"

	"github.com/go-sprout/sprout"
	"github.com/go-sprout/sprout/group/all"
	"github.com/pkg/errors"
)

func ProcessTemplate(rawYAML []byte) ([]byte, error) {
	handler := sprout.New(
		sprout.WithGroups(all.RegistryGroup()),
	)

	tmpl, err := template.New("config").
		Funcs(handler.Build()).
		Parse(string(rawYAML))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse template")
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute template")
	}

	return buf.Bytes(), nil
}
