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
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type CLI struct {
	flags.GlobalFlags

	Version kong.VersionFlag `name:"version" help:"Show version"`

	Init struct {
		Output string `name:"output" short:"o" help:"Output file path" default:"panix.yml"`
		Force  bool   `name:"force" short:"f" help:"Overwrite existing file"`
	} `cmd:"" help:"Initialize a new panix configuration file"`

	Schema struct {
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"panix-schema.yaml"`
	} `cmd:"" help:"Generate YAML schema for configuration files"`

	Eval struct {
		Output string `name:"output" short:"o" help:"Output file path, use '-' for stdout" default:"-"`
	} `cmd:"" help:"Evaluate config (process templates and anchors) and output result"`

	Snapshot struct {
		Path string `name:"path" short:"p" help:"Snapshot file or directory" default:"."`
	} `cmd:"" help:"View snapshot in TUI"`

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
	} `cmd:"" help:"Rollback to a previous generation, use --gen=<number> flag, default is -1"`
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
	case "init":
		ctx.FatalIfErrorf(cli.runInitCommand(cli.Init.Output, cli.Init.Force))
	case "schema":
		ctx.FatalIfErrorf(cli.runSchemaCommand(cli.GlobalFlags, cli.Schema.Output))
	case "eval":
		ctx.FatalIfErrorf(cli.runEvalCommand(cli.GlobalFlags, cli.Eval.Output))
	case "snapshot":
		ctx.FatalIfErrorf(cli.runSnapshot(cli.Snapshot.Path))

	// Wokflow commands
	case "inspect":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Inspect.WorkflowFlags, []phases.Phase{phases.Inspect}))
	case "build":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Build.WorkflowFlags, []phases.Phase{phases.Build}))
	case "deploy":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Deploy.WorkflowFlags, phases.DeployPhasesInOrder()))
	case "secrets":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Secrets.WorkflowFlags, []phases.Phase{phases.Inspect, phases.Secrets}))
	case "rollback":
		ctx.FatalIfErrorf(cli.runTui(cli.GlobalFlags, cli.Rollback.WorkflowFlags, []phases.Phase{phases.Inspect, phases.Rollback},
			func(conf *config.Config) {
				//conf.Flags.RollbackFlags.Generation = cli.Rollback.Generation
			}))
	default:
		// Kong handles unknown commands, this should never be reached
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

func (c *CLI) runEvalCommand(gf flags.GlobalFlags, outputPath string) error {
	return errors.Wrap(template.EvalConfig(gf.Config, outputPath), "failed to evaluate config")
}

func (c *CLI) runSchemaCommand(_ flags.GlobalFlags, outputPath string) error {
	return errors.Wrap(schema.GenerateSchema(outputPath), "failed to generate schema")
}

func (c *CLI) runSnapshot(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %s", path)
	}

	var snapshots []*config.Config

	if stat.IsDir() {
		snapshots, err = snapshot.ReadDir(path)
		if err != nil {
			return errors.Wrap(err, "failed to read snapshots from directory")
		}
	} else {
		var s *config.Config

		s, err = snapshot.Read(path)
		if err != nil {
			return errors.Wrap(err, "failed to read snapshot file")
		}

		snapshots = []*config.Config{s}
	}

	if len(snapshots) == 0 {
		return errors.New("no snapshots found")
	}

	// Use the first snapshot for TUI display
	return errors.Wrap(tui.NewSnapshotTui(context.Background(), snapshots[0]), "snapshot TUI error")
}

// Wokflow

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
