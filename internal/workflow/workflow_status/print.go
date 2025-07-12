package workflow_status

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

func (s *MachineStatus) getStatusIcon() string {
	if s.Error != nil {
		return "❌"
	}
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

func (s *MachineStatus) getStatusText() string {
	if s.Error != nil {
		return fmt.Sprintf("ERROR: %s", s.Error.Error())
	}
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

func PrintStatusTable(statuses []*MachineStatus) {
	if len(statuses) == 0 {
		fmt.Println("No machines to display")
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("", "MACHINE", "HOST", "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	for _, status := range statuses {
		errorMsg := ""
		if status.Error != nil {
			errorMsg = status.Error.Error()
			if len(errorMsg) > 50 {
				errorMsg = errorMsg[:47] + "..."
			}
		}

		table.Append(
			status.getStatusIcon(),
			status.Name,
			status.Machine.Ssh.Host,
			status.getStatusText(),
			status.CurrentGeneration,
			status.LastDeployTime,
			errorMsg,
		)
	}

	table.Render()
}
