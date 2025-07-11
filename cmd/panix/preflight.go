package panix

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Check host reachability and bootstrap status",
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}

		fmt.Printf("Checking status of %d machines...\n", len(machines))

		executor := workflow.NewWorkflowExecutor(cmd.Context(), &config.C.Global, machines)
		opts := workflow.WorkflowOptions{
			DryRun:  config.C.Global.DryRun,
			Verbose: config.C.Global.Verbose,
			Phases:  []workflow.WorkflowPhase{workflow.PhasePreflight},
		}

		return executor.Execute(opts)
	},
}

func init() {
	rootCmd.AddCommand(preflightCmd)
}
