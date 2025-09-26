package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/spf13/cobra"
)

// secretsCmd represents the secrets command
var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Deploy secrets to all machines",
	Long:  `Secrets deploys encrypted secrets and credentials to all selected machines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		phases := []workflow_definition.WorkflowPhase{
			workflow_definition.PhaseSecrets,
		}

		workflowExec, err := workflow.NewWorkflow(cmd.Context(), phases)
		if err != nil {
			return err
		}

		return tui.NewTui(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}
