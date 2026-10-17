package statstable

import (
	"bytes"
	"strconv"

	"github.com/mihakrumpestar/panix/internal/config/theme"
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
	fleet     *fleet.Fleet
	tbl       *table.Table
	scheme    *theme.Theme
	content   *buffer.LinesBuf
	rowMarker []byte
}

func New(fleet *fleet.Fleet, scheme *theme.Theme) *StatsTable {
	statsTable := &StatsTable{
		fleet:   fleet,
		content: buffer.NewLinesBuf(),
	}
	statsTable.ApplyTheme(scheme)
	statsTable.tbl.SetZonePrefix(statsTableZonePrefix)

	return statsTable
}

// ApplyTheme re-runs the constructor styling: column styles, headers, row marker
// and table config, invalidating table render caches. Row selection is preserved.
// The theme carries derived render values, so no per-widget snapshot is kept.
func (s *StatsTable) ApplyTheme(scheme *theme.Theme) {
	s.scheme = scheme

	cfg := s.tableConfig(scheme)
	if s.tbl == nil {
		s.tbl = table.New(cfg)
	} else {
		s.tbl.ApplyTheme(cfg)
	}

	s.rowMarker = append([]byte{' '}, scheme.Chars.RowSpanMarkerBytes...)
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

	s.scheme.Header.Title.Style.RenderLineInto(s.content, []byte("=== Stats Table ==="))
	s.content.EmptyLine()

	s.tbl.Width(width).SetRows(s.buildRows())
	s.content.AppendFrom(s.tbl.Render())

	s.content.EmptyLine()

	return s.content
}

func (s *StatsTable) tableConfig(scheme *theme.Theme) table.Config {
	indexWidth := len(strconv.Itoa(s.fleet.MachineCount()))

	// The status column must fit the widest status icon: a 1-cell-only theme gets
	// a 1-cell column, a mixed 1/2-cell theme gets 2, so nothing overflows or
	// shifts the row separator.
	statusIconWidth := max(
		style.CellWidth(scheme.Status.Icons.OKBytes),
		style.CellWidth(scheme.Status.Icons.FailedBytes),
		style.CellWidth(scheme.Status.Icons.RunningBytes),
	)

	columnStyles := []style.Style{
		scheme.Table.Row.Style.Width(indexWidth).Align(style.Right),
		scheme.Table.Row.Style.Width(statusIconWidth),
		scheme.Flake.Style,
		scheme.OutputType.Style,
		scheme.Installable.Style,
		scheme.Machine.Style,
		scheme.Table.Row.Style,
		scheme.Table.Row.Style,
		scheme.Table.Row.Style,
		scheme.Table.Row.Style,
		scheme.Table.Row.Style,
		scheme.Table.Row.Style,
	}

	headers := [][]byte{[]byte(""), []byte(""),
		joinBytes(scheme.Flake.IconBytes, " FLAKE"),
		joinBytes(scheme.OutputType.IconBytes, " OUTPUT TYPE"),
		joinBytes(scheme.Installable.IconBytes, " NAME"),
		joinBytes(scheme.Machine.IconBytes, " MACHINE"),
		[]byte("ARCH"), []byte("STATUS"), []byte("GEN"), []byte("GEN DATE"), []byte("OS VERSION"), []byte("KERNEL")}

	return table.Config{
		Border:              scheme.Border.TableGlyphs,
		BorderStyle:         scheme.Table.Border.Style,
		Headers:             headers,
		Wrap:                false,
		SelectionBackground: scheme.Table.SelectionHighlightBackground.Style.GetBackground(),
		ColumnStyles:        columnStyles,
	}
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
			s.statusIcon(machineInfo.State.Status),
			flakeDisplay,
			outputTypeDisplay,
			outputNameDisplay,
			machineInfo.MachineName.Bytes(),
			machineInfo.Architecture.Bytes(),
			s.statusText(machineInfo.State.Status, machineInfo.StatusMsgBytes.Bytes()),
			getGeneration(machineInfo),
			machineInfo.Date.Bytes(),
			machineInfo.OSVersion.Bytes(),
			machineInfo.Kernel.Bytes(),
		}
	}

	return rows
}

func (s *StatsTable) statusIcon(status stats.StatsState) []byte {
	switch status {
	case stats.Running:
		return s.scheme.Status.Icons.RunningBytes
	case stats.Failed:
		return s.scheme.Status.Icons.FailedBytes
	case stats.Done:
		return s.scheme.Status.Icons.OKBytes
	default:
		return []byte("invalid")
	}
}

func (s *StatsTable) statusText(status stats.StatsState, statusMsg []byte) []byte {
	switch status {
	case stats.Running:
		return s.scheme.Status.Running.Style.RenderLine(statusMsg)
	case stats.Failed:
		return s.scheme.Status.Failed.Style.RenderLine(statusMsg)
	case stats.Done:
		return s.scheme.Status.OK.Style.RenderLine(statusMsg)
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
