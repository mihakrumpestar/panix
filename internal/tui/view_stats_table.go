package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// tableColumn represents a column definition with its header and style.
type tableColumn struct {
	header string
	style  func(*config.ColorScheme) lipgloss.Style
}

// makeTableColumns returns all column definitions.
// The empty header strings are for index and status icon columns that use visual indicators.
func makeTableColumns(colors *config.ColorScheme, indexWidth int) ([]string, func(int) lipgloss.Style) {
	columns := []tableColumn{
		{header: "", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow.Width(indexWidth).Align(lipgloss.Right)
		}},
		{header: "", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow.Width(3).Align(lipgloss.Center)
		}},
		{header: string(colors.Flake.Icon) + " FLAKE", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Flake.Color
		}},
		{header: string(colors.Configuration.Icon) + " CONFIG", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Configuration.Color
		}},
		{header: string(colors.Machine.Icon) + " MACHINE", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Machine.Color
		}},
		{header: "ARCH", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow
		}},
		{header: "STATUS", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow
		}},
		{header: "GEN", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow
		}},
		{header: "DATE", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow
		}},
		{header: "NIXOS", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow
		}},
		{header: "KERNEL", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.TableRow
		}},
	}

	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.header
	}

	return headers, func(col int) lipgloss.Style {
		if col >= 0 && col < len(columns) {
			return columns[col].style(colors)
		}
		return colors.TableRow
	}
}

// rowSpanMarker is the visual indicator shown for spanned (merged) cells in the table.
const rowSpanMarker = " 󱞩"

func (m *model) ViewStatsTable() string {
	state := m.resetable.workflow.State()
	phasesList := m.conf.Phases
	colors := m.conf.ColorScheme

	if m.conf.Flags.DryRun || !slices.Contains(phasesList, phases.Inspect) {
		return ""
	}

	var builder strings.Builder

	builder.WriteString(colors.HeaderTitle.Render("=== Stats table ===\n"))

	// Account for main viewport scrollbar and padding
	usableWidth := max(m.resetable.viewports.ContentWidth(), 40)

	machineCount := m.resetable.workflow.MachineCount()
	indexWidth := len(fmt.Sprintf("%d", machineCount))

	headers, styleFunc := makeTableColumns(colors, indexWidth)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(colors.TableBorder).
		Headers(headers...).
		Width(usableWidth).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				return colors.TableRow
			}
			return styleFunc(col)
		})

	var prevFlakeName, prevConfigName string

	m.resetable.workflow.RootTree(func(i int, machine *config.Machine) {
		configuration := machine.ParentConfiguration
		flake := configuration.ParentFlake
		xpath := machine.Xpath

		ps := machine.MetaInspect
		phaseLog := state.TargetsLogs.GetFirstLogErrorOrLastLog(machine.Xpath)

		// Row spanning logic: show cell content only on first occurrence
		showFlake := flake.Name != prevFlakeName
		if showFlake {
			prevFlakeName = flake.Name
		}

		showConfig := configuration.Name != prevConfigName || flake.Name != prevFlakeName
		if showConfig {
			prevConfigName = configuration.Name
		}

		flakeDisplay := rowSpanMarker
		if showFlake {
			flakeDisplay = flake.Name
		}

		configDisplay := rowSpanMarker
		if showConfig {
			configDisplay = configuration.Name
		}

		generationString := ""
		if generation := ps.Generation.Load(); generation != 0 {
			generationString = fmt.Sprintf("%d", generation)
		}

		t.Row(
			fmt.Sprintf("%d", i+1),
			m.getStatusIcon(xpath, phaseLog),
			flakeDisplay,
			configDisplay,
			machine.Name,
			ps.Architecture.Load(),
			m.getStatusText(phaseLog, colors),
			generationString,
			ps.Date.Load(),
			ps.Nixos.Load(),
			ps.Kernel.Load(),
		)
	})

	builder.WriteString("\n" + t.String() + "\n\n")

	return builder.String()
}

func (m *model) getStatusIcon(xpath config_attributes.Xpath, phaseLog *logs_phase.PhaseLog) string {
	if phaseLog == nil {
		return m.resetable.spinners.GetOrCreateSpinner(xpath).View()
	}

	tas := phaseLog.TimeAndState()
	if !tas.IsFinished() {
		return m.resetable.spinners.GetOrCreateSpinner(xpath).View()
	}

	if tas.GetEndError() != nil {
		return "🔴"
	}

	return "✅"
}

func (m *model) getStatusText(phaseLog *logs_phase.PhaseLog, colors *config.ColorScheme) string {
	if phaseLog == nil {
		return ""
	}

	tas := phaseLog.TimeAndState()
	if !tas.IsFinished() {
		lastCommand := phaseLog.Last()
		if lastCommand == nil {
			return "" // TODO: fix this bug
		}

		return colors.StatusRunning.Render(lastCommand.StatusIfRunning)
	}

	if tas.GetEndError() != nil {
		return colors.StatusError.Render(phaseLog.Last().StatusIfFailed)
	}

	return colors.StatusOK.Render("done")
}
