package panix

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	toGeneration int
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [machine...]",
	Short: "Revert host(s) to previous generation",
	Long: `Rollback reverts one or more machines to a previous NixOS generation.
If no machines are specified, all configured machines will be rolled back.
Use --to-generation to specify a specific generation number.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// TODO: Implement actual rollback logic
		fmt.Println("Rollback functionality not yet implemented")
		return nil
	},
}

func init() {
	rollbackCmd.Flags().IntVar(&toGeneration, "to-generation", 0, "rollback to specific generation number")
	rootCmd.AddCommand(rollbackCmd)
}
