package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Do full workflow (preflight→bootstrap→secrets→build→push→activate)",
	Long: `Deploy performs the complete deployment workflow:
1. Preflight checks (SSH connectivity, bootstrap status)
2. Bootstrap uninitialized machines (if needed)
3. Deploy secrets to all machines
4. Build NixOS configurations
5. Transfer closures to target machines
6. Activate new configurations

This is the main command for deploying NixOS configurations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		phases := []workflow_definition.WorkflowPhase{
			workflow_definition.PhaseStatus,
			workflow_definition.PhaseBuild,
			workflow_definition.PhaseTransfer,
			workflow_definition.PhaseSecrets,
			workflow_definition.PhaseActivate,
		}

		workflowExec, err := workflow.NewWorkflow(cmd.Context(), phases)
		if err != nil {
			return err
		}

		return tui.NewTui(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
