package cmd

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/sshclient"
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

		if config.C.Global.DryRun {
			fmt.Println("DRY RUN: Would check status of the following machines:")
			for _, machine := range machines {
				fmt.Printf("  - %s (%s@%s:%d)\n", machine.Name, machine.Ssh.User, machine.Ssh.Host, machine.Ssh.Port)
			}
			return nil
		}

		fmt.Printf("Checking status of %d machines...\n", len(machines))

		sshClients, err := sshclient.New(config.C, machines)
		if err != nil {
			return fmt.Errorf("failed to initialize SSH clients: %w", err)
		}

		statuses := sshClients.GetAllMachineStatuses(machines)

		fmt.Println()
		sshclient.PrintStatusTable(statuses)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
