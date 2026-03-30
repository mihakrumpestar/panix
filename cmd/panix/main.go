package main

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/schema"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var ErrUnknownCommand = errors.New("unknown command")

type CLI struct {
	flags.Flags

	Version kong.VersionFlag `name:"version" help:"Show version"`

	Inspect struct{} `cmd:"" help:"Inspect machine per host"`

	Build struct{} `cmd:"" help:"Build all selected closures"`

	Deploy struct{} `cmd:"" help:"Do full workflow (inspect -> bootstrap -> secrets -> build -> push -> activate)"`

	Secrets struct{} `cmd:"" help:"Deploy secrets to all machines"`

	Schema struct {
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"panix-schema.yaml"`
	} `cmd:"" help:"Generate YAML schema for configuration files"`

	Rollback struct {
		Generation int `arg:"" name:"generation" help:"0=current generation, -N=Nth before current, N=specific generation" default:"-1"`
	} `cmd:"" help:"Rollback to a previous generation"`
}

func main() {
	cli := CLI{}

	ctx := kong.Parse(&cli,
		kong.Name("panix"),
		kong.Description("Universal NixOS Deployment Tool"),
		kong.Vars{"version": gen.Version()},
		kong.DefaultEnvars("PANIX"),
	)

	switch ctx.Command() {
	case "inspect":
		ctx.FatalIfErrorf(cli.runTui([]phases.Phase{phases.Inspect}, func(conf *config.Config) {
			conf.Flags.Bootstrap.DisableAuto = true
		}))
	case "build":
		ctx.FatalIfErrorf(cli.runTui([]phases.Phase{phases.Build}))
	case "deploy":
		ctx.FatalIfErrorf(cli.runTui(phases.PhasesInOrder()))
	case "secrets":
		ctx.FatalIfErrorf(cli.runTui([]phases.Phase{phases.Inspect, phases.Secrets}))
	case "schema":
		ctx.FatalIfErrorf(cli.runSchemaCommand(cli.Schema.Output))
	case "rollback", "rollback <generation>":
		ctx.FatalIfErrorf(cli.runTui([]phases.Phase{phases.Inspect, phases.Rollback}, func(conf *config.Config) {
			conf.RollbackGeneration = cli.Rollback.Generation
		}))
	default:
		ctx.FatalIfErrorf(errors.Wrapf(ErrUnknownCommand, "%s", ctx.Command()))
	}
}

func (c *CLI) runTui(commandPhases []phases.Phase, modifiers ...func(conf *config.Config)) error {
	conf, err := config.LoadConfig(c.Flags, commandPhases)
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	for _, modifier := range modifiers {
		modifier(conf)
	}

	return errors.Wrap(tui.NewTui(context.Background(), conf), "TUI error")
}

func (c *CLI) runSchemaCommand(outputPath string) error {
	return errors.Wrap(schema.GenerateSchema(outputPath), "failed to generate schema")
}
