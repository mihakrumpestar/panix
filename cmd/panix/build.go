package panix

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all selected closures",
	Long:  `Build compiles the NixOS configurations for all selected machines without deploying them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}

		fmt.Printf("Building configurations for %d machines...\n", len(machines))

		executor := workflow.NewWorkflowExecutor(cmd.Context(), &config.C.Global, machines)
		opts := workflow.WorkflowOptions{
			DryRun:  config.C.Global.DryRun,
			Verbose: config.C.Global.Verbose,
			Phases:  []workflow.WorkflowPhase{workflow.PhaseBuild},
		}

		return executor.Execute(opts)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
