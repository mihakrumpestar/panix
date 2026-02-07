package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/urfave/cli/v3"
	"github.com/urfave/sflags"
	"github.com/urfave/sflags/gen/gcli"
)

func main() {
	ctx := context.Background()

	flags := config_flags.Flags{}
	flags.SetDefault(false)

	parsedFlags, err := gcli.ParseV3(&flags,
		sflags.EnvPrefix("PANIX_"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	cmd := &cli.Command{
		Name:    "panix",
		Usage:   "Universal NixOS Deployment Tool",
		Version: "0.1.0",
		Flags:   parsedFlags,
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			conf, err := config.LoadConfig(flags)
			if err != nil {
				return ctx, fmt.Errorf("failed to load config: %w", err)
			}

			return context.WithValue(ctx, config.ContextConfigKey, conf), nil
		},
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "Query machine status per host",
				Description: `Status queries and displays the current machine status of all configured hosts.
This includes:
- Current NixOS generation
- Last deployment time
- SSH connectivity status
- Bootstrap status (initialized/uninitialized)`,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					conf := ConfFromContext(ctx)
					phases := []phases.Phase{phases.Inspect}
					workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
					if err != nil {
						return err
					}
					return RunTUI(workflowExec)
				},
			},
			{
				Name:  "bootstrap",
				Usage: "Explicit bootstrap phase",
				Description: `Bootstrap initializes uninitialized machines.
This command will:
- Check which machines need bootstrapping
- Copy pre-bootstrap secrets if configured
- Run nixos-anywhere to install NixOS
- Wait for machines to reboot and become available

Same as "deploy --bootstrap.only".`,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					conf := ConfFromContext(ctx)
					conf.Flags.Bootstrap.Only = true
					phases := phases.PhasesInOrder()
					workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
					if err != nil {
						return err
					}
					return RunTUI(workflowExec)
				},
			},
			{
				Name:        "build",
				Usage:       "Build all selected closures",
				Description: "Build compiles the NixOS configurations for all selected machines.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					conf := ConfFromContext(ctx)
					phases := []phases.Phase{phases.Build}
					workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
					if err != nil {
						return err
					}
					return RunTUI(workflowExec)
				},
			},
			{
				Name:  "deploy",
				Usage: "Do full workflow (inspect → bootstrap → secrets → build → push → activate)",
				Description: `Deploy performs the complete deployment workflow:
1. Inspect (SSH connectivity, bootstrap status)
2. Bootstrap uninitialized machines (if needed)
3. Deploy secrets to all machines
4. Build NixOS configurations
5. Transfer closures to target machines
6. Activate new configurations

This is the main command for deploying NixOS configurations.`,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					conf := ConfFromContext(ctx)
					phases := phases.PhasesInOrder()
					workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
					if err != nil {
						return err
					}
					return RunTUI(workflowExec)
				},
			},
			{
				Name:        "secrets",
				Usage:       "Deploy secrets to all machines",
				Description: "Secrets deploys encrypted secrets and credentials to all selected machines.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					conf := ConfFromContext(ctx)
					phases := []phases.Phase{phases.Inspect, phases.Secrets}
					workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
					if err != nil {
						return err
					}
					return tui.NewTui(workflowExec)
				},
			},
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// ConfFromContext retrieves config from context
func ConfFromContext(ctx context.Context) *config.Config {
	conf := ctx.Value(config.ContextConfigKey).(*config.Config)
	if conf == nil {
		panic(fmt.Errorf("internal error: %s key is nil/empty in cmd context", config.ContextConfigKey))
	}
	return conf
}

// RunTUI runs the TUI with the given workflow
func RunTUI(workflowExec *workflow.Workflow) error {
	err := tui.NewTui(workflowExec)
	if err != nil {
		os.Exit(1)
	}
	return nil
}
