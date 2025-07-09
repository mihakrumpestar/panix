package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all selected closures",
	Long: `Build compiles the NixOS configurations for all selected machines
without deploying them. This is useful for:
- Pre-building configurations to check for errors
- Warming up the Nix store before deployment
- Testing configuration changes without deployment`,
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}

		fmt.Printf("Building configurations for %d machines...\n", len(machines))

		if dryRun {
			fmt.Println("DRY RUN: Would build the following configurations:")
			for _, machine := range machines {
				fmt.Printf("  - %s: %s\n", machine.Name, machine.FlakeOutput)
			}
			return nil
		}

		// TODO: Implement actual build logic
		fmt.Println("Build functionality not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
