package main

import (
	"context"
	"os"

	"github.com/alecthomas/kong"
	"github.com/mihakrumpestar/panix/examples"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/schema"
	"github.com/mihakrumpestar/panix/internal/config/template"
	"github.com/mihakrumpestar/panix/internal/snapshot"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

type CLI struct {
	Version kong.VersionFlag `name:"version" help:"Show version"`

	// Complementary commands

	Init struct {
		Output string `name:"output" short:"o" help:"Output file path" default:"panix.yml"`
		Force  bool   `name:"force" short:"f" help:"Overwrite existing file"`
	} `cmd:"" help:"Initialize a new panix configuration file"`

	Schema struct {
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"panix-schema.yaml"`
	} `cmd:"" help:"Generate YAML schema for configuration files"`

	Eval struct {
		flags.ConfigFlag
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"-"`
	} `cmd:"" help:"Evaluate config (process templates and anchors) and output result"`

	Snapshot struct {
		Path string `name:"path" short:"p" help:"Snapshot file path" required:""`
	} `cmd:"" help:"View snapshot in TUI"`

	// Workflow commands

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
		flags.RollbackFlags
		flags.WorkflowFlags
	} `cmd:"" help:"Rollback to a previous generation, use optional --gen=NUMBER flag, default is -1"`
}

//nolint:cyclop
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

	// Complementary commands

	case "init":
		ctx.FatalIfErrorf(cli.runInitCommand(cli.Init.Output, cli.Init.Force))
	case "schema":
		ctx.FatalIfErrorf(cli.runSchemaCommand(cli.Schema.Output))
	case "eval":
		ctx.FatalIfErrorf(cli.runEvalCommand(cli.Eval.Config, cli.Eval.Output))
	case "snapshot":
		ctx.FatalIfErrorf(cli.runSnapshot(cli.Snapshot.Path))

	// Wokflow commands

	case "inspect":
		ctx.FatalIfErrorf(cli.runTui(flags.Flags{WorkflowFlags: cli.Rollback.WorkflowFlags}, []phase.Phase{phase.Inspect}))
	case "build":
		ctx.FatalIfErrorf(cli.runTui(flags.Flags{WorkflowFlags: cli.Rollback.WorkflowFlags}, []phase.Phase{phase.Build}))
	case "deploy":
		ctx.FatalIfErrorf(cli.runTui(flags.Flags{WorkflowFlags: cli.Rollback.WorkflowFlags}, []phase.Phase{phase.Inspect, phase.Build, phase.Bootstrap, phase.Transfer, phase.Secrets, phase.Activate}))
	case "secrets":
		ctx.FatalIfErrorf(cli.runTui(flags.Flags{WorkflowFlags: cli.Rollback.WorkflowFlags}, []phase.Phase{phase.Inspect, phase.Secrets}))
	case "rollback":
		ctx.FatalIfErrorf(cli.runTui(flags.Flags{WorkflowFlags: cli.Rollback.WorkflowFlags, RollbackFlags: cli.Rollback.RollbackFlags}, []phase.Phase{phase.Inspect, phase.Rollback}))
	}
}

func (c *CLI) runInitCommand(outputPath string, force bool) error {
	if !force {
		_, err := os.Stat(outputPath)
		if err == nil {
			return errors.Errorf("file %s already exists, use --force to overwrite", outputPath)
		}
	}

	err := os.WriteFile(outputPath, examples.ExampleConfig, filepermissions.DefaultFilePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	return nil
}

func (c *CLI) runEvalCommand(configPath string, outputPath string) error {
	return errors.Wrap(template.EvalConfig(configPath, outputPath), "failed to evaluate config")
}

func (c *CLI) runSchemaCommand(outputPath string) error {
	return errors.Wrap(schema.GenerateSchema(outputPath), "failed to generate schema")
}

func (c *CLI) runSnapshot(path string) error {
	snap, err := snapshot.Read(path)
	if err != nil {
		return errors.Wrap(err, "failed to read snapshot file")
	}

	return errors.Wrap(tui.New(context.Background(), snap, true), "snapshot TUI error")
}

// Wokflow

func (c *CLI) runTui(f flags.Flags, commandPhases []phase.Phase, modifiers ...func(conf *config.Config)) error {
	conf, err := config.LoadConfig(f, commandPhases)
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	for _, modifier := range modifiers {
		modifier(conf)
	}

	if conf.Flags.Output != flags.OutputModeTui {
		return errors.Wrap(tui.NewHeadless(context.Background(), conf), "headless error")
	}

	return errors.Wrap(tui.New(context.Background(), conf, false), "TUI error")
}
