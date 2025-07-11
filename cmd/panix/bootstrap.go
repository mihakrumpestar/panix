package panix

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
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

		if config.C.Global.DryRun {
			fmt.Println("DRY RUN: Would bootstrap the following machines:")
			for _, machine := range machines {
				if machine.Bootstrap != nil {
					fmt.Printf("  - %s (%s@%s:%d)\n", machine.Name, machine.Ssh.User, machine.Ssh.Host, machine.Ssh.Port)
				} else {
					fmt.Printf("  - %s (%s@%s:%d) - auto-detect bootstrap\n", machine.Name, machine.Ssh.User, machine.Ssh.Host, machine.Ssh.Port)
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
