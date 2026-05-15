package commands_standalone

import (
	"reflect"

	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/pkg/yamlschema"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/pkg/errors"
)

type SchemaCmd struct {
	Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"panix-schema.yaml"`
}

func (c *SchemaCmd) Run() error {
	generator := yamlschema.NewSchema(yamlschema.SchemaConfig{
		RootType:    reflect.TypeFor[config.Config](),
		SchemaID:    "https://raw.githubusercontent.com/mihakrumpestar/panix/main/gen/panix-schema.yaml",
		Title:       "Panix Configuration Schema",
		Description: "Schema for Panix NixOS deployment configuration files",
		Version:     gen.Version(),
	})

	schema, err := generator.Generate()
	if err != nil {
		return errors.Wrap(err, "failed to generate schema")
	}

	err = yamlx.WriteTo(schema, c.Output)

	return errors.Wrap(err, "failed to write schema")
}
