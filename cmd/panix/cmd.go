package panix

import (
	"fmt"
	"os"
	"runtime"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "panix",
	Short: "Universal NixOS Deployment Tool",
	Long: `Panix - Universal NixOS Deployment Tool
	// TODO: add proper description
	`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	var configFile string

	// Config file flag
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "panix.yml", "config file")

	// Filter flags
	rootCmd.PersistentFlags().StringSlice("flakes", nil, "a list of flakes to deploy")
	rootCmd.PersistentFlags().StringSlice("configurations", nil, "a list of configurations to deploy")
	rootCmd.PersistentFlags().StringSlice("machines", nil, "a list of machines to deploy")
	rootCmd.PersistentFlags().StringSlice("tags", nil, "filter machines by tags")

	// Global flags
	hostname, _ := os.Hostname()
	concurrency := runtime.GOMAXPROCS(0)

	rootCmd.PersistentFlags().Bool("require-all", false, "abort & rollback if any host fails")
	rootCmd.PersistentFlags().Bool("auto-bootstrap", false, "automatically bootstrap uninitialized hosts")
	rootCmd.PersistentFlags().String("local-machine", hostname, "machine name that is local (won't use ssh to connect to it)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "show what would be done without executing")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug output")
	rootCmd.PersistentFlags().Int("concurrency", concurrency, "number of concurrent operations")
	rootCmd.PersistentFlags().Int("timeout", 7200, "timeout for operations in seconds")

	cobra.OnInitialize(func() { initConfig(configFile) })
}

func initConfig(configFile string) {
	_, err := config.LoadConfig(configFile, rootCmd.Flags())
	if err != nil {
		err = fmt.Errorf("failed to load config: %w", err)
		fmt.Println(err)
		os.Exit(1)
	}
}
