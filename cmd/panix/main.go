package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"

	"github.com/alecthomas/kong"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/nixschema"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
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

	Deploy struct{} `cmd:"" help:"Do full workflow (inspect -> build -> bootstrap -> transfer -> secrets -> activate)"`

	Secrets struct{} `cmd:"" help:"Deploy secrets to all machines"`

	Eval struct {
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"-"`
	} `cmd:"" help:"Evaluate and print Nix config as JSON"`

	NixOptions struct {
		Output string `name:"output" short:"o" help:"Output file path for generated Nix options" default:"gen/panix-options.nix"`
	} `cmd:"" help:"Generate Nix module options from Go structs"`

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
	case "eval":
		ctx.FatalIfErrorf(cli.runEvalCommand(cli.Eval.Output))
	case "nix-options":
		ctx.FatalIfErrorf(cli.runNixOptionsCommand(cli.NixOptions.Output))
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

func (c *CLI) runEvalCommand(outputPath string) error {
	conf, err := config.LoadNixConfig(c.Flags.Config, config.NixArgs{
		"env":           c.Flags.Env,
		"validatePaths": "false",
	})
	if err != nil {
		return errors.Wrap(err, "failed to load nix config")
	}

	// Initialize flags with defaults
	if conf.Flags == nil {
		conf.Flags = &flags.Flags{}
	}
	if err := conf.Flags.MergeConfWithCliFlags(c.Flags); err != nil {
		return errors.Wrap(err, "failed to merge flags")
	}

	// Apply the transformation that would normally happen in LoadConfig
	// (but skip validation and filtering for eval)
	if err := conf.InitializeFlakes(); err != nil {
		return errors.Wrap(err, "failed to initialize flakes")
	}

	// Output as JSON
	output, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal config to JSON")
	}

	if outputPath == "-" {
		os.Stdout.Write(output)
		os.Stdout.WriteString("\n")
	} else {
		if err := os.WriteFile(outputPath, output, 0644); err != nil {
			return errors.Wrap(err, "failed to write output file")
		}
	}

	return nil
}

func (c *CLI) runNixOptionsCommand(outputPath string) error {
	// Generate Nix schema from Go structs
	generator := nixschema.NewGenerator()

	// Parse all relevant structs for schema documentation
	// Order matters - base types first, then complex types that use them
	generator.ParseStruct(reflect.TypeOf(ssh.SSHClient{}))
	generator.ParseStruct(reflect.TypeOf(attributes.KexecConfig{}))
	generator.ParseStruct(reflect.TypeOf(attributes.NixConfig{}))
	generator.ParseStruct(reflect.TypeOf(attributes.PlainFileOrDirToTransfer{}))
	generator.ParseStruct(reflect.TypeOf(attributes.Bootstrap{}))
	generator.ParseStruct(reflect.TypeOf(attributes.Attributes{}))
	generator.ParseStruct(reflect.TypeOf(config.Flake{}))
	generator.ParseStruct(reflect.TypeOf(config.Configuration{}))
	generator.ParseStruct(reflect.TypeOf(config.Machine{}))
	generator.ParseStruct(reflect.TypeOf(flags.Flags{}))

	// Generate Nix module with options
	nixContent := generator.GenerateNixModule()

	if outputPath == "-" {
		os.Stdout.WriteString(nixContent)
	} else {
		// Ensure directory exists
		dir := filepath.Dir(outputPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return errors.Wrap(err, "failed to create output directory")
			}
		}

		if err := os.WriteFile(outputPath, []byte(nixContent), 0644); err != nil {
			return errors.Wrap(err, "failed to write output file")
		}
	}

	return nil
}
