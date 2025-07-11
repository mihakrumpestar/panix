package panix

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Query last deployment status per host",
	Long: `Status queries and displays the current deployment status of all configured machines.
This includes:
- Current NixOS generation
- Last deployment time
- SSH connectivity status
- Bootstrap status (initialized/uninitialized)`,
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
			Phases:  []workflow.WorkflowPhase{workflow.PhaseStatus},
		}

		return executor.Execute(opts)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
