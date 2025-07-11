package panix

import (
	"fmt"

	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show which hosts would build/bootstrap/deploy",
	Long: `Plan shows which hosts would be affected by the deployment operation
without actually performing any changes. It displays:
- Which hosts would be bootstrapped
- Which hosts would have secrets deployed
- Which hosts would be built and deployed
- Any hosts that would be skipped due to filters or errors`,
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			fmt.Println("No machines match the specified filters")
			return nil
		}

		fmt.Printf("Planning deployment for %d machines:\n", len(machines))

		for _, machine := range machines {
			fmt.Printf("\nMachine: %s\n", machine.Name)
			fmt.Printf("  Host: %s@%s:%d\n", machine.Ssh.User, machine.Ssh.Host, machine.Ssh.Port)
			fmt.Printf("  Flake Output: %s\n", machine.FlakeOutput)
			fmt.Printf("  Tags: %v\n", machine.Tags)

			if machine.Bootstrap != nil {
				fmt.Printf("  Bootstrap: Explicit configuration\n")
			} else {
				fmt.Printf("  Bootstrap: Auto-detect required\n")
			}

			if len(machine.Secrets) > 0 {
				fmt.Printf("  Secrets: %d configured\n", len(machine.Secrets))
			} else {
				fmt.Printf("  Secrets: None\n")
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
}
