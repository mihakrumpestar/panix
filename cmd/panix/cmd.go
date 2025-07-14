package panix

import (
	"fmt"
	"os"
	"runtime"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "panix",
	Short: "Universal NixOS Deployment Tool",
	Long:  `Panix - Universal NixOS Deployment Tool // TODO: add proper description`,
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
	rootCmd.PersistentFlags().Bool("require-all", false, "abort & rollback if any host fails")
	rootCmd.PersistentFlags().Bool("auto-bootstrap", false, "automatically bootstrap uninitialized hosts")
	rootCmd.PersistentFlags().Bool("dry-run", false, "show what would be done without executing")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug output")
	rootCmd.PersistentFlags().Int("concurrency", runtime.NumCPU(), "number of concurrent operations")
	rootCmd.PersistentFlags().Int("timeout", 7200, "timeout for operations in seconds")

	// Bind flags to viper
	viper.BindPFlag("global.filters.flakes", rootCmd.PersistentFlags().Lookup("flakes"))
	viper.BindPFlag("global.filters.configurations", rootCmd.PersistentFlags().Lookup("configurations"))
	viper.BindPFlag("global.filters.machines", rootCmd.PersistentFlags().Lookup("machines"))
	viper.BindPFlag("global.filters.tags", rootCmd.PersistentFlags().Lookup("tags"))
	viper.BindPFlag("global.requireAllSuccess", rootCmd.PersistentFlags().Lookup("require-all"))
	viper.BindPFlag("global.autoBootstrap", rootCmd.PersistentFlags().Lookup("auto-bootstrap"))
	viper.BindPFlag("global.dryRun", rootCmd.PersistentFlags().Lookup("dry-run"))
	viper.BindPFlag("global.verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("global.debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("global.concurrency", rootCmd.PersistentFlags().Lookup("concurrency"))
	viper.BindPFlag("global.timeout", rootCmd.PersistentFlags().Lookup("timeout"))

	cobra.OnInitialize(func() { initConfig(configFile) })
}

func initConfig(configFile string) {
	_, err := config.LoadConfig(viper.GetViper(), configFile)
	if err != nil {
		err = fmt.Errorf("failed to load config: %w", err)
		fmt.Println(err)
		os.Exit(1)
	}
}
