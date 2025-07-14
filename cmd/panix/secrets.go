package panix

import (
	"github.com/spf13/cobra"
)

// secretsCmd represents the secrets command
var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Deploy secrets to all machines",
	Long:  `Secrets deploys encrypted secrets and credentials to all selected machines.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		return nil
	},
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}
