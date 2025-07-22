package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
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
		phases := []workflow_definition.WorkflowPhase{workflow_definition.PhaseStatus}
		workflowExec, err := workflow.NewWorkflow(cmd.Context(), phases)
		if err != nil {
			return err
		}

		return tui.NewTui(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
