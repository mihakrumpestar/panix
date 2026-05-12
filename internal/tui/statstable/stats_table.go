package statstable

import (
	"strconv"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/table"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

const (
	statsTableZonePrefix = "stats-table"
)

type StatsTable struct {
	fleet       *fleet.Fleet
	tbl         *table.Table
	colorScheme *colorscheme.ColorScheme
	content     *buffer.LinesBuf
}

func New(fleet *fleet.Fleet, colorScheme *colorscheme.ColorScheme) *StatsTable {
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

	headers := [][]byte{[]byte(""), []byte(""),
		joinBytes(colorScheme.Flake.Icon, " FLAKE"),
		joinBytes(colorScheme.Configuration.Icon, " CONFIGURATION"),
		joinBytes(colorScheme.Machine.Icon, " MACHINE"),
		[]byte("ARCH"), []byte("STATUS"), []byte("GEN"), []byte("DATE"), []byte("OS VERSION"), []byte("KERNEL")}

	tbl := table.New(table.Config{
		Border:              style.NormalBorder(),
		BorderStyle:         colorScheme.Table.Border,
		Headers:             headers,
		Wrap:                false,
		SelectionBackground: colorScheme.Table.SelectionHighlightBackground.GetBackground(),
		ColumnStyles:        columnStyles,
	})
	tbl.SetZonePrefix(statsTableZonePrefix)

	return &StatsTable{
		fleet:       fleet,
		tbl:         tbl,
		colorScheme: colorScheme,
		content:     buffer.NewLinesBuf(),
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

func (s *StatsTable) HandleMouseClick(msg zeroterm.MouseClickMsg) bool {
	return s.tbl.HandleMouseClick(msg)
}

func (s *StatsTable) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	return s.tbl.HandleNavigation(key, hasActiveInnerViewport)
}

// Render renders the stats table and returns the output buffer.
func (s *StatsTable) Render(width int) *buffer.LinesBuf {
	s.content.Reset()

	statsTableHeader := [][]byte{
		s.colorScheme.Header.Title.RenderLine([]byte("=== Stats Table ===")),
		[]byte{},
	}
	s.content.WriteLines(statsTableHeader)

	s.tbl.Width(width).SetRows(s.buildRows())
	s.content.AppendFrom(s.tbl.Render())

	s.content.EmptyLine()

	return s.content
}

func (s *StatsTable) buildRows() [][][]byte {
	machineInfos := s.fleet.CacheMachineInfos
	rows := make([][][]byte, len(machineInfos))

	var prevFlakeName, prevConfigurationName string

	for idx, machineInfo := range machineInfos {
		flakeName, configurationName, machineName := machineInfo.Xpath.FleetLeaf()

		marker := append([]byte{' '}, s.colorScheme.Chars.RowSpanMarker...)

		flakeDisplay := marker
		if flakeName != prevFlakeName {
			flakeDisplay = []byte(flakeName)
			prevFlakeName = flakeName
		}

		configDisplay := marker
		if configurationName != prevConfigurationName || flakeName != prevFlakeName {
			configDisplay = []byte(configurationName)
			prevConfigurationName = configurationName
		}

		rows[idx] = [][]byte{
			[]byte(strconv.Itoa(idx + 1)),
			getStatusIcon(machineInfo.State.Status, s.colorScheme),
			flakeDisplay,
			configDisplay,
			[]byte(machineName),
			[]byte(machineInfo.MetaInspect.Architecture),
			getStatusText(machineInfo.State.Status, machineInfo.State.StatusMsg, s.colorScheme),
			getGeneration(machineInfo),
			[]byte(machineInfo.MetaInspect.Date),
			[]byte(machineInfo.MetaInspect.OSVersion),
			[]byte(machineInfo.MetaInspect.Kernel),
		}
	}

	return rows
}

func getStatusIcon(status stats.StatsState, colorScheme *colorscheme.ColorScheme) []byte {
	switch status {
	case stats.Running:
		return colorScheme.Status.Icons.Running
	case stats.Failed:
		return colorScheme.Status.Icons.Failed
	case stats.Done:
		return colorScheme.Status.Icons.OK
	default:
		return []byte("invalid")
	}
}

func getStatusText(status stats.StatsState, statusMsg string, colorScheme *colorscheme.ColorScheme) []byte {
	switch status {
	case stats.Running:
		return colorScheme.Status.Running.RenderLine([]byte(statusMsg))
	case stats.Failed:
		return colorScheme.Status.Failed.RenderLine([]byte(statusMsg))
	case stats.Done:
		return colorScheme.Status.OK.RenderLine([]byte(statusMsg))
	default:
		return []byte("invalid")
	}
}

func getGeneration(machineInfo fleet.MachineInfo) []byte {
	if machineInfo.MetaInspect.Generations == nil {
		return nil
	}

	return []byte(strconv.FormatUint(uint64(machineInfo.MetaInspect.Generations.Current), 10))
}

func joinBytes(prefix []byte, suffix string) []byte {
	b := make([]byte, len(prefix)+len(suffix))
	copy(b, prefix)
	copy(b[len(prefix):], suffix)

	return b
}
