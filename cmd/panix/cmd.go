package panix

import (
	"context"
	"fmt"
	"os"

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

	// Bootstrap
	rootCmd.PersistentFlags().Bool("bootstrap.disableAuto", false, "disable automatic bootstrap (even if target machine does not have NixOS installed)")
	rootCmd.PersistentFlags().Bool("bootstrap.disableDisko", false, "disables building, transfer and bootstrap of disko tool")
	rootCmd.PersistentFlags().Bool("bootstrap.only", false, "only initializes uninitialized machines")

	// Others
	rootCmd.PersistentFlags().StringSlice("tags", nil, "filter machines by tags (flakes, configurations and machine names are already registered as tags, children inherit all parent tags)")
	rootCmd.PersistentFlags().Bool("requireAllSuccess", false, "abort & rollback if any task fails, primarily for CI/CD")
	hostname, _ := os.Hostname()
	rootCmd.PersistentFlags().String("overrideLocalMachine", hostname, "hostname of the machine that is local (won't use ssh to connect to it)")
	rootCmd.PersistentFlags().Bool("dryRun", false, "show what would be done without executing")
	rootCmd.PersistentFlags().Bool("dryRunWithStatus", false, "show what would be done without executing, but with real status query")
	rootCmd.PersistentFlags().Int("timeout", 7200, "timeout for TUI in seconds, default is 1 hour")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug output")
	rootCmd.PersistentFlags().String("cpuprofile", "", "cpu profiling to file")
}

// Helpers

func ConfFromContext(ctx context.Context) *config.Config {
	conf := ctx.Value(config.ContextConfigKey).(*config.Config)

	if conf == nil {
		panic(fmt.Errorf("internal error: %s key is nil/empty in cmd context", config.ContextConfigKey))
	}

	return conf
}
