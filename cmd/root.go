package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile         string
	tags            []string
	requireAll      bool
	continueOnError bool
	autoBootstrap   bool
	noAutoBootstrap bool
	dryRun          bool
	verbose         bool
	concurrency     int
	timeout         int

	cfg *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "panix",
	Short: "Universal NixOS Deployment Tool",
	Long: `Panix – Universal NixOS Deployment Tool

A Go-based, flakes-aware, SSH-first deployer combining:
• Two-mode bootstrapping (explicit via config, or implicit with detection+prompt)
• TOML inventory with per-machine secrets (files or dirs auto-detected)
• Full preflight checks (reachability, init-status, tag filtering)
• Parallel builds, content-addressed transfers, atomic activation & rollbacks
• Optional "all-or-nothing" vs "best-effort" failure policies
• Native nixos-anywhere reuse, minimal on-disk state, plugin hooks`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.LoadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if requireAll {
			cfg.Global.RequireAllSuccess = true
		}
		if continueOnError {
			cfg.Global.RequireAllSuccess = false
		}
		if autoBootstrap {
			cfg.Global.AutoBootstrap = true
		}
		if noAutoBootstrap {
			cfg.Global.AutoBootstrap = false
		}
		if concurrency > 0 {
			cfg.Global.Concurrency = concurrency
		}
		if timeout > 0 {
			cfg.Global.Timeout = timeout
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is panix.toml)")
	rootCmd.PersistentFlags().StringSliceVar(&tags, "tags", nil, "filter machines by tags (e.g., +prod,-canary)")
	rootCmd.PersistentFlags().BoolVar(&requireAll, "require-all", false, "abort & rollback if any host fails")
	rootCmd.PersistentFlags().BoolVar(&continueOnError, "continue-on-error", false, "continue deployment on errors")
	rootCmd.PersistentFlags().BoolVar(&autoBootstrap, "auto-bootstrap", false, "automatically bootstrap uninitialized hosts")
	rootCmd.PersistentFlags().BoolVar(&noAutoBootstrap, "no-auto-bootstrap", false, "disable automatic bootstrapping")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without executing")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 0, "number of concurrent operations")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 0, "timeout for operations in seconds")

	rootCmd.MarkFlagsMutuallyExclusive("require-all", "continue-on-error")
	rootCmd.MarkFlagsMutuallyExclusive("auto-bootstrap", "no-auto-bootstrap")
}

func parseTags(tagStrings []string) []string {
	var result []string
	for _, tagString := range tagStrings {
		tags := strings.Split(tagString, ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				result = append(result, tag)
			}
		}
	}
	return result
}

func getFilteredMachines() []config.MachineConfig {
	filterTags := parseTags(tags)
	return cfg.GetMachinesByTags(filterTags)
}
