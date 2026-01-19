package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (m *model) ViewStatsTable() string {
	state := m.workflow.State()
	phasesList := state.Phases
	colors := state.Conf.Tui.ColorScheme

	if state.Conf.Flags.DryRun || !slices.Contains(phasesList, phases.Inspect) {
		return ""
	}

	var builder strings.Builder

	// Header for the log view
	builder.WriteString(colors.HeaderTitle.Render("=== Stats table ===\n"))

	// Use provided width, with reasonable bounds and accounting for borders
	usableWidth := max(m.modelView.dimensions.Width-2, 60)

	// Header text lengths including icons
	flakeHeader := string(colors.Flake.Icon) + " FLAKE"
	configurationHeader := string(colors.Configuration.Icon) + " CONFIGURATION"
	machineHeader := string(colors.Machine.Icon) + " MACHINE"

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(colors.TableBorder).
		Headers("", "", flakeHeader, configurationHeader, machineHeader, "ARCH", "STATUS", "GENERATIONS", "LAST GENERATION DATE", "NIXOS", "KERNEL").
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
				return colors.Flake.Color
			case 3: // CONFIG
				return colors.Configuration.Color
			case 4: // MACHINE
				return colors.Machine.Color
			case 5: // Architecture
				return colors.TableRow
			case 6: // Bootstrap status
				return colors.TableRow
			case 7: // Generation
				return colors.TableRow
			case 8: // Date
				return colors.TableRow
			case 9: // Nixos
				return colors.TableRow
			case 10: // Kernel
				return colors.TableRow
			default:
				return colors.TableRow
			}
		})

	// Track previous values to implement row spanning
	var prevFlakeName, prevConfigName string

	state.RootTree(func(i int, machine *config.Machine) {
		configuration := machine.Configuration
		flake := configuration.Flake

		xpath := machine.Xpath

		ps := machine.MetaStatus

		phaseLog := state.Logs.GetFirstLogErrorOrLastLog(machine.Xpath)

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

		generationString := ""
		generation := ps.Generation.Load()
		if generation != 0 {
			generationString = fmt.Sprintf("%d", generation)
		}

		t.Row(
			fmt.Sprintf("%d", i),
			m.getStatusIcon(ps, xpath, phaseLog),
			flakeDisplay,
			configDisplay,
			machine.Name,
			ps.Architecture.Load(),
			m.getStatusText(ps, xpath, phaseLog),
			generationString,
			ps.Date.Load(),
			ps.Nixos.Load(),
			ps.Kernel.Load(),
		)
	})

	builder.WriteString("\n" + t.String() + "\n\n")

	return builder.String()
}

func (m *model) getStatusIcon(ps *config.MetaStatus, xpath config_attributes.Xpath, phaseLog *logs.PhaseLog) string {
	if phaseLog == nil {
		return m.modelView.spinners.GetOrCreateSpinner(xpath).View()
	}

	tas := phaseLog.TimeAndState().GetTimeAndState()
	if !tas.Finished {
		return m.modelView.spinners.GetOrCreateSpinner(xpath).View()
	}

	m.modelView.spinners.RemoveIfExistsSpinner(xpath)

	if tas.Error != nil {
		return "🔴"
	}

	return "✅"
}

func (m *model) getStatusText(ps *config.MetaStatus, xpath config_attributes.Xpath, phaseLog *logs.PhaseLog) string {
	if phaseLog == nil {
		return ""
	}

	tas := phaseLog.TimeAndState().GetTimeAndState()
	if !tas.Finished {
		return phaseLog.LastNonMsgOnlyCommand().Description + "-ing"
	}

	if tas.Error != nil {
		return phaseLog.LastNonMsgOnlyCommand().StatusIfFailed
	}

	return "DONE"
}
