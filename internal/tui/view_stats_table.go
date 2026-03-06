package tui

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const (
	statsTableZonePrefix    = "stats-table"
	rowSpanMarker           = " 󱞩"
	statusIconReservedWidth = 3
)

type StatsTable struct {
	SelectedMachine int
	MachineXpaths   []attributes.Xpath
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
	zoneInfo := zone.Get(statsTableZonePrefix)
	if zoneInfo == nil || !zoneInfo.InBounds(msg) {
		return false
	}

	dataRows := len(s.MachineXpaths)
	if dataRows == 0 {
		return false
	}

	mouse := msg.Mouse()
	relY := mouse.Y - zoneInfo.StartY
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

func (s *StatsTable) GetSelectedXpath() attributes.Xpath {
	if s.SelectedMachine < 0 || s.SelectedMachine >= len(s.MachineXpaths) {
		return attributes.Xpath{}
	}

	return s.MachineXpaths[s.SelectedMachine]
}

func (m *model) ViewStatsTable() string {
	resetable := m.resetable.Load()
	state := resetable.workflow.State()

	if m.conf.Flags.DryRun || !slices.Contains(m.conf.Phases, phases.Inspect) {
		return ""
	}

	var builder strings.Builder

	builder.WriteString(m.conf.ColorScheme.Header.Title.Render("=== Stats table ===\n"))

	usableWidth := resetable.viewports.ContentWidth()
	indexWidth := len(strconv.Itoa(resetable.workflow.MachineCount()))
	statsTable := resetable.statsTable
	statsTable.MachineXpaths = nil

	headers, styleFunc := makeTableColumns(m.conf.ColorScheme, indexWidth, statsTable.SelectedMachine)
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(m.conf.ColorScheme.Table.Border).
		Headers(headers...).
		Width(usableWidth).
		Wrap(false).
		StyleFunc(styleFunc)

	m.populateTableRows(tbl, statsTable, state.TargetsLogs)

	tableContent := zone.Mark(statsTableZonePrefix, tbl.String())
	builder.WriteString("\n" + tableContent + "\n\n")

	return builder.String()
}

func (m *model) populateTableRows(tbl *table.Table, statsTable *StatsTable, targetsLogs *logs.TargetsLogs) {
	var prevFlakeName, prevConfigName string

	m.resetable.Load().workflow.RootTree(func(idx int, machine *config.Machine) {
		configuration := machine.ParentConfiguration
		flake := configuration.ParentFlake
		xpath := machine.Xpath

		statsTable.MachineXpaths = append(statsTable.MachineXpaths, xpath)

		metaInspect := machine.MetaInspect
		phaseLog := targetsLogs.MustGetFirstLogErrorOrLastLog(machine.Xpath)

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
		if generation := metaInspect.Generation.Load(); generation != 0 {
			generationString = strconv.FormatUint(uint64(generation), 10)
		}

		tbl.Row(
			strconv.Itoa(idx+1),
			m.getStatusIcon(xpath, phaseLog),
			flakeDisplay,
			configDisplay,
			machine.Name,
			metaInspect.Architecture.Load(),
			m.getStatusText(phaseLog, m.conf.ColorScheme),
			generationString,
			metaInspect.Date.Load(),
			metaInspect.Nixos.Load(),
			metaInspect.Kernel.Load(),
		)
	})
}

type tableColumn struct {
	header string
	style  func(*config.ColorScheme) lipgloss.Style
}

func makeTableColumns(colors *config.ColorScheme, indexWidth int, selectedRow int) ([]string, func(row, col int) lipgloss.Style) {
	columns := []tableColumn{
		{header: "", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Table.Row.Width(indexWidth).Align(lipgloss.Right)
		}},
		{header: "", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Table.Row.Width(statusIconReservedWidth).Align(lipgloss.Center)
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
			return c.Table.Row
		}},
		{header: "STATUS", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "GEN", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "DATE", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "NIXOS", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "KERNEL", style: func(c *config.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
	}

	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.header
	}

	return headers, func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return colors.Table.Row
		}

		if col >= 0 && col < len(columns) {
			style := columns[col].style(colors)

			if row == selectedRow {
				style = style.Background(colors.Table.SelectionHighlightBackground.GetBackground())
			}

			return style
		}

		return colors.Table.Row
	}
}

func (m *model) getStatusIcon(xpath attributes.Xpath, phaseLog *phase.PhaseLog) string {
	resetable := m.resetable.Load()
	if phaseLog == nil {
		return resetable.spinners.GetOrCreateSpinner(xpath).View()
	}

	tas := phaseLog.TimeAndState()
	if !tas.IsFinished() {
		return resetable.spinners.GetOrCreateSpinner(xpath).View()
	}

	if tas.GetEndError() != nil {
		return "🔴"
	}

	return "✅"
}

func (m *model) getStatusText(phaseLog *phase.PhaseLog, colors *config.ColorScheme) string {
	if phaseLog == nil {
		return ""
	}

	tas := phaseLog.TimeAndState()
	if !tas.IsFinished() {
		lastCommand := phaseLog.Last()
		if lastCommand == nil {
			return ""
		}

		return colors.Status.Running.Render(lastCommand.StatusIfRunning)
	}

	if tas.GetEndError() != nil {
		lastCommand := phaseLog.Last()
		if lastCommand == nil {
			return colors.Status.Error.Render("internal error: last command is nil")
		}

		return colors.Status.Error.Render(lastCommand.StatusIfFailed)
	}

	return colors.Status.OK.Render("done")
}
