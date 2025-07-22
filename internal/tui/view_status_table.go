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
	state := m.workflow.State()

	if state.Conf.Global.DryRun {
		if state.Conf.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return "No table in dryRun"
	}

	var builder strings.Builder

	// Use color scheme from model
	colors := m.modelView.colors

	// Header for the log view
	builder.WriteString("\n" + colors.HeaderTitle.Render("=== Stats table ===\n"))

	// Use provided width, with reasonable bounds and accounting for borders
	usableWidth := m.modelView.width - 4 // Account for borders and padding
	if usableWidth < 60 {
		usableWidth = 60 // absolute minimum
	}

	// Header text lengths including icons
	flakeHeader := string(colors.IconFlake) + " FLAKE"
	configurationHeader := string(colors.IconConfiguration) + " CONFIGURATION"
	machineHeader := string(colors.IconMachine) + " MACHINE"

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(colors.TableBorder).
		Headers("", "", flakeHeader, configurationHeader, machineHeader, "STATUS", "GENERATION", "DEPLOY", "ERROR").
		Width(usableWidth).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				return colors.TableRow
			}

			switch col {
			case 0: // Index
				return colors.TableRow.Width(3).Align(lipgloss.Center)
			case 1: // Status icon
				return colors.TableRow.Width(3).Align(lipgloss.Center)
			case 2: // FLAKE
				return colors.Flake
			case 3: // CONFIG
				return colors.Configuration
			case 4: // MACHINE
				return colors.Machine
			case 5: // Status text
				return colors.TableRow.Width(lipgloss.Width("NOT_BOOTSTRAPPED"))
			case 6: // Generation
				return colors.TableRow.Width(lipgloss.Width("GENERATION"))
			case 7: // Last deploy
				return colors.TableRow.Width(lipgloss.Width("2025-07-22 08:12:19"))
			case 8: // ERROR
				return colors.TableRow.MaxWidth(1000)
			default:
				return colors.TableRow
			}
		})

	// Track previous values to implement row spanning
	var prevFlakeName, prevConfigName string

	state.ExpandFlakeConfigurationMachine(func(i int, flakeName, configurationName string, configuration *config.Configuration, machineName url.URL, machine *config.Machine) {
		xpath := flakeName + configurationName + machineName.String()

		ps := machine.Phases.Status
		log := machine.Logs.SafeGet(workflow_definition.PhaseStatus)

		err := ""
		errPtr := log.TimeAndState.GetTimeAndState().Error
		if errPtr != nil {
			err = errPtr.Error()
		}

		// Determine if we should show flake name (only on first occurrence)
		showFlake := flakeName != prevFlakeName
		if showFlake {
			prevFlakeName = flakeName
		}

		// Determine if we should show config name (only on first occurrence of each config within a flake)
		showConfig := configurationName != prevConfigName || flakeName != prevFlakeName
		if showConfig {
			prevConfigName = configurationName
		}

		// Get display values (empty for spanning)
		flakeDisplay := ""
		if showFlake {
			flakeDisplay = flakeName
		}

		configDisplay := ""
		if showConfig {
			configDisplay = configurationName
		}

		t.Row(
			fmt.Sprintf("%d", i),
			m.getStatusIcon(ps, xpath, log),
			flakeDisplay,
			configDisplay,
			strings.TrimPrefix(machineName.String(), "ssh://"),
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
	m.modelView.spinners.RemoveIfExistsSpinner(xpath)
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
	m.modelView.spinners.RemoveIfExistsSpinner(xpath)
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
