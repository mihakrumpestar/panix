package commands_standalone

import (
	"github.com/mihakrumpestar/panix/internal/pkg/schema"
	"github.com/mihakrumpestar/panix/internal/pkg/yamlx"
	"github.com/pkg/errors"
)

type SchemaCmd struct {
	Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"panix-schema.yaml"`
}

func (c *SchemaCmd) Run() error {
	generator := schema.NewSchema()

	schema, err := generator.Generate()
	if err != nil {
		return errors.Wrap(err, "failed to generate schema")
	}

	err = yamlx.WriteTo(schema, c.Output)

	return errors.Wrap(err, "failed to write schema")
}
