package panix

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/tui"
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
		executor := workflow.NewWorkflowExecutor(cmd.Context(), &config.C)

		go func() {
			_ = executor.ExecuteStatusPhase()
		}()

		return tui.NewTui(executor.Metadatas(), executor.GetChannel(), executor.Cancel())

		//godump.Dump(executor.Metadatas())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
