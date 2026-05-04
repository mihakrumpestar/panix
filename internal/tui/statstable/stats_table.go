package statstable

import (
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/table"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

const (
	statsTableZonePrefix = "stats-table"
)

type StatsTable struct {
	fleet       *fleet.Fleet
	tbl         *table.Table
	colorScheme *colorscheme.ColorScheme
}

func NewStatsTable(fleet *fleet.Fleet, colorScheme *colorscheme.ColorScheme) *StatsTable {
	indexWidth := len(strconv.Itoa(fleet.MachineCount()))

	columnStyles := []style.Style{
		colorScheme.Table.Row.Width(indexWidth).Align(style.Right),
		colorScheme.Table.Row.Width(2), //nolint:mnd
		colorScheme.Flake.Color,
		colorScheme.Configuration.Color,
		colorScheme.Machine.Color,
		colorScheme.Table.Row,
		colorScheme.Table.Row,
		colorScheme.Table.Row,
		colorScheme.Table.Row,
		colorScheme.Table.Row,
		colorScheme.Table.Row,
	}

	headers := []string{"", "",
		colorScheme.Flake.Icon + " FLAKE",
		colorScheme.Configuration.Icon + " CONFIGURATION",
		colorScheme.Machine.Icon + " MACHINE",
		"ARCH", "STATUS", "GEN", "DATE", "NIXOS", "KERNEL"}

	tbl := table.New().
		Border(style.NormalBorder()).
		SetZonePrefix(statsTableZonePrefix).
		BorderStyle(colorScheme.Table.Border).
		Headers(headers...).
		Wrap(false).
		SelectionBackground(colorScheme.Table.SelectionHighlightBackground.GetBackground()).
		ColumnStyles(columnStyles)

	return &StatsTable{
		fleet:       fleet,
		tbl:         tbl,
		colorScheme: colorScheme,
	}
}

func (s *StatsTable) SelectedIndex() int {
	return s.tbl.SelectedIndex()
}

func (s *StatsTable) SelectedXpath() xpath.Xpath {
	idx := s.tbl.SelectedIndex()
	if idx >= 0 && idx < len(s.fleet.CacheMachineInfos) {
		return s.fleet.CacheMachineInfos[idx].Xpath
	}

	return ""
}

func (s *StatsTable) Reset() {
	s.tbl.Deselect()
}

func (s *StatsTable) HandleMouseClick(msg render.MouseClickMsg) bool {
	return s.tbl.HandleMouseClick(msg)
}

func (s *StatsTable) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	return s.tbl.HandleNavigation(key, hasActiveInnerViewport)
}

func (s *StatsTable) View(width int) string {
	var builder strings.Builder

	builder.WriteString(s.colorScheme.Header.Title.Render("=== Stats Table ===\n"))

	s.tbl.Width(width).SetRows(s.buildRows())

	builder.WriteString("\n" + s.tbl.String() + "\n\n")

	return builder.String()
}

func (s *StatsTable) buildRows() [][]string {
	machineInfos := s.fleet.CacheMachineInfos
	rows := make([][]string, len(machineInfos))

	var prevFlakeName, prevConfigurationName string

	for idx, machineInfo := range machineInfos {
		flakeName, configurationName, machineName := machineInfo.Xpath.FleetLeaf()

		marker := " " + s.colorScheme.Chars.RowSpanMarker

		flakeDisplay := marker
		if flakeName != prevFlakeName {
			flakeDisplay = flakeName
			prevFlakeName = flakeName
		}

		configDisplay := marker
		if configurationName != prevConfigurationName || flakeName != prevFlakeName {
			configDisplay = configurationName
			prevConfigurationName = configurationName
		}

		rows[idx] = []string{
			strconv.Itoa(idx + 1),
			getStatusIcon(machineInfo.State.Status, s.colorScheme),
			flakeDisplay,
			configDisplay,
			machineName,
			machineInfo.MetaInspect.Architecture,
			getStatusText(machineInfo.State.Status, machineInfo.State.StatusMsg, s.colorScheme),
			getGeneration(machineInfo),
			machineInfo.MetaInspect.Date,
			machineInfo.MetaInspect.Nixos,
			machineInfo.MetaInspect.Kernel,
		}
	}

	return rows
}

func getStatusIcon(status stats.StatsState, colorScheme *colorscheme.ColorScheme) string {
	switch status {
	case stats.Running:
		return colorScheme.Status.Icons.Running
	case stats.Failed:
		return colorScheme.Status.Icons.Failed
	case stats.Done:
		return colorScheme.Status.Icons.OK
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

func getGeneration(machineInfo fleet.MachineInfo) string {
	if machineInfo.MetaInspect.Generations == nil {
		return ""
	}

	return strconv.FormatUint(uint64(machineInfo.MetaInspect.Generations.Current), 10)
}
