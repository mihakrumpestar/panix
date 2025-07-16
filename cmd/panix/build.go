package panix

import (
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all selected closures",
	Long:  `Build compiles the NixOS configurations for all selected machines without deploying them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		/*
			wexc, err := workflow.NewWorkflowExecutor(cmd.Context(), &config.C)
			if err != nil {
				return err
			}

			_, err = wexc.ExecuteBuildPhase(nil)
			if err != nil {
				return err
			}
		*/

		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
