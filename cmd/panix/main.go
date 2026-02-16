package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/config/config_schema"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
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
		fmt.Fprintf(os.Stderr, "failed parsing flags: %v\n", err)
		os.Exit(1)
	}

	cmd := &cli.Command{
		Name:    "panix",
		Usage:   "Universal NixOS Deployment Tool",
		Version: "0.1.0",
		Flags:   parsedFlags,
		Suggest: true,
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
					return runTui(ctx, flags, []phases.Phase{phases.Inspect})
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
					return runTui(ctx, flags, phases.PhasesInOrder(), func(conf *config.Config) {
						conf.Flags.Bootstrap.Only = true
					})
				},
			},
			{
				Name:        "build",
				Usage:       "Build all selected closures",
				Description: "Build compiles the NixOS configurations for all selected machines.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runTui(ctx, flags, []phases.Phase{phases.Build})
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
					return runTui(ctx, flags, phases.PhasesInOrder())
				},
			},
			{
				Name:        "secrets",
				Usage:       "Deploy secrets to all machines",
				Description: "Secrets deploys encrypted secrets and credentials to all selected machines.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runTui(ctx, flags, []phases.Phase{phases.Inspect, phases.Secrets})
				},
			},
			{
				Name:  "schema",
				Usage: "Generate YAML schema for configuration files",
				Description: `Schema generates a JSON/YAML schema for the Panix configuration file format.
This schema can be used by editors and IDEs to provide autocompletion and validation.`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output file path, use '-' for stdout",
						Value:   "panix-schema.yaml",
					},
				},
				Action: runSchemaCommand,
			},
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}

// Helpers

func runTui(ctx context.Context, flags config_flags.Flags, commandPhases []phases.Phase, fn ...func(conf *config.Config)) error {
	conf, err := config.LoadConfig(flags, commandPhases)
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	for _, callback := range fn {
		callback(conf)
	}

	return tui.NewTui(ctx, conf)
}

func runSchemaCommand(ctx context.Context, cmd *cli.Command) error {
	outputPath := cmd.String("output")

	// Generate the schema
	schemaYAML, err := config_schema.GenerateYAML()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Output the schema
	if outputPath == "-" {
		fmt.Print(string(schemaYAML))
	} else {
		// Ensure directory exists
		dir := filepath.Dir(outputPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		if err := os.WriteFile(outputPath, schemaYAML, 0644); err != nil {
			return fmt.Errorf("failed to write schema to %s: %w", outputPath, err)
		}

		fmt.Printf("Schema written to: %s\n", outputPath)
	}

	return nil
}
