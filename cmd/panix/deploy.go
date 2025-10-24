package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
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
		ctx := cmd.Context()
		conf := ConfFromContext(ctx)

		phases := phases.PhasesInOrder()

		workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
		if err != nil {
			return err
		}

		return tui.NewTui(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
