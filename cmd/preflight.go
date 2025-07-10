package cmd

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/sshclient"
	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Check host reachability and bootstrap status",
	RunE: func(cmd *cobra.Command, args []string) error {
		machines := getFilteredMachines()
		if len(machines) == 0 {
			fmt.Println("No machines match the specified filters")
			return nil
		}

		client, err := sshclient.New(config.C, machines)
		if err != nil {
			return fmt.Errorf("SSH client initialization failed: %w", err)
		}

		for _, m := range machines {
			status, err := client.CheckHost(m, sshclient.CheckMinimal)
			if err != nil {
				fmt.Printf("Machine %s check failed: %v\n", m.Name, err)
				if config.C.Global.RequireAllSuccess {
					return fmt.Errorf("aborting due to failure on %s", m.Name)
				}
				continue
			}
			if status.AllOk {
				fmt.Printf("Machine %s is bootstrapped\n", m.Name)
			} else {
				fmt.Printf("Machine %s not bootstrapped\n", m.Name)
				if !config.C.Global.AutoBootstrap {
					fmt.Printf("Init machine %s? [Y/n]: ", m.Name)
					var resp string
					fmt.Scanln(&resp)
					if resp != "Y" && resp != "y" && resp != "" {
						if config.C.Global.RequireAllSuccess {
							return fmt.Errorf("aborting due to declination on %s", m.Name)
						}
					}
				}
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(preflightCmd)
}
