package tui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (m *model) PrintStatusPhaseMachineTable() string {
	if m.state.Conf.Global.DryRun {
		if m.state.Conf.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return "No table in dryRun"
	}

	var builder strings.Builder

	// Header for the log view
	builder.WriteString("\n" + lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#b1f132ff")).
		Render("=== Stats table ===\n"))

	// Use provided width, with reasonable bounds and accounting for borders
	usableWidth := m.modelView.width - 4 // Account for borders and padding
	if usableWidth < 60 {
		usableWidth = 60 // absolute minimum
	} else if usableWidth > 200 {
		usableWidth = 200 // reasonable maximum
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("", "", "FLAKE", "CONFIGURATION", "MACHINE", "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR").
		Width(usableWidth)

	m.state.ExpandFlakeConfigurationMachine(func(i int, flakeName, configurationName string, configuration *config.Configuration, machineName url.URL, machine *config.Machine) {
		xpath := flakeName + configurationName + machineName.String()

		ps := machine.Phases.Status
		log := machine.Logs[workflow_definition.PhaseStatus]

		err := ""
		errPtr := log.TimeAndState.GetTimeAndState().Error
		if errPtr != nil {
			err = errPtr.Error()
		}

		t.Row(
			fmt.Sprintf("%d", i),
			m.getStatusIcon(ps, xpath, log),
			flakeName,
			configurationName,
			strings.TrimPrefix(machineName.String(), "ssh://"),
			//machine.Ssh.Alias,
			m.getStatusText(ps, xpath, log),
			ps.CurrentGeneration,
			ps.LastDeployTime,
			err,
		)
	})

	builder.WriteString("\n" + t.String() + "\n")

	return builder.String()
}

func (m *model) getStatusIcon(ps *config.PhaseStatus, xpath string, log *config.Log) string {
	tas := log.TimeAndState.GetTimeAndState()
	if !tas.Finished {
		return m.modelView.spinners.GetOrCreateSpinner(xpath).View()
	}
	if !ps.Reachable {
		return "🔴"
	}
	if !ps.SSHConnectable {
		return "🟡"
	}
	if !ps.Bootstrapped {
		return "🟠"
	}
	return "✅"
}

func (m *model) getStatusText(ps *config.PhaseStatus, xpath string, log *config.Log) string {
	tas := log.TimeAndState.GetTimeAndState()
	if !tas.Finished {
		return m.modelView.spinners.GetOrCreateSpinner(xpath).View()
	}
	if !ps.Reachable {
		return "UNREACHABLE"
	}
	if !ps.SSHConnectable {
		return "SSH_FAILED"
	}
	if !ps.Bootstrapped {
		return "NOT_BOOTSTRAPPED"
	}
	return "OK"
}
