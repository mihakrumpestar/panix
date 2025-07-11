package panix

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
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
		var targetMachines []string
		if len(args) > 0 {
			targetMachines = args
		}

		machines := getFilteredMachines()
		if len(machines) == 0 {
			return fmt.Errorf("no machines match the specified filters")
		}

		// Filter machines by name if specific machines were requested
		if len(targetMachines) > 0 {
			var filtered []config.MachineConfig
			for _, machine := range machines {
				for _, target := range targetMachines {
					if machine.Name == target {
						filtered = append(filtered, machine)
						break
					}
				}
			}
			machines = filtered
		}

		if len(machines) == 0 {
			return fmt.Errorf("no machines found matching the specified names")
		}

		fmt.Printf("Rolling back %d machines...\n", len(machines))

		if config.C.Global.DryRun {
			fmt.Println("DRY RUN: Would rollback the following machines:")
			for _, machine := range machines {
				if toGeneration > 0 {
					fmt.Printf("  - %s to generation %d\n", machine.Name, toGeneration)
				} else {
					fmt.Printf("  - %s to previous generation\n", machine.Name)
				}
			}
			return nil
		}

		// TODO: Implement actual rollback logic
		fmt.Println("Rollback functionality not yet implemented")
		return nil
	},
}

func init() {
	rollbackCmd.Flags().IntVar(&toGeneration, "to-generation", 0, "rollback to specific generation number")
	rootCmd.AddCommand(rollbackCmd)
}
