package panix

import (
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all selected closures",
	Long:  `Build compiles the NixOS configurations for all selected machines without deploying them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		phases := []workflow_definition.WorkflowPhase{workflow_definition.PhaseBuild}
		workflowExec, err := workflow.NewWorkflow(cmd.Context(), phases)
		if err != nil {
			return err
		}

		return tui.NewTui(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
