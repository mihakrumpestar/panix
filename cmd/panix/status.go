package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Query machine status per host",
	Long: `Status queries and displays the current machine status of all configured hosts.
This includes:
- Current NixOS generation
- Last deployment time
- SSH connectivity status
- Bootstrap status (initialized/uninitialized)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conf := ConfFromContext(ctx)

		phases := []phases.Phase{
			phases.Inspect,
		}

		workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
		if err != nil {
			return err
		}

		return tui.NewTui(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
