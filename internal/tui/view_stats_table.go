package tui

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const (
	statsTableZonePrefix = "stats-table"
	rowSpanMarker        = " 󱞩"
)

type MachineRow = config.MachineRow

type StatsTable struct {
	Data *config.StatsTableData
}

func NewStatsTable() *StatsTable {
	return &StatsTable{Data: &config.StatsTableData{SelectedMachine: -1}}
}

func (s *StatsTable) Reset() {
	s.Data = &config.StatsTableData{SelectedMachine: -1}
}

func (s *StatsTable) HandleMouseClick(msg tea.MouseClickMsg) bool {
	zoneInfo := zone.Get(statsTableZonePrefix)
	if zoneInfo == nil || !zoneInfo.InBounds(msg) {
		return false
	}

	dataRows := len(s.Data.MachineXpaths)
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
	if rowIndex < 0 || rowIndex >= dataRows || rowIndex >= len(s.Data.MachineXpaths) {
		return false
	}

	if s.Data.SelectedMachine == rowIndex {
		s.Data.SelectedMachine = -1
	} else {
		s.Data.SelectedMachine = rowIndex
	}

	return true
}

func (s *StatsTable) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(s.Data.MachineXpaths) == 0 || s.Data.SelectedMachine < 0 {
		return false
	}

	switch key {
	case "left":
		if s.Data.SelectedMachine > 0 {
			s.Data.SelectedMachine--
		}

		return true
	case "right":
		if s.Data.SelectedMachine < len(s.Data.MachineXpaths)-1 {
			s.Data.SelectedMachine++
		}

		return true
	}

	return false
}

func (s *StatsTable) GetSelectedXpath() attributes.Xpath {
	if s.Data.SelectedMachine < 0 || s.Data.SelectedMachine >= len(s.Data.MachineXpaths) {
		return attributes.Xpath{}
	}

	return s.Data.MachineXpaths[s.Data.SelectedMachine]
}

func (m *model) ViewStatsTable() string {
	resetable := m.resetable.Load()
	if resetable == nil {
		return ""
	}

	if m.conf.Flags.DryRun || !slices.Contains(m.conf.Phases, phases.Inspect) {
		return ""
	}

	statsTable := resetable.statsTable
	usableWidth := resetable.viewports.ContentWidth()

	currentRows := m.collectMachineRows(statsTable)
	if statsTable.Data == nil {
		return ""
	}

	currentHash := cacheHash(currentRows, usableWidth, statsTable.Data.SelectedMachine)

	if statsTable.Data.CacheHash == currentHash && statsTable.Data.CacheTableContent != "" {
		return statsTable.Data.CacheTableContent
	}

	result := m.buildStatsTable(statsTable, usableWidth, currentRows)

	return result
}

func cacheHash(rows []MachineRow, usableWidth int, selectedMachine int) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d\x00%d\x00%+v", usableWidth, selectedMachine, rows)
	return h.Sum64()
}

func (m *model) collectMachineRows(statsTable *StatsTable) []MachineRow {
	selectedMachine := statsTable.Data.SelectedMachine

	m.conf.Fleet.CalculateDurationAndError(m.conf.Phases)
	m.conf.Fleet.CollectStatsTableData()

	data := m.conf.Fleet.StatsTable()
	if data != nil {
		data.SelectedMachine = selectedMachine
		statsTable.Data = data
	}

	return statsTable.Data.Rows
}

func (m *model) machineByXpath(xpath string) *config.Machine {
	var found *config.Machine
	m.conf.Fleet.IterateMachines(func(machine *config.Machine) {
		if machine.Xpath.String() == xpath && found == nil {
			found = machine
		}
	})
	return found
}

func (m *model) buildStatsTable(statsTable *StatsTable, usableWidth int, rows []MachineRow) string {
	var builder strings.Builder

	builder.WriteString(m.conf.ColorScheme.Header.Title.Render("=== Stats table ===\n"))

	indexWidth := len(strconv.Itoa(len(rows)))
	headers, styleFunc := makeTableColumns(m.conf.ColorScheme, indexWidth, statsTable.Data.SelectedMachine)
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(m.conf.ColorScheme.Table.Border).
		Headers(headers...).
		Width(usableWidth).
		Wrap(false).
		StyleFunc(styleFunc)

	var prevFlakeName, prevConfigName string

	for idx, row := range rows {
		machine := m.machineByXpath(row.Xpath)
		phaseLog := machine.GetCurrentTargetLog()

		flakeDisplay := rowSpanMarker
		if row.FlakeName != prevFlakeName {
			flakeDisplay = row.FlakeName
			prevFlakeName = row.FlakeName
		}

		configDisplay := rowSpanMarker
		if row.ConfigName != prevConfigName || row.FlakeName != prevFlakeName {
			configDisplay = row.ConfigName
			prevConfigName = row.ConfigName
		}

		statusIcon := m.getStatusIcon(phaseLog)
		statusText := m.getStatusText(phaseLog, m.conf.ColorScheme)

		tbl.Row(
			strconv.Itoa(idx+1),
			statusIcon,
			flakeDisplay,
			configDisplay,
			row.MachineName,
			row.Architecture,
			statusText,
			row.Generation,
			row.Date,
			row.Nixos,
			row.Kernel,
		)
	}

	tableContent := zone.Mark(statsTableZonePrefix, tbl.String())
	builder.WriteString("\n" + tableContent + "\n\n")

	result := builder.String()

	statsTable.Data.CacheHash = cacheHash(rows, usableWidth, statsTable.Data.SelectedMachine)
	statsTable.Data.CacheTableContent = result

	return result
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
			return c.Table.Row.Width(2) //nolint:mnd
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

func (m *model) getStatusIcon(phaseLog *phase.PhaseLog) string {
	if phaseLog == nil {
		return "🔄"
	}

	tas := phaseLog.TimeAndState.Load()
	if !tas.IsFinished() {
		return "🔄"
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

	tas := phaseLog.TimeAndState.Load()
	if !tas.IsFinished() {
		lastCommand, ok := phaseLog.CommandLogs.Last()
		if !ok {
			return ""
		}

		return colors.Status.Running.Render(lastCommand.StatusIfRunning)
	}

	if tas.GetEndError() != nil {
		lastCommand, ok := phaseLog.CommandLogs.Last()
		if !ok {
			return colors.Status.Error.Render("internal error: last command is nil")
		}

		return colors.Status.Error.Render(lastCommand.StatusIfFailed)
	}

	return colors.Status.OK.Render("done")
}
