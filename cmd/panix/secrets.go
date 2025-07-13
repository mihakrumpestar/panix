package panix

import (
	"github.com/mihakrumpestar/panix/internal/config"
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
		phases := []workflow_definition.WorkflowPhase{workflow_definition.PhaseSecrets}
		executor, err := workflow.NewWorkflowExecutor(cmd.Context(), &config.C, phases)
		if err != nil {
			return err
		}

		return executor.Execute()
	},
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}
