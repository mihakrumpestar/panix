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

const (
	ContextConfigKey = "config"
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

			return context.WithValue(ctx, ContextConfigKey, conf), nil
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
					return runWorkflow(ctx, []phases.Phase{phases.Inspect})
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
					return runWorkflow(ctx, phases.PhasesInOrder())
				},
			},
			{
				Name:        "build",
				Usage:       "Build all selected closures",
				Description: "Build compiles the NixOS configurations for all selected machines.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runWorkflow(ctx, []phases.Phase{phases.Build})
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
					return runWorkflow(ctx, phases.PhasesInOrder())
				},
			},
			{
				Name:        "secrets",
				Usage:       "Deploy secrets to all machines",
				Description: "Secrets deploys encrypted secrets and credentials to all selected machines.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runWorkflow(ctx, []phases.Phase{phases.Inspect, phases.Secrets})
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
	conf, ok := ctx.Value(ContextConfigKey).(*config.Config)
	if !ok || conf == nil {
		panic(fmt.Errorf("internal error: %s key is nil/empty in cmd context", ContextConfigKey))
	}
	return conf
}

// runWorkflow creates a workflow with the given phases and runs the TUI
func runWorkflow(ctx context.Context, phaseList []phases.Phase) error {
	conf := ConfFromContext(ctx)
	workflowExec, err := workflow.NewWorkflow(ctx, conf, phaseList)
	if err != nil {
		return err
	}
	return tui.NewTui(workflowExec)
}
