package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/mihakrumpestar/panix/gen"
	commands_standalone "github.com/mihakrumpestar/panix/internal/commands/standalone"
	commands_workflow "github.com/mihakrumpestar/panix/internal/commands/workflow"
)

//nolint:lll
type cliCmd struct {
	Version kong.VersionFlag `name:"version" help:"Show version"`

	Init     commands_standalone.InitCmd     `cmd:"" help:"Initialize a new panix configuration file"`
	Schema   commands_standalone.SchemaCmd   `cmd:"" help:"Generate YAML schema for configuration files"`
	Template commands_standalone.TemplateCmd `cmd:"" help:"Process templates and anchors, output the result"`
	Eval     commands_standalone.EvalCmd     `cmd:"" help:"Fully evaluate and validate configuration (including templating) for execution, output the result"`
	Snapshot commands_standalone.SnapshotCmd `cmd:"" help:"View snapshot in TUI"`

	Inspect  commands_workflow.InspectCmd  `cmd:"" help:"Inspect machines"`
	Build    commands_workflow.BuildCmd    `cmd:"" help:"Build NixOS closures"`
	Deploy   commands_workflow.DeployCmd   `cmd:"" help:"Do full workflow (inspect -> bootstrap -> build -> transfer -> secrets -> activate)"`
	Secrets  commands_workflow.SecretsCmd  `cmd:"" help:"Deploy secrets to machines"`
	Rollback commands_workflow.RollbackCmd `cmd:"" help:"Rollback to a previous generation, use optional --gen=NUMBER flag (default is -1)"`
}

func main() {
	cli := cliCmd{}

	if len(os.Args) <= 1 {
		os.Args = append(os.Args, "--help")
	}

	ctx := kong.Parse(&cli,
		kong.Name("panix"),
		kong.Description("Universal NixOS Deployment Tool"),
		kong.Vars{"version": gen.Version()},
		kong.DefaultEnvars("PANIX"),
	)

	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
