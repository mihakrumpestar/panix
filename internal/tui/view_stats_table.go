package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const statsTableZonePrefix = "stats-table"
const rowSpanMarker = " 󱞩"

type StatsTable struct {
	SelectedMachine int
	MachineXpaths   []config_attributes.Xpath
}

func NewStatsTable() *StatsTable {
	return &StatsTable{
		SelectedMachine: -1,
	}
}

func (s *StatsTable) Reset() {
	s.SelectedMachine = -1
	s.MachineXpaths = nil
}

func (s *StatsTable) HandleMouseClick(msg tea.MouseClickMsg) bool {
	z := zone.Get(statsTableZonePrefix)
	if z == nil || !z.InBounds(msg) {
		return false
	}

	dataRows := len(s.MachineXpaths)
	if dataRows == 0 {
		return false
	}

	mouse := msg.Mouse()
	relY := mouse.Y - z.StartY
	headerLines := 3

	if relY < headerLines {
		return false
	}

	rowIndex := relY - headerLines
	if rowIndex < 0 || rowIndex >= dataRows || rowIndex >= len(s.MachineXpaths) {
		return false
	}

	if s.SelectedMachine == rowIndex {
		s.SelectedMachine = -1
	} else {
		s.SelectedMachine = rowIndex
	}
	return true
}

func (s *StatsTable) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(s.MachineXpaths) == 0 || s.SelectedMachine < 0 {
		return false
	}

	switch key {
	case "left":
		if s.SelectedMachine > 0 {
			s.SelectedMachine--
		}
		return true
	case "right":
		if s.SelectedMachine < len(s.MachineXpaths)-1 {
			s.SelectedMachine++
		}
		return true
	}
	return false
}

func (s *StatsTable) GetSelectedXpath() config_attributes.Xpath {
	if s.SelectedMachine < 0 || s.SelectedMachine >= len(s.MachineXpaths) {
		return config_attributes.Xpath{}
	}
	return s.MachineXpaths[s.SelectedMachine]
}

func (m *model) ViewStatsTable() string {
	r := m.resetable.Load()
	state := r.workflow.State()
	phasesList := m.conf.Phases
	colors := m.conf.ColorScheme

	if m.conf.Flags.DryRun || !slices.Contains(phasesList, phases.Inspect) {
		return ""
	}

	var builder strings.Builder

	builder.WriteString(colors.HeaderTitle.Render("=== Stats table ===\n"))

	usableWidth := max(r.viewports.ContentWidth(), 40)
	machineCount := r.workflow.MachineCount()
	indexWidth := len(fmt.Sprintf("%d", machineCount))

	statsTable := r.statsTable
	statsTable.MachineXpaths = nil

	headers, styleFunc := makeTableColumns(colors, indexWidth, statsTable.SelectedMachine)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(colors.TableBorder).
		Headers(headers...).
		Width(usableWidth).
		Wrap(false).
		StyleFunc(styleFunc)

	var prevFlakeName, prevConfigName string

	r.workflow.RootTree(func(i int, machine *config.Machine) {
		configuration := machine.ParentConfiguration
		flake := configuration.ParentFlake
		xpath := machine.Xpath

		statsTable.MachineXpaths = append(statsTable.MachineXpaths, xpath)

		ps := machine.MetaInspect
		phaseLog := state.TargetsLogs.MustGetFirstLogErrorOrLastLog(machine.Xpath)

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

	tableContent := zone.Mark(statsTableZonePrefix, t.String())
	builder.WriteString("\n" + tableContent + "\n\n")

	return builder.String()
}

type tableColumn struct {
	header string
	style  func(*config.ColorScheme) lipgloss.Style
}

func makeTableColumns(colors *config.ColorScheme, indexWidth int, selectedRow int) ([]string, func(row, col int) lipgloss.Style) {
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
		{header: string(colors.Configuration.Icon) + " CONFIGURATION", style: func(c *config.ColorScheme) lipgloss.Style {
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

	return headers, func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return colors.TableRow
		}

		if col >= 0 && col < len(columns) {
			style := columns[col].style(colors)

			if row == selectedRow {
				style = style.Background(colors.SelectionHighlightBackground.GetBackground())
			}

			return style
		}

		return colors.TableRow
	}
}

func (m *model) getStatusIcon(xpath config_attributes.Xpath, phaseLog *logs_phase.PhaseLog) string {
	r := m.resetable.Load()
	if phaseLog == nil {
		return r.spinners.GetOrCreateSpinner(xpath).View()
	}

	tas := phaseLog.TimeAndState()
	if !tas.IsFinished() {
		return r.spinners.GetOrCreateSpinner(xpath).View()
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
			return ""
		}

		return colors.StatusRunning.Render(lastCommand.StatusIfRunning)
	}

	if tas.GetEndError() != nil {
		return colors.StatusError.Render(phaseLog.Last().StatusIfFailed)
	}

	return colors.StatusOK.Render("done")
}
