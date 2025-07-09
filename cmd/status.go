package cmd

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Query last deployment status per host",
	Long: `Status queries and displays the current deployment status of all configured machines.
This includes:
- Current NixOS generation
- Last deployment time
- SSH connectivity status
- Bootstrap status (initialized/uninitialized)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}

		fmt.Printf("Checking status of %d machines...\n", len(machines))

		if config.C.Global.DryRun {
			fmt.Println("DRY RUN: Would check status of the following machines:")
			for _, machine := range machines {
				fmt.Printf("  - %s (%s@%s:%d)\n", machine.Name, machine.User, machine.Host, machine.Port)
			}
			return nil
		}

		// TODO: Implement actual status checking logic
		fmt.Println("Status functionality not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
