package panix

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/spf13/cobra"
	"github.com/yassinebenaid/godump"
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
		executor, err := workflow.NewWorkflow(cmd.Context())
		if err != nil {
			return err
		}

		//go func() {
		_ = executor.ExecuteStatusPhase()
		fmt.Println("ExecuteStatusPhase finished")
		godump.Dump(executor.State())
		//}()

		//return tui.NewTui(executor.State(), executor.GetChannel(), executor.Cancel())

		//godump.Dump(executor.Metadatas())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
