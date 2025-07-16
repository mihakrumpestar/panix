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
		executor := workflow.NewWorkflowExecutor(cmd.Context(), &config.C)

		//tui.NewTui()
		err := executor.ExecuteStatusPhase()
		//<-executor.Done()

		for i := range executor.Done() {
			fmt.Println(i)
		}

		executor.PrintStatusPhaseMachineTable()

		return err
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
