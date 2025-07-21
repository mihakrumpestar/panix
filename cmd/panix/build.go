package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all selected closures",
	Long:  `Build compiles the NixOS configurations for all selected machines without deploying them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		executor, err := workflow.NewWorkflow(cmd.Context())
		if err != nil {
			return err
		}

		go func() {
			_ = executor.ExecuteBuildPhase()
		}()

		return tui.NewTui(executor.Ctx(), executor.State(), executor.GetChannel(), executor.Cancel())
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
