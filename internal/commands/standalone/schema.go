package commands_standalone

import (
	"reflect"

	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/yamlschema"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/pkg/errors"
)

// schemaIDPrefix is the raw content base of the published generated schemas;
// every schema $id appends its default output file name.
const schemaIDPrefix = "https://raw.githubusercontent.com/mihakrumpestar/panix/main/gen/"

type SchemaCmd struct {
	// Output is the panix.yml configuration schema output path.
	Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"panix-schema.yaml" completion-predictor:"yaml-file"`

	// ThemesOutput is the panix-themes.yml themes file schema output path.
	ThemesOutput string `name:"themes-output" short:"t" help:"Themes file schema output path, use '-' for stdout" default:"panix-themes-schema.yaml" completion-predictor:"yaml-file"` //nolint:lll // struct tag is one literal; help and defaults are the public flag contract
}

// Run emits both generated schemas in one run: the panix.yml configuration
// schema to Output and the panix-themes.yml themes file schema to ThemesOutput.
func (c *SchemaCmd) Run() error {
	err := writeSchema(c.Output, configSchemaConfig())
	if err != nil {
		return err
	}

	return writeSchema(c.ThemesOutput, themesSchemaConfig())
}

// configSchemaConfig is the schema metadata for the panix.yml deployment
// configuration file.
func configSchemaConfig() yamlschema.SchemaConfig {
	return yamlschema.SchemaConfig{
		RootType:    reflect.TypeFor[config.Config](),
		SchemaID:    schemaIDPrefix + "panix-schema.yaml",
		Title:       "Panix Configuration Schema",
		Description: "Schema for Panix deployment configuration file",
		Version:     gen.Version(),
	}
}

// themesSchemaConfig is the schema metadata for the panix-themes.yml custom
// themes file, rooted at theme.ThemesFileRootType.
func themesSchemaConfig() yamlschema.SchemaConfig {
	return yamlschema.SchemaConfig{
		RootType:    theme.ThemesFileRootType(),
		SchemaID:    schemaIDPrefix + "panix-themes-schema.yaml",
		Title:       "Panix Themes File Schema",
		Description: "Schema for Panix custom themes file " + theme.ThemesFileName,
		Version:     gen.Version(),
	}
}

// writeSchema generates the schema described by cfg and writes it to output
// ("-" writes to stdout), the shared tail of both schema outputs.
func writeSchema(output string, cfg yamlschema.SchemaConfig) error {
	schema, err := yamlschema.NewSchema(cfg).Generate()
	if err != nil {
		return errors.Wrap(err, "failed to generate schema")
	}

	return errors.Wrap(yamlx.WriteTo(schema, output), "failed to write schema")
}
