package cmd

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
		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}

		fmt.Printf("Bootstrapping %d machines...\n", len(machines))

		if dryRun {
			fmt.Println("DRY RUN: Would bootstrap the following machines:")
			for _, machine := range machines {
				if machine.Bootstrap != nil {
					fmt.Printf("  - %s (%s@%s:%d)\n", machine.Name, machine.User, machine.Host, machine.Port)
				} else {
					fmt.Printf("  - %s (%s@%s:%d) - auto-detect bootstrap\n", machine.Name, machine.User, machine.Host, machine.Port)
				}
			}
			return nil
		}

		// TODO: Implement actual bootstrap logic
		fmt.Println("Bootstrap functionality not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}
