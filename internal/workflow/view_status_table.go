package workflow

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
)

func (meta *Metadatas) PrintStatusPhaseMachineTable() (*table.Table, error) {
	if config.C.Global.DryRun {
		if config.C.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return nil, nil
	}

	if meta == nil {
		return nil, fmt.Errorf("meta is nil")
	}

	if meta.StatusPhaseMeta == nil {
		return nil, fmt.Errorf("meta.StatusPhaseMeta is nil")
	}

	if meta.StatusPhaseMeta.MachineStatuses == nil {
		return nil, fmt.Errorf("meta.StatusPhaseMeta.MachineStatuses is nil")
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("INDEX", "ICON", "FLAKE", "CONFIGURATION", "MACHINE" /* "HOST", */, "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	for i, sm := range meta.StatusPhaseMeta.MachineStatuses {
		err := ""
		if sm.BaseMeta.Error != nil {
			err = sm.BaseMeta.Error.Error()
		}

		t.Row(
			fmt.Sprintf("%d", i),
			sm.getStatusIcon(),
			sm.BaseMeta.FlakeName,
			sm.BaseMeta.ConfigurationName,
			sm.BaseMeta.MachineName.String(),
			//machine.Ssh.Alias,
			sm.getStatusText(),
			sm.CurrentGeneration,
			sm.LastDeployTime,
			err,
		)
	}

	return t, nil
}

func (s *StatusMachineMeta) getStatusIcon() string {
	if !s.BaseMeta.EndTime.IsZero() {
		return spinner.New().View()
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

func (s *StatusMachineMeta) getStatusText() string {
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
