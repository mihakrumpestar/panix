package config

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

func (s *MachineMetadataStatus) getStatusIcon() string {
	if !s.Reachable {
		return "🔴"
	}
	if !s.SSHConnectable {
		return "🟡"
	}
	if !s.Bootstrapped {
		return "🟠"
	}
	return "✅"
}

func (s *MachineMetadataStatus) getStatusText() string {
	if !s.Reachable {
		return "UNREACHABLE"
	}
	if !s.SSHConnectable {
		return "SSH_FAILED"
	}
	if !s.Bootstrapped {
		return "NOT_BOOTSTRAPPED"
	}
	return "OK"
}

func (c *Config) PrintMachineMetadataStatusTable() {
	if c.Global.DryRun {
		if c.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return
	}

	machines, err := c.traverseMachines()
	if err != nil {
		fmt.Println(err)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("", "FLAKE", "CONFIGURATION", "MACHINE", "HOST", "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	for _, machine := range machines {
		errorMsg := ""
		if machine.Metadata.Status.Error != nil {
			errorMsg = machine.Metadata.Status.Error.Error()
			if len(errorMsg) > 50 {
				errorMsg = errorMsg[:47] + "..."
			}
		}

		table.Append(
			machine.Metadata.Status.getStatusIcon(),
			machine.Metadata.FlakeName,
			machine.Metadata.ConfigurationName,
			machine.Metadata.MachineName,
			machine.Ssh.Alias,
			machine.Metadata.Status.getStatusText(),
			machine.Metadata.Status.CurrentGeneration,
			machine.Metadata.Status.LastDeployTime,
			errorMsg,
		)
	}

	table.Render()
}

func (c *Config) traverseMachines() ([]*Machine, error) {
	machines := make([]*Machine, 0)

	for _, flake := range c.Flakes {
		for _, configuration := range flake.Configurations {
			for _, machine := range configuration.Machines {
				machines = append(machines, machine)
			}
		}
	}

	if len(machines) == 0 {
		return nil, fmt.Errorf("No machines to traverse")
	}

	return machines, nil
}
