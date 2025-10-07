package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/spf13/cobra"
)

// secretsCmd represents the secrets command
var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Deploy secrets to all machines",
	Long:  `Secrets deploys encrypted secrets and credentials to all selected machines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conf := ConfFromContext(ctx)

		phases := []phases.Phase{
			phases.Status,
			phases.Secrets,
		}

		workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
		if err != nil {
			return err
		}

		return tui.NewTui(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}
