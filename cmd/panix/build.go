package panix

import (
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all selected closures",
	Long:  `Build compiles the NixOS configurations for all selected machines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conf := ConfFromContext(ctx)

		phases := []phases.Phase{
			phases.Build,
		}

		workflowExec, err := workflow.NewWorkflow(ctx, conf, phases)
		if err != nil {
			return err
		}

		return RunTUI(workflowExec)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
