package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (m *model) PrintStatusPhaseMachineTable() string {
	state := m.workflow.State()

	colors := config.DefaultColorScheme()

	if state.Conf.Global.DryRun {
		return "No table in dryRun"
	}

	var builder strings.Builder

	// Header for the log view
	builder.WriteString("\n" + colors.HeaderTitle.Render("=== Stats table ===\n"))

	// Use provided width, with reasonable bounds and accounting for borders
	usableWidth := max(m.modelView.dimensions.width-4, 60)

	// Header text lengths including icons
	flakeHeader := string(colors.IconFlake) + " FLAKE"
	configurationHeader := string(colors.IconConfiguration) + " CONFIGURATION"
	machineHeader := string(colors.IconMachine) + " MACHINE"

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(colors.TableBorder).
		Headers("", "", flakeHeader, configurationHeader, machineHeader, "STATUS", "GENERATION", "DATE", "NIXOS", "KERNEL", "STATUS").
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
			case 7: // Date
				return colors.TableRow.Width(lipgloss.Width("2025-07-22 08:12:19"))
			case 8: // Nixos
				return colors.TableRow.Width(lipgloss.Width("25.11.20250624.4b1164c"))
			case 9: // Kernel
				return colors.TableRow.Width(lipgloss.Width("KERNEL"))
			case 10: // STATUS
				return colors.TableRow.MaxWidth(1000)
			default:
				return colors.TableRow
			}
		})

	// Track previous values to implement row spanning
	var prevFlakeName, prevConfigName string

	state.ExpandFlakeConfigurationMachine(false,
		func(i int, flake *config.Flake, configuration *config.Configuration, machine *config.Machine) {

			xpath := flake.Name + configuration.Name + machine.Name

			ps := machine.MetaStatus
			log := machine.Logs.SafeGet(phases.Status)

			status := ""
			lastErr := log.TimeAndState.GetTimeAndState().Error
			if lastErr != nil {
				status = lastErr.Error()
			}

			// Determine if we should show flake name (only on first occurrence)
			showFlake := flake.Name != prevFlakeName
			if showFlake {
				prevFlakeName = flake.Name
			}

			// Determine if we should show config name (only on first occurrence of each config within a flake)
			showConfig := configuration.Name != prevConfigName || flake.Name != prevFlakeName
			if showConfig {
				prevConfigName = configuration.Name
			}

			// Get display values (empty for spanning)
			flakeDisplay := ""
			if showFlake {
				flakeDisplay = flake.Name
			}

			configDisplay := ""
			if showConfig {
				configDisplay = configuration.Name
			}

			t.Row(
				fmt.Sprintf("%d", i),
				m.getStatusIcon(ps, xpath, log),
				flakeDisplay,
				configDisplay,
				machine.Name,
				m.getStatusText(ps, xpath, log),
				fmt.Sprintf("%d", ps.Generation.Load()),
				ps.Date.Load(),
				ps.Nixos.Load(),
				ps.Kernel.Load(),
				status,
			)
		})

	builder.WriteString("\n" + t.String() + "\n")

	return builder.String()
}

func (m *model) getStatusIcon(ps *config.MetaStatus, xpath string, log *config.PhaseLog) string {
	tas := log.TimeAndState.GetTimeAndState()
	if !tas.Finished {
		return m.modelView.spinners.GetOrCreateSpinner(xpath).View()
	}
	m.modelView.spinners.RemoveIfExistsSpinner(xpath)
	if !ps.Reachable.Load() {
		return "🔴"
	}
	if !ps.SSHConnectable.Load() {
		return "🟡"
	}
	if !ps.Bootstrapped.Load() {
		return "🟠"
	}
	return "✅"
}

func (m *model) getStatusText(ps *config.MetaStatus, xpath string, log *config.PhaseLog) string {
	tas := log.TimeAndState.GetTimeAndState()
	if !tas.Finished {
		return m.modelView.spinners.GetOrCreateSpinner(xpath).View()
	}
	m.modelView.spinners.RemoveIfExistsSpinner(xpath)
	if !ps.Reachable.Load() {
		return "UNREACHABLE"
	}
	if !ps.SSHConnectable.Load() {
		return "SSH_FAILED"
	}
	if !ps.Bootstrapped.Load() {
		return "NOT_BOOTSTRAPPED"
	}
	return "OK"
}
