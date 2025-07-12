package panix

import (
	"fmt"
	"os"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "panix",
	Short: "Universal NixOS Deployment Tool",
	Long:  `Panix - Universal NixOS Deployment Tool // TODO:`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is panix.yml)")
	rootCmd.PersistentFlags().StringSliceVar(&config.C.Global.Filters.Flakes, "flakes", nil, "a list of flakes to deploy")
	rootCmd.PersistentFlags().StringSliceVar(&config.C.Global.Filters.Configurations, "configurations", nil, "a list of configurations to deploy")
	rootCmd.PersistentFlags().StringSliceVar(&config.C.Global.Filters.Machines, "machines", nil, "a list of machines to deploy")
	rootCmd.PersistentFlags().StringSliceVar(&config.C.Global.Filters.Tags, "tags", nil, "filter machines by tags (e.g., +prod,-canary)")
	rootCmd.PersistentFlags().BoolVar(&config.C.Global.RequireAllSuccess, "require-all", false, "abort & rollback if any host fails")
	rootCmd.PersistentFlags().BoolVar(&config.C.Global.AutoBootstrap, "auto-bootstrap", false, "automatically bootstrap uninitialized hosts")
	rootCmd.PersistentFlags().BoolVar(&config.C.Global.DryRun, "dry-run", false, "show what would be done without executing")
	rootCmd.PersistentFlags().BoolVarP(&config.C.Global.Verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().IntVar(&config.C.Global.Concurrency, "concurrency", 4, "number of concurrent operations")
	rootCmd.PersistentFlags().DurationVar(&config.C.Global.Timeout, "timeout", 300, "timeout for operations in seconds")

	// rootCmd.MarkFlagsMutuallyExclusive("require-all", "continue-on-error")
}

func initConfig() {
	_, err := config.LoadConfig(cfgFile)
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}
}
