package statstable

import (
	"bytes"
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
	rowMarker   []byte
}

func New(fleet *fleet.Fleet, colorScheme *colorscheme.ColorScheme) *StatsTable {
	indexWidth := len(strconv.Itoa(fleet.MachineCount()))

	columnStyles := []style.Style{
		colorScheme.Table.Row.Width(indexWidth).Align(style.Right),
		colorScheme.Table.Row.Width(2),
		colorScheme.Flake.Color,
		colorScheme.OutputType.Color,
		colorScheme.Installable.Color,
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
		joinBytes(colorScheme.OutputType.Icon, " OUTPUT TYPE"),
		joinBytes(colorScheme.Installable.Icon, " NAME"),
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
		rowMarker:   append([]byte{' '}, colorScheme.Chars.RowSpanMarker...),
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

	return xpath.Xpath{}
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

	s.colorScheme.Header.Title.RenderLineInto(s.content, []byte("=== Stats Table ==="))
	s.content.EmptyLine()

	s.tbl.Width(width).SetRows(s.buildRows())
	s.content.AppendFrom(s.tbl.Render())

	s.content.EmptyLine()

	return s.content
}

func (s *StatsTable) buildRows() [][][]byte {
	machineInfos := s.fleet.CacheMachineInfos
	rows := make([][][]byte, len(machineInfos))

	var prevFlakeName, prevOutputType, prevOutputName []byte

	marker := s.rowMarker

	for idx, machineInfo := range machineInfos {
		flakeDisplay := marker
		if !bytes.Equal(machineInfo.FlakeName.Bytes(), prevFlakeName) {
			flakeDisplay = machineInfo.FlakeName.Bytes()
			prevFlakeName = machineInfo.FlakeName.Bytes()
		}

		outputTypeDisplay := marker
		if !bytes.Equal(machineInfo.OutputType.Bytes(), prevOutputType) ||
			!bytes.Equal(machineInfo.FlakeName.Bytes(), prevFlakeName) {
			outputTypeDisplay = machineInfo.OutputType.Bytes()
			prevOutputType = machineInfo.OutputType.Bytes()
		}

		outputNameDisplay := marker
		if !bytes.Equal(machineInfo.OutputName.Bytes(), prevOutputName) ||
			!bytes.Equal(machineInfo.OutputType.Bytes(), prevOutputType) ||
			!bytes.Equal(machineInfo.FlakeName.Bytes(), prevFlakeName) {
			outputNameDisplay = machineInfo.OutputName.Bytes()
			prevOutputName = machineInfo.OutputName.Bytes()
		}

		rows[idx] = [][]byte{
			strconv.AppendInt(nil, int64(idx+1), 10), //nolint:mnd // decimal base
			getStatusIcon(machineInfo.State.Status, s.colorScheme),
			flakeDisplay,
			outputTypeDisplay,
			outputNameDisplay,
			machineInfo.MachineName.Bytes(),
			machineInfo.Architecture.Bytes(),
			getStatusText(machineInfo.State.Status, machineInfo.StatusMsgBytes.Bytes(), s.colorScheme),
			getGeneration(machineInfo),
			machineInfo.Date.Bytes(),
			machineInfo.OSVersion.Bytes(),
			machineInfo.Kernel.Bytes(),
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

func getStatusText(status stats.StatsState, statusMsg []byte, colorScheme *colorscheme.ColorScheme) []byte {
	switch status {
	case stats.Running:
		return colorScheme.Status.Running.RenderLine(statusMsg)
	case stats.Failed:
		return colorScheme.Status.Failed.RenderLine(statusMsg)
	case stats.Done:
		return colorScheme.Status.OK.RenderLine(statusMsg)
	default:
		return []byte("invalid")
	}
}

func getGeneration(machineInfo fleet.MachineInfo) []byte {
	if machineInfo.MetaInspect.Generations == nil {
		return nil
	}

	return strconv.AppendUint(nil, uint64(machineInfo.MetaInspect.Generations.Current), 10) //nolint:mnd // decimal base
}

func joinBytes(prefix []byte, suffix string) []byte {
	b := make([]byte, len(prefix)+len(suffix))
	copy(b, prefix)
	copy(b[len(prefix):], suffix)

	return b
}
