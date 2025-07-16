package panix

import (
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
		/*
			wexc, err := workflow.NewWorkflowExecutor(cmd.Context(), &config.C)
			if err != nil {
				return err
			}
				statusMetadatas := wexc.ExecuteStatusPhase()



				configurationMetadata, err := wexc.ExecuteBuildPhase(statusMetadata)
				if err != nil {
					return err
				}

				fmt.Println(configurationMetadata)

			fmt.Println(wexc)
		*/
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
