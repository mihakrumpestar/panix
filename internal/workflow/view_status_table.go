package workflow

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (state *WorkflowState) PrintStatusPhaseMachineTable() (*table.Table, error) {
	if state.Conf.Global.DryRun {
		if state.Conf.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return nil, nil
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("INDEX", "ICON", "FLAKE", "CONFIGURATION", "MACHINE", "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	state.expandFlakeConfigurationMachine(func(i int, flakeName, configurationName string, configuration *config.Configuration, machineName url.URL, machine *config.Machine) {
		ps := machine.Phases.Status
		log := machine.Logs[workflow_definition.PhaseStatus]

		err := ""
		errPtr := log.TimeAndState.GetTimeAndState().Error
		if errPtr != nil {
			err = errPtr.Error()
		}

		t.Row(
			fmt.Sprintf("%d", i),
			getStatusIcon(ps, log),
			flakeName,
			configurationName,
			strings.TrimPrefix(machineName.String(), "ssh://"),
			//machine.Ssh.Alias,
			getStatusText(ps),
			ps.CurrentGeneration,
			ps.LastDeployTime,
			err,
		)
	})

	return t, nil
}

func getStatusIcon(ps *config.PhaseStatus, log *config.Log) string {
	if !log.TimeAndState.GetTimeAndState().Finished {
		return spinner.New().View()
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

func getStatusText(ps *config.PhaseStatus) string {
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
