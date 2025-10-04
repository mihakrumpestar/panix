package panix

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/spf13/cobra"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "panix",
	Short: "Universal NixOS Deployment Tool",
	Long: `Panix - Universal NixOS Deployment Tool
	// TODO: add proper description
	`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		conf, err := config.LoadConfig(configFile, cmd.Flags())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		ctxWithConf := context.WithValue(cmd.Context(), config.ContextConfigKey, conf)

		cmd.SetContext(ctxWithConf)

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
	// Config file flag
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "panix.yml", "config file")

	// Global flags

	// Filter flags
	rootCmd.PersistentFlags().StringSlice("filters.flakes", nil, "a list of flakes to deploy")
	rootCmd.PersistentFlags().StringSlice("filters.configurations", nil, "a list of configurations to deploy")
	rootCmd.PersistentFlags().StringSlice("filters.machines", nil, "a list of machines to deploy")
	rootCmd.PersistentFlags().StringSlice("filters.tags", nil, "filter machines by tags")

	hostname, _ := os.Hostname()
	concurrency := runtime.GOMAXPROCS(0)

	// Bootstrap
	rootCmd.PersistentFlags().Bool("bootstrap.disableAuto", false, "disable automatic bootstrap (even if target machine does not have NixOS installed)")
	rootCmd.PersistentFlags().Bool("bootstrap.disableDisko", false, "disables building, transfer and bootstrap with disko tool")

	// Others
	rootCmd.PersistentFlags().Bool("requireAllSuccess", false, "abort & rollback if any host fails")
	rootCmd.PersistentFlags().String("overrideLocalMachine", hostname, "hostname of the machine that is local (won't use ssh to connect to it)")
	rootCmd.PersistentFlags().Bool("dryRun", false, "show what would be done without executing")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug output")
	rootCmd.PersistentFlags().Int("concurrency", concurrency, "number of concurrent operations")
	rootCmd.PersistentFlags().Int("timeout", 7200, "timeout for TUI in seconds, default is 1 hour")
}
