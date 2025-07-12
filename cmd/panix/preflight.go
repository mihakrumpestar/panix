package panix

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Check host reachability and bootstrap status",
	RunE: func(cmd *cobra.Command, args []string) error {
		phases := []workflow_definition.WorkflowPhase{workflow_definition.PhasePreflight}
		executor, err := workflow.NewWorkflowExecutor(cmd.Context(), &config.C, phases)
		if err != nil {
			return err
		}

		_, err = executor.Execute()
		return err
	},
}

func init() {
	rootCmd.AddCommand(preflightCmd)
}
