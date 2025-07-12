package panix

import (
	"fmt"

	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Explicit bootstrap phase",
	Long: `Bootstrap initializes uninitialized machines using nixos-anywhere.
This command will:
- Check which machines need bootstrapping
- Copy pre-bootstrap secrets if configured
- Run nixos-anywhere to install NixOS
- Wait for machines to reboot and become available`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// TODO: Implement actual bootstrap logic
		fmt.Println("Bootstrap functionality not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}
