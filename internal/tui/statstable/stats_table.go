package statstable

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

const (
	statsTableZonePrefix = "stats-table"
	rowSpanMarker        = " 󱞩"
)

type StatsTable struct {
	Selected Selected `json:"selected"`

	CacheMachineInfos  []MachineInfo `json:"-"`
	CacheFlattenedLogs []*logs.Logs  `json:"-"`

	cache cache.Cache[string]
}

type Selected struct {
	Xpath xpath.Xpath `json:"xpath"`
	Index int         `json:"index"`
}

type MachineInfo struct {
	Xpath       xpath.Xpath
	MetaInspect machine.MetaInspect
	State       machine.State
}

func NewStatsTable() *StatsTable {
	return &StatsTable{
		Selected: Selected{Index: -1},
	}
}

func (s *StatsTable) Reset() {
	s.Selected.Xpath.Clear()
	s.Selected.Index = -1
}

func (s *StatsTable) HandleMouseClick(msg tea.MouseClickMsg) bool {
	zoneInfo := zone.Get(statsTableZonePrefix)
	if zoneInfo == nil || !zoneInfo.InBounds(msg) {
		return false
	}

	dataRows := len(s.CacheMachineInfos)
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
	if rowIndex < 0 || rowIndex >= dataRows || rowIndex >= len(s.CacheMachineInfos) {
		return false
	}

	if s.Selected.Index == rowIndex {
		s.Selected.Index = -1
		s.Selected.Xpath.Clear()
	} else {
		s.Selected.Index = rowIndex
		s.applyIndexToXpath()
	}

	return true
}

func (s *StatsTable) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(s.CacheMachineInfos) == 0 || s.Selected.Index < 0 {
		return false
	}

	switch key {
	case "left":
		if s.Selected.Index > 0 {
			s.Selected.Index--
			s.applyIndexToXpath()

			return true
		}
	case "right":
		if s.Selected.Index < len(s.CacheMachineInfos)-1 {
			s.Selected.Index++
			s.applyIndexToXpath()

			return true
		}
	}

	return false
}

func (s *StatsTable) View(width int, colorScheme *colorscheme.ColorScheme) string {
	width -= 2 // For scrollbar

	return s.cache.Get(
		func() (string, bool) {
			return s.buildStatsTable(width, colorScheme), true
		},
		s.CacheMachineInfos, width, s.Selected.Index)
}

func (s *StatsTable) buildStatsTable(width int, colorScheme *colorscheme.ColorScheme) string {
	var builder strings.Builder

	builder.WriteString(colorScheme.Header.Title.Render("=== Stats Table ===\n"))

	indexWidth := len(strconv.Itoa(len(s.CacheMachineInfos))) // Get width of the string representation of the number
	headers, styleFunc := makeTableColumns(colorScheme, indexWidth, s.Selected.Index)
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(colorScheme.Table.Border).
		Headers(headers...).
		Width(width).
		Wrap(false).
		StyleFunc(styleFunc)

	var prevFlakeName, prevConfigurationName string

	for idx, machineInfo := range s.CacheMachineInfos {
		flakeName, configurationName, machineName := machineInfo.Xpath.FleetLeaf()

		flakeDisplay := rowSpanMarker
		if flakeName != prevFlakeName {
			flakeDisplay = flakeName
			prevFlakeName = flakeName
		}

		configDisplay := rowSpanMarker
		if configurationName != prevConfigurationName || flakeName != prevFlakeName {
			configDisplay = configurationName
			prevConfigurationName = configurationName
		}

		tbl.Row(
			strconv.Itoa(idx+1),
			getStatusIcon(machineInfo.State.Status),
			flakeDisplay,
			configDisplay,
			machineName,
			machineInfo.MetaInspect.Architecture,
			getStatusText(machineInfo.State.Status, machineInfo.State.StatusMsg, colorScheme),
			getGeneration(machineInfo),
			machineInfo.MetaInspect.Date,
			machineInfo.MetaInspect.Nixos,
			machineInfo.MetaInspect.Kernel,
		)
	}

	tableContent := zone.Mark(statsTableZonePrefix, tbl.String())
	builder.WriteString("\n" + tableContent + "\n\n")

	result := builder.String()

	return result
}

func (s *StatsTable) applyIndexToXpath() {
	s.Selected.Xpath = s.CacheMachineInfos[s.Selected.Index].Xpath
}

type tableColumn struct {
	header string
	style  func(*colorscheme.ColorScheme) lipgloss.Style
}

func makeTableColumns(colorScheme *colorscheme.ColorScheme, indexWidth int, selectedIndex int) ([]string, func(row, col int) lipgloss.Style) {
	columns := []tableColumn{
		{header: "", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row.Width(indexWidth).Align(lipgloss.Right)
		}},
		{header: "", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row.Width(2) //nolint:mnd
		}},
		{header: string(colorScheme.Flake.Icon) + " FLAKE", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Flake.Color
		}},
		{header: string(colorScheme.Configuration.Icon) + " CONFIGURATION", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Configuration.Color
		}},
		{header: string(colorScheme.Machine.Icon) + " MACHINE", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Machine.Color
		}},
		{header: "ARCH", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "STATUS", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "GEN", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "DATE", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "NIXOS", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
		{header: "KERNEL", style: func(c *colorscheme.ColorScheme) lipgloss.Style {
			return c.Table.Row
		}},
	}

	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.header
	}

	return headers, func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return colorScheme.Table.Row
		}

		if col >= 0 && col < len(columns) {
			style := columns[col].style(colorScheme)

			if row == selectedIndex {
				style = style.Background(colorScheme.Table.SelectionHighlightBackground.GetBackground())
			}

			return style
		}

		return colorScheme.Table.Row
	}
}

func getStatusIcon(status stats.StatsState) string {
	switch status {
	case stats.Running:
		return "🔄"
	case stats.Failed:
		return "🔴"
	case stats.Done:
		return "✅"
	default:
		return "invalid"
	}
}

func getStatusText(status stats.StatsState, statusMsg string, colorScheme *colorscheme.ColorScheme) string {
	switch status {
	case stats.Running:
		return colorScheme.Status.Running.Render(statusMsg)
	case stats.Failed:
		return colorScheme.Status.Failed.Render(statusMsg)
	case stats.Done:
		return colorScheme.Status.OK.Render(statusMsg)
	default:
		return "invalid"
	}
}

func getGeneration(machineInfo MachineInfo) string {
	if machineInfo.MetaInspect.Generations == nil {
		return ""
	}

	return strconv.FormatUint(uint64(machineInfo.MetaInspect.Generations.Current), 10)
}
