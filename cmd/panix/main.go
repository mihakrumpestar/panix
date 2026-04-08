package main

import (
	"context"
	"os"

	"github.com/alecthomas/kong"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/schema"
	"github.com/mihakrumpestar/panix/internal/config/template"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type CLI struct {
	flags.GlobalFlags

	Version kong.VersionFlag `name:"version" help:"Show version"`

	Schema struct {
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"panix-schema.yaml"`
	} `cmd:"" help:"Generate YAML schema for configuration files"`

	Eval struct {
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"-"`
	} `cmd:"" help:"Evaluate config (process templates and anchors) and output result"`

	Inspect struct {
		flags.WorkflowFlags
	} `cmd:"" help:"Inspect machine per host"`

	Build struct {
		flags.WorkflowFlags
	} `cmd:"" help:"Build all selected closures"`

	Deploy struct {
		flags.WorkflowFlags
	} `cmd:"" help:"Do full workflow (inspect -> build -> bootstrap -> transfer -> secrets -> activate)"`

	Secrets struct {
		flags.WorkflowFlags
	} `cmd:"" help:"Deploy secrets to all machines"`

	Rollback struct {
		flags.WorkflowFlags

		Generation int `arg:"" name:"generation" help:"0=current generation, -N=Nth before current, N=specific generation" default:"-1"`
	} `cmd:"" help:"Rollback to a previous generation"`
}

func main() {
	cli := CLI{}

	if len(os.Args) <= 1 {
		os.Args = append(os.Args, "--help")
	}

	ctx := kong.Parse(&cli,
		kong.Name("panix"),
		kong.Description("Universal NixOS Deployment Tool"),
		kong.Vars{"version": gen.Version()},
		kong.DefaultEnvars("PANIX"),
	)

	switch ctx.Command() {
	case "schema":
		ctx.FatalIfErrorf(cli.runSchemaCommand(cli.GlobalFlags, cli.Schema.Output))
	case "eval":
		ctx.FatalIfErrorf(cli.runEvalCommand(cli.GlobalFlags, cli.Eval.Output))

	// Wokflow commands
	case "inspect":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Inspect.WorkflowFlags, []phases.Phase{phases.Inspect}))
	case "build":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Build.WorkflowFlags, []phases.Phase{phases.Build}))
	case "deploy":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Deploy.WorkflowFlags, phases.PhasesInOrder()))
	case "secrets":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Secrets.WorkflowFlags, []phases.Phase{phases.Inspect, phases.Secrets}))
	case "rollback", "rollback <generation>":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Rollback.WorkflowFlags, []phases.Phase{phases.Inspect, phases.Rollback},
			func(conf *config.Config) {
				conf.RollbackGeneration = cli.Rollback.Generation
			}))
	default:
		// Kong handles unknown commands, this should never be reached
	}
}

func (c *CLI) runTui(gf flags.GlobalFlags, wf flags.WorkflowFlags, commandPhases []phases.Phase, modifiers ...func(conf *config.Config)) error {
	conf, err := config.LoadConfig(flags.Flags{GlobalFlags: gf, WorkflowFlags: wf}, commandPhases)
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	for _, modifier := range modifiers {
		modifier(conf)
	}

	if conf.Flags.Output != flags.OutputModeTui {
		return errors.Wrap(tui.NewHeadless(context.Background(), conf), "headless error")
	}

	return errors.Wrap(tui.NewTui(context.Background(), conf), "TUI error")
}

func (c *CLI) runEvalCommand(gf flags.GlobalFlags, outputPath string) error {
	return errors.Wrap(template.EvalConfig(gf.Config, outputPath), "failed to evaluate config")
}

func (c *CLI) runSchemaCommand(_ flags.GlobalFlags, outputPath string) error {
	return errors.Wrap(schema.GenerateSchema(outputPath), "failed to generate schema")
}
