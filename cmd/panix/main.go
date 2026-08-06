package main

import (
	"os"

	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
	"github.com/mihakrumpestar/panix/gen"
	commands_standalone "github.com/mihakrumpestar/panix/internal/commands/standalone"
	commands_workflow "github.com/mihakrumpestar/panix/internal/commands/workflow"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/posener/complete"
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

	Completion kongcompletion.Completion `cmd:"" help:"Print shell code for initializing tab completion (bash, zsh, fish)"`
}

func main() {
	cli := cliCmd{}

	if len(os.Args) <= 1 {
		os.Args = append(os.Args, "--help")
	}

	parser := kong.Must(&cli,
		kong.Name("panix"),
		kong.Description("Universal NixOS Deployment Tool"),
		kong.Vars{"version": gen.Version()},
		kong.DefaultEnvars("PANIX"),
	)

	// Derive phase names and activation modes from their source-of-truth
	// registries so the predictors stay in sync if new values are added.
	phaseNames := make([]string, len(phase.PhasesInOrder()))
	for i, p := range phase.PhasesInOrder() {
		phaseNames[i] = string(p)
	}

	// Register shell completion handlers. Must be called before Parse.
	// When the binary is invoked by a shell completion mechanism (COMP_LINE set),
	// kongcompletion intercepts the call, prints completions to stdout, and exits early.
	kongcompletion.Register(parser,
		// File path predictors
		kongcompletion.WithPredictor("file", complete.PredictFiles("*")),
		kongcompletion.WithPredictor("yaml-file", complete.PredictOr(
			complete.PredictFiles("*.yml"),
			complete.PredictFiles("*.yaml"),
		)),
		kongcompletion.WithPredictor("json-file", complete.PredictFiles("*.json")),
		kongcompletion.WithPredictor("log-file", complete.PredictFiles("*.log")),
		kongcompletion.WithPredictor("dir", complete.PredictDirs("*")),
		// Enum predictors for fixed-value flags
		kongcompletion.WithPredictor("phase", complete.PredictSet(phaseNames...)),
		kongcompletion.WithPredictor("output-mode", complete.PredictSet(
			string(flags.OutputModeTui),
			string(flags.OutputModeConsole),
			string(flags.OutputModeJSON),
		)),
		kongcompletion.WithPredictor("activation-mode", complete.PredictSet(
			installable.ActivationModes()...,
		)),
	)

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	err = ctx.Run()
	ctx.FatalIfErrorf(err)
}
