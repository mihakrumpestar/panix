package table

import (
	"bytes"
	"slices"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

const HeaderRow = -1

// Config holds immutable table configuration set at construction time.
// Changing any field after construction has no effect — use Width() for
// runtime width updates and SetRows() for data updates.
type Config struct {
	Width               int
	Border              style.Border
	BorderStyle         style.Style
	Headers             [][]byte
	ColumnStyles        []style.Style
	Wrap                bool
	SelectionBackground style.Color
}

type Table struct {
	cfg Config

	rows     [][][]byte
	bordered bool

	selectedIndex int
	selBgStyle    style.Style
	zonePrefix    string
	zoneIDs       []zeroterm.ZoneID

	colWidths       []int
	colWidthsCached bool

	colAlign  []style.Position
	colANSIOK bool

	outDirty bool

	content *buffer.LinesBuf

	rowBuf *buffer.LineBuf

	borderVertical      []byte
	selBgBorderVertical []byte

	zoneStarts [][]byte
	zoneEnds   [][]byte

	rowCacheBytes     [][]byte
	rowCacheData      [][][]byte
	rowCacheSel       int
	rowCacheColWidths []int

	structuralChange bool

	widthsBuf []int
}

func New(cfg Config) *Table {
	bordered := len(cfg.Border.Vertical) > 0

	table := &Table{
		cfg:           cfg,
		bordered:      bordered,
		selBgStyle:    style.NewStyle().Background(cfg.SelectionBackground),
		selectedIndex: -1,
		outDirty:      true,
		rowCacheSel:   -2,
		content:       buffer.NewLinesBuf(),
		rowBuf:        buffer.NewLineBuf(),
	}

	if bordered {
		table.borderVertical = cfg.BorderStyle.RenderLine(cfg.Border.Vertical)
		selBorderStyle := cfg.BorderStyle.Background(cfg.SelectionBackground)
		table.selBgBorderVertical = selBorderStyle.RenderLine(cfg.Border.Vertical)
	}

	return table
}

// Width updates the table width at runtime (e.g. on terminal resize).
// All other configuration is set via Config at construction time.
func (t *Table) Width(w int) *Table {
	if t.cfg.Width == w {
		return t
	}

	t.cfg.Width = w
	t.colWidthsCached = false
	t.outDirty = true
	t.structuralChange = true

	return t
}

// SetRows replaces all row data. Selection is preserved. Detects
// identical data and skips cache invalidation, avoiding all
// allocations for the common "same data, re-render" pattern.
func (t *Table) SetRows(rows [][][]byte) *Table {
	if len(t.rows) == len(rows) {
		anyChanged := false

		for i, row := range rows {
			if !cellsEqual(t.rows[i], row) {
				anyChanged = true

				break
			}
		}

		if !anyChanged {
			return t
		}
	}

	t.rows = rows
	t.colWidthsCached = false
	t.outDirty = true

	return t
}

func (t *Table) SelectedIndex() int {
	return t.selectedIndex
}

func (t *Table) Select(idx int) {
	if idx < -1 {
		idx = -1
	}

	if idx >= len(t.rows) {
		idx = len(t.rows) - 1
	}

	if t.selectedIndex == idx {
		return
	}

	t.selectedIndex = idx
	t.outDirty = true
}

func (t *Table) Deselect() {
	if t.selectedIndex == -1 {
		return
	}

	t.selectedIndex = -1
	t.outDirty = true
}

func (t *Table) SetZonePrefix(prefix string) *Table {
	t.zonePrefix = prefix
	t.outDirty = true
	t.zoneStarts = nil
	t.zoneIDs = nil

	return t
}

// HandleMouseClick checks if a mouse click landed on a data row and
// updates the selection accordingly. Clicking outside any row zone
// deselects the current selection. Returns true if the selection state
// was changed.

func (t *Table) HandleMouseClick(msg zeroterm.MouseClickMsg) bool {
	if t.zonePrefix == "" || len(t.rows) == 0 || msg.Lines == nil {
		return false
	}

	if msg.Y < 0 || msg.Y >= msg.Lines.Len() {
		return t.deselectIfSelected()
	}

	clickedID, ok := zeroterm.ZoneIDAtCol(msg.Lines.Line(msg.Y), msg.X)
	if !ok {
		return t.deselectIfSelected()
	}

	if idx := t.findClickedRow(clickedID); idx >= 0 {
		if t.selectedIndex != idx {
			t.selectedIndex = idx
			t.outDirty = true

			return true
		}

		return false
	}

	return t.deselectIfSelected()
}

// HandleNavigation processes left/right key navigation. Returns true if
// the navigation was consumed. Allows initial selection with left/right
// when nothing is selected.
func (t *Table) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(t.rows) == 0 {
		return false
	}

	switch key {
	case "left":
		return t.navigateLeft()
	case "right":
		return t.navigateRight()
	}

	return false
}

// Render builds the table into the owned content buffer and returns it.
// On cache hit (no data/width/selection change since last Render), returns
// the existing buffer immediately with zero allocations.
// Callers must use AppendFrom or read Line(i) — do not retain the
// pointer across frames.
//

func (t *Table) Render() *buffer.LinesBuf {
	if !t.outDirty {
		return t.content
	}

	t.content.Reset()

	numCols := t.numCols()
	if numCols == 0 {
		t.outDirty = false

		return t.content
	}

	colWidths := t.ensureColWidths(numCols)
	t.ensureColumnANSI()
	t.ensureZoneMarkers()
	t.syncRowCache(colWidths, t.bordered)

	hasContent := len(t.cfg.Headers) > 0 || len(t.rows) > 0

	if hasContent {
		t.renderTopBorder(colWidths)
		t.renderHeaders(colWidths)
		t.renderRows()
		t.renderBottomBorder(colWidths)
	}

	t.outDirty = false

	return t.content
}

// RenderInto writes the table output into dst, one line per LinesBuf entry.
// Convenience wrapper — callers needing the table's own buffer can call
// Render() directly and use AppendFrom.
func (t *Table) RenderInto(dst *buffer.LinesBuf) {
	dst.AppendFrom(t.Render())
}

func (t *Table) deselectIfSelected() bool {
	if t.selectedIndex < 0 {
		return false
	}

	t.selectedIndex = -1
	t.outDirty = true

	return true
}

func (t *Table) findClickedRow(clickedID zeroterm.ZoneID) int {
	for idx := range len(t.rows) {
		if idx < len(t.zoneIDs) && t.zoneIDs[idx].Equal(clickedID) {
			return idx
		}
	}

	return -1
}

func (t *Table) ensureColWidths(numCols int) []int {
	if t.colWidthsCached && t.colWidths != nil {
		return t.colWidths
	}

	t.colWidths = t.distributeWidths(numCols)
	t.colWidthsCached = true

	return t.colWidths
}

func (t *Table) ensureColumnANSI() {
	if !t.colANSIOK {
		t.updateColumnANSI()
	}
}

func (t *Table) ensureZoneMarkers() {
	if t.zonePrefix != "" && (t.zoneStarts == nil || len(t.zoneStarts) != len(t.rows)) {
		t.updateZoneMarkers()
	}
}

func (t *Table) renderTopBorder(colWidths []int) {
	if !t.bordered {
		return
	}

	t.writeHorizontalBorder(
		t.cfg.Border.TopLeft, t.cfg.Border.TopMid, t.cfg.Border.TopRight,
		colWidths,
	)
	t.content.WriteLine(t.rowBuf.Bytes())
}

func (t *Table) renderHeaders(colWidths []int) {
	if len(t.cfg.Headers) == 0 {
		return
	}

	t.buildRow(t.cfg.Headers, colWidths, HeaderRow, t.bordered)
	t.content.WriteLine(t.rowBuf.Bytes())

	if t.bordered {
		t.writeHorizontalBorder(
			t.cfg.Border.LeftMid, t.cfg.Border.MidMid, t.cfg.Border.RightMid,
			colWidths,
		)
		t.content.WriteLine(t.rowBuf.Bytes())
	}
}

func (t *Table) renderRows() {
	for rowIdx, rowBytes := range t.rowCacheBytes {
		if t.zonePrefix != "" && rowIdx < len(t.zoneStarts) && rowIdx < len(t.zoneEnds) {
			t.content.WriteLine3(t.zoneStarts[rowIdx], rowBytes, t.zoneEnds[rowIdx])
		} else {
			t.content.WriteLine(rowBytes)
		}
	}
}

func (t *Table) renderBottomBorder(colWidths []int) {
	if !t.bordered {
		return
	}

	t.writeHorizontalBorder(
		t.cfg.Border.BottomLeft, t.cfg.Border.BottomMid, t.cfg.Border.BottomRight,
		colWidths,
	)
	t.content.WriteLine(t.rowBuf.Bytes())
}

func (t *Table) navigateLeft() bool {
	if t.selectedIndex > 0 {
		t.selectedIndex--
		t.outDirty = true

		return true
	}

	if t.selectedIndex < 0 && len(t.rows) > 0 {
		t.selectedIndex = 0
		t.outDirty = true

		return true
	}

	return false
}

func (t *Table) navigateRight() bool {
	if t.selectedIndex < 0 && len(t.rows) > 0 {
		t.selectedIndex = 0
		t.outDirty = true

		return true
	}

	if t.selectedIndex < len(t.rows)-1 {
		t.selectedIndex++
		t.outDirty = true

		return true
	}

	return false
}

func (t *Table) updateColumnANSI() {
	numCols := len(t.cfg.ColumnStyles)
	if numCols == 0 {
		t.colAlign = t.colAlign[:0]
		t.colANSIOK = true

		return
	}

	if cap(t.colAlign) < numCols {
		t.colAlign = make([]style.Position, numCols)
	} else {
		t.colAlign = t.colAlign[:numCols]
	}

	for i, sty := range t.cfg.ColumnStyles {
		t.colAlign[i] = sty.GetAlign()
	}

	t.colANSIOK = true
}

func (t *Table) updateZoneMarkers() {
	numRows := len(t.rows)
	if cap(t.zoneStarts) < numRows {
		t.zoneStarts = make([][]byte, numRows)
		t.zoneEnds = make([][]byte, numRows)
		t.zoneIDs = make([]zeroterm.ZoneID, numRows)
	} else {
		t.zoneStarts = t.zoneStarts[:numRows]
		t.zoneEnds = t.zoneEnds[:numRows]
		t.zoneIDs = t.zoneIDs[:numRows]
	}

	var openBuf, closeBuf [16]byte

	for rowIdx := range numRows {
		id := zeroterm.NewZoneID()
		t.zoneIDs[rowIdx] = id
		open := id.FormatOpen(openBuf[:0])
		t.zoneStarts[rowIdx] = append([]byte(nil), open...)
		closeMarker := id.FormatClose(closeBuf[:0])
		t.zoneEnds[rowIdx] = append([]byte(nil), closeMarker...)
	}
}

// syncRowCache ensures rowCacheBytes[i] contains the rendered bytes for
// rows[i]. Only rows whose data or selection state changed since the
// last render are re-rendered; all others reuse their cached bytes.
func (t *Table) syncRowCache(colWidths []int, hasBorder bool) {
	numRows := len(t.rows)
	selChanged := t.rowCacheSel != t.selectedIndex

	// Detect column width changes: even if cell data is unchanged,
	// different colWidths means every row renders differently.
	colWidthsChanged := len(t.rowCacheColWidths) != len(colWidths)
	if !colWidthsChanged {
		for i := range colWidths {
			if t.rowCacheColWidths[i] != colWidths[i] {
				colWidthsChanged = true

				break
			}
		}
	}

	// Full rebuild: structural change, row count change, or column
	// width change.
	if t.structuralChange || t.rowCacheBytes == nil ||
		len(t.rowCacheBytes) != numRows || len(t.rowCacheData) != numRows ||
		colWidthsChanged {
		t.fullRowCacheRebuild(colWidths, numRows, hasBorder)

		// Snapshot colWidths so we can detect changes next time.
		if cap(t.rowCacheColWidths) >= len(colWidths) {
			t.rowCacheColWidths = t.rowCacheColWidths[:len(colWidths)]
		} else {
			t.rowCacheColWidths = make([]int, len(colWidths))
		}

		copy(t.rowCacheColWidths, colWidths)
		t.structuralChange = false

		return
	}

	// Incremental: re-render only dirty rows.
	t.incrementalRowCacheUpdate(colWidths, selChanged, hasBorder)
}

func (t *Table) fullRowCacheRebuild(colWidths []int, numRows int, hasBorder bool) {
	if cap(t.rowCacheBytes) < numRows {
		t.rowCacheBytes = make([][]byte, numRows)
		t.rowCacheData = make([][][]byte, numRows)
	} else {
		t.rowCacheBytes = t.rowCacheBytes[:numRows]
		t.rowCacheData = t.rowCacheData[:numRows]
	}

	for rowIdx, row := range t.rows {
		t.buildRow(row, colWidths, rowIdx, hasBorder)

		rowBytes := t.rowBuf.Bytes()

		if cap(t.rowCacheBytes[rowIdx]) < len(rowBytes) {
			t.rowCacheBytes[rowIdx] = make([]byte, len(rowBytes))
		} else {
			t.rowCacheBytes[rowIdx] = t.rowCacheBytes[rowIdx][:len(rowBytes)]
		}

		copy(t.rowCacheBytes[rowIdx], rowBytes)
		t.rowCacheData[rowIdx] = row
	}

	t.rowCacheSel = t.selectedIndex
}

func (t *Table) incrementalRowCacheUpdate(colWidths []int, selChanged bool, hasBorder bool) {
	for rowIdx, row := range t.rows {
		dirty := !cellsEqual(t.rowCacheData[rowIdx], row)

		if !dirty && selChanged {
			dirty = rowIdx == t.rowCacheSel || rowIdx == t.selectedIndex
		}

		if dirty {
			t.buildRow(row, colWidths, rowIdx, hasBorder)

			rowBytes := t.rowBuf.Bytes()

			if cap(t.rowCacheBytes[rowIdx]) < len(rowBytes) {
				t.rowCacheBytes[rowIdx] = make([]byte, len(rowBytes))
			} else {
				t.rowCacheBytes[rowIdx] = t.rowCacheBytes[rowIdx][:len(rowBytes)]
			}

			copy(t.rowCacheBytes[rowIdx], rowBytes)
			t.rowCacheData[rowIdx] = row
		}
	}

	t.rowCacheSel = t.selectedIndex
}

// buildRow renders a single row into t.rowBuf.
func (t *Table) buildRow(cells [][]byte, colWidths []int,
	rowIdx int, hasBorder bool,
) {
	selected := rowIdx >= 0 && rowIdx == t.selectedIndex

	t.rowBuf.Reset()

	if hasBorder {
		t.rowBuf.Write(t.borderVertical)
	}

	if selected {
		t.selBgStyle.AppendStyledLine(t.rowBuf, nil)
	}

	innerBv := t.borderVertical
	if selected {
		innerBv = t.selBgBorderVertical
	}

	t.renderRowCells(cells, colWidths, hasBorder, selected, rowIdx, innerBv)

	if selected {
		t.selBgStyle.AppendStyledLine(t.rowBuf, nil)
	}

	if hasBorder {
		t.rowBuf.Write(t.borderVertical)
	}
}

func (t *Table) renderRowCells(cells [][]byte, colWidths []int, hasBorder bool, selected bool, rowIdx int, bv []byte) {
	for colIdx, colWidth := range colWidths {
		if colIdx > 0 && hasBorder {
			t.rowBuf.Write(bv)

			if selected {
				t.selBgStyle.AppendStyledLine(t.rowBuf, nil)
			}
		}

		cell := []byte(nil)
		if colIdx < len(cells) {
			cell = cells[colIdx]
		}

		align := t.cellAlign(rowIdx, colIdx)
		t.renderCell(cell, colWidth, colIdx, align, selected, rowIdx)
	}
}

func (t *Table) cellAlign(rowIdx, colIdx int) style.Position {
	if rowIdx == HeaderRow || colIdx >= len(t.colAlign) {
		return 0
	}

	return t.colAlign[colIdx]
}

// renderCell appends a single cell's content to t.rowBuf, handling alignment,
// truncation, wrapping, and ANSI sequences.
func (t *Table) renderCell(cell []byte, colWidth, colIdx int,
	align style.Position, selected bool, rowIdx int,
) {
	if hasNewline(cell) {
		var sty style.Style
		if rowIdx == HeaderRow {
			sty = style.NewStyle()
		} else {
			sty = t.columnStyle(colIdx)
		}

		if selected {
			sty = sty.Background(t.cfg.SelectionBackground)
		}

		sty = t.cellStyle(sty, colWidth)
		tmp := buffer.NewLinesBuf()
		sty.RenderInto(tmp, [][]byte{cell})

		for i := range tmp.Len() {
			if i > 0 {
				t.rowBuf.WriteByte('\n')
			}

			t.rowBuf.Write(tmp.Line(i))
		}

		tmp.Release()

		return
	}

	sty := t.composedCellStyle(rowIdx, colIdx, selected)
	cellWidth := style.CellWidth(cell)

	var selBg style.Style
	if selected {
		selBg = style.NewStyle().Background(t.cfg.SelectionBackground)
	}

	t.appendAlignedCell(cell, cellWidth, colWidth, align, sty, selBg)
}

// composedCellStyle returns the column style for a cell, optionally with
// selection background. The composed style's AppendStyledLine handles
// re-emitting the full prefix (col fg + sel bg) after every ANSI reset
// in the content.
func (t *Table) composedCellStyle(rowIdx, colIdx int, selected bool) style.Style {
	sty := style.NewStyle()
	if rowIdx != HeaderRow && colIdx < len(t.cfg.ColumnStyles) {
		sty = t.cfg.ColumnStyles[colIdx]
	}

	if selected {
		sty = sty.Background(t.cfg.SelectionBackground)
	}

	return sty
}

// appendAlignedCell appends a cell's styled content to t.rowBuf with proper
// alignment and truncation. The cell style is applied inline via
// AppendStyledLine, and padding spaces are styled via AppendStyledPad,
// eliminating per-cell allocations. selBg is applied to padding spaces when
// a row is selected.
func (t *Table) appendAlignedCell(cell []byte, cellWidth, colWidth int, align style.Position, sty, selBg style.Style) {
	switch {
	case cellWidth < colWidth:
		pad := colWidth - cellWidth

		switch {
		case align >= style.Right:
			selBg.AppendStyledPad(t.rowBuf, pad)
			sty.AppendStyledLine(t.rowBuf, cell)
		case align == style.Center:
			left := pad / 2
			right := pad - left
			selBg.AppendStyledPad(t.rowBuf, left)
			sty.AppendStyledLine(t.rowBuf, cell)
			selBg.AppendStyledPad(t.rowBuf, right)
		default:
			sty.AppendStyledLine(t.rowBuf, cell)
			selBg.AppendStyledPad(t.rowBuf, pad)
		}

	case cellWidth > colWidth:
		truncated := style.TruncateToWidth(cell, colWidth, !t.cfg.Wrap)
		sty.AppendStyledLine(t.rowBuf, truncated)

		remaining := colWidth - style.CellWidth(truncated)
		if remaining > 0 {
			selBg.AppendStyledPad(t.rowBuf, remaining)
		}

	default:
		sty.AppendStyledLine(t.rowBuf, cell)
	}
}

// cellStyle configures width and truncation for a cell that contains newlines.
func (t *Table) cellStyle(sty style.Style, colWidth int) style.Style {
	if t.cfg.Wrap {
		return sty.Width(colWidth)
	}

	return sty.Width(colWidth).MaxWidth(colWidth).TruncateEllipsis(true)
}

func (t *Table) numCols() int {
	if len(t.cfg.Headers) > 0 {
		return len(t.cfg.Headers)
	}

	if len(t.rows) > 0 {
		return len(t.rows[0])
	}

	return 0
}

func (t *Table) columnStyle(col int) style.Style {
	if col >= 0 && col < len(t.cfg.ColumnStyles) {
		return t.cfg.ColumnStyles[col]
	}

	return style.NewStyle()
}

// ensureWidthsBuf guarantees t.widthsBuf has capacity for at least
// 4*numCols ints and returns it zeroed and sliced to that length.
// Partition layout:
//
//	[0, numCols)           = contentWidths
//	[numCols, 2*numCols)   = fixedWidths
//	[2*numCols, 3*numCols) = distributed
//	[3*numCols, 4*numCols) = medians (for shrinkNonFixedColumns)
func (t *Table) ensureWidthsBuf(numCols int) {
	need := numCols * 4 //nolint:mnd
	if cap(t.widthsBuf) < need {
		t.widthsBuf = make([]int, need)
	} else {
		t.widthsBuf = t.widthsBuf[:need]
	}

	for i := range t.widthsBuf {
		t.widthsBuf[i] = 0
	}
}

func (t *Table) contentWidths(numCols int, dst []int) {
	for i := range dst {
		dst[i] = 0
	}

	for i, h := range t.cfg.Headers {
		if i < numCols {
			w := style.CellWidth(h)
			if w > dst[i] {
				dst[i] = w
			}
		}
	}

	for _, row := range t.rows {
		for i, cell := range row {
			if i < numCols {
				w := style.CellWidth(cell)
				if w > dst[i] {
					dst[i] = w
				}
			}
		}
	}
}

// distributeWidths calculates column widths that exactly fill the table
// width. When content is narrower than the table, columns expand. When
// content is wider, columns shrink — prioritizing shrinking the widest
// columns first (matching lipgloss behavior).
func (t *Table) distributeWidths(numCols int) []int {
	if numCols == 0 {
		return nil
	}

	t.ensureWidthsBuf(numCols)
	buf := t.widthsBuf

	contentWidths := buf[0:numCols]
	fixedWidths := buf[numCols : 2*numCols]
	distributed := buf[2*numCols : 3*numCols]

	t.contentWidths(numCols, contentWidths)
	t.computeFixedWidths(numCols, fixedWidths)

	if t.cfg.Width <= 0 {
		return t.distributeWidthsNoTableWidth(numCols, contentWidths, fixedWidths)
	}

	availableWidth := max(t.cfg.Width-t.totalBorderWidth(numCols), 0)

	copy(distributed, contentWidths)

	fixedTotal, nonFixedContent := t.applyFixedWidths(numCols, distributed, fixedWidths)
	nonFixedAvailable := max(availableWidth-fixedTotal, 0)

	return t.resolveDistributedWidths(numCols, buf, distributed, contentWidths, fixedWidths, nonFixedAvailable, nonFixedContent)
}

func (t *Table) computeFixedWidths(numCols int, fixedWidths []int) {
	for i := range numCols {
		fw := t.columnStyle(i).GetWidth()
		if fw > 0 {
			fixedWidths[i] = fw
		}
	}
}

func (t *Table) applyFixedWidths(numCols int, distributed, fixedWidths []int) (int, int) {
	fixedTotal := 0
	nonFixedContent := 0

	for i := range numCols {
		if fixedWidths[i] > 0 {
			distributed[i] = fixedWidths[i]
			fixedTotal += fixedWidths[i]
		} else {
			nonFixedContent += distributed[i]
		}
	}

	return fixedTotal, nonFixedContent
}

func (t *Table) resolveDistributedWidths(
	numCols int, buf, distributed, contentWidths, fixedWidths []int,
	nonFixedAvailable, nonFixedContent int,
) []int {
	if nonFixedContent != nonFixedAvailable {
		if nonFixedContent < nonFixedAvailable {
			t.growNonFixedColumns(distributed, fixedWidths, nonFixedAvailable)
		} else {
			medians := buf[3*numCols : 4*numCols]
			t.shrinkNonFixedColumns(distributed, contentWidths, fixedWidths, medians, nonFixedAvailable)
		}
	}

	if cap(t.colWidths) < numCols {
		t.colWidths = make([]int, numCols)
	} else {
		t.colWidths = t.colWidths[:numCols]
	}

	copy(t.colWidths, distributed)

	return t.colWidths
}

// distributeWidthsNoTableWidth returns column widths when no table width is set.
func (t *Table) distributeWidthsNoTableWidth(numCols int, contentWidths, fixedWidths []int) []int {
	if cap(t.colWidths) < numCols {
		t.colWidths = make([]int, numCols)
	} else {
		t.colWidths = t.colWidths[:numCols]
	}

	for i := range numCols {
		if fixedWidths[i] > 0 {
			t.colWidths[i] = fixedWidths[i]
		} else {
			t.colWidths[i] = contentWidths[i]
		}
	}

	return t.colWidths
}

func (t *Table) totalBorderWidth(numCols int) int {
	if !t.bordered {
		return 0
	}

	// left + right + column separators
	return 2 + numCols - 1
}

// growNonFixedColumns distributes surplus width to the shortest non-fixed columns
// until the total reaches nonFixedAvailable.
func (t *Table) growNonFixedColumns(distributed, fixedWidths []int, nonFixedAvailable int) {
	for {
		total := 0

		for i := range distributed {
			if fixedWidths[i] == 0 {
				total += distributed[i]
			}
		}

		if total >= nonFixedAvailable {
			break
		}

		shortestIdx := -1
		shortestW := 0

		for i, w := range distributed {
			if fixedWidths[i] == 0 {
				if shortestIdx < 0 || w < shortestW {
					shortestW = w
					shortestIdx = i
				}
			}
		}

		if shortestIdx < 0 {
			break
		}

		distributed[shortestIdx]++
	}
}

func (t *Table) shrinkNonFixedColumns(distributed, contentWidths, fixedWidths, medians []int, availableWidth int) {
	t.shrinkWideColumns(distributed, fixedWidths, availableWidth)
	t.computeMedians(medians, contentWidths, fixedWidths)
	t.shrinkByMedianDeviation(distributed, fixedWidths, medians, availableWidth)
	t.shrinkAnyRemaining(distributed, fixedWidths, availableWidth)
}

func (t *Table) shrinkWideColumns(distributed, fixedWidths []int, availableWidth int) {
	for {
		total := nonFixedTotal(distributed, fixedWidths)
		if total <= availableWidth {
			break
		}

		bigIdx := biggestNonFixedAtLeastHalf(distributed, fixedWidths, availableWidth)
		if bigIdx < 0 || distributed[bigIdx] == 0 {
			break
		}

		distributed[bigIdx]--
	}
}

func (t *Table) computeMedians(medians, contentWidths, fixedWidths []int) {
	for i := range medians {
		if fixedWidths[i] == 0 {
			medians[i] = max(contentWidths[i]/2, 1)
		}
	}
}

func (t *Table) shrinkByMedianDeviation(distributed, fixedWidths, medians []int, availableWidth int) {
	for {
		total := nonFixedTotal(distributed, fixedWidths)
		if total <= availableWidth {
			break
		}

		bigDiffIdx := biggestMedianDeviation(distributed, fixedWidths, medians)
		if bigDiffIdx < 0 || distributed[bigDiffIdx] == 0 {
			break
		}

		distributed[bigDiffIdx]--
	}
}

func (t *Table) shrinkAnyRemaining(distributed, fixedWidths []int, availableWidth int) {
	for {
		total := nonFixedTotal(distributed, fixedWidths)
		if total <= availableWidth {
			break
		}

		bigIdx := biggestNonFixed(distributed, fixedWidths)
		if bigIdx < 0 || distributed[bigIdx] == 0 {
			break
		}

		distributed[bigIdx]--
	}
}

func nonFixedTotal(distributed, fixedWidths []int) int {
	total := 0

	for i, w := range distributed {
		if fixedWidths[i] == 0 {
			total += w
		}
	}

	return total
}

func biggestNonFixedAtLeastHalf(distributed, fixedWidths []int, availableWidth int) int {
	bigIdx := -1
	bigW := -1

	for i, w := range distributed {
		if fixedWidths[i] == 0 && w >= availableWidth/2 && w > bigW {
			bigW = w
			bigIdx = i
		}
	}

	return bigIdx
}

func biggestMedianDeviation(distributed, fixedWidths, medians []int) int {
	bigDiffIdx := -1
	bigDiff := -1

	for i, w := range distributed {
		if fixedWidths[i] == 0 {
			diff := w - medians[i]
			if diff > 0 && diff > bigDiff {
				bigDiff = diff
				bigDiffIdx = i
			}
		}
	}

	return bigDiffIdx
}

func biggestNonFixed(distributed, fixedWidths []int) int {
	bigIdx := -1
	bigW := -1

	for i, w := range distributed {
		if fixedWidths[i] == 0 && w > bigW {
			bigW = w
			bigIdx = i
		}
	}

	return bigIdx
}

// writeHorizontalBorder builds a horizontal border line into t.rowBuf.
func (t *Table) writeHorizontalBorder(left, mid, right []byte,
	colWidths []int,
) {
	styledLeft := t.cfg.BorderStyle.RenderLine(left)

	t.rowBuf.Reset()
	t.rowBuf.Write(styledLeft)

	styledMid := t.cfg.BorderStyle.RenderLine(mid)

	for colIdx, colWidth := range colWidths {
		if colIdx > 0 {
			t.rowBuf.Write(styledMid)
		}

		if colWidth > 0 {
			horizBytes := appendRepeatBytes(nil, t.cfg.Border.Horizontal, colWidth)
			t.rowBuf.Write(t.cfg.BorderStyle.RenderLine(horizBytes))
		}
	}

	t.rowBuf.Write(t.cfg.BorderStyle.RenderLine(right))
}

func appendRepeatBytes(buf []byte, b []byte, n int) []byte {
	for n > 0 {
		buf = append(buf, b...)
		n--
	}

	return buf
}

func cellsEqual(left, right [][]byte) bool {
	if len(left) != len(right) {
		return false
	}

	for idx := range left {
		if !bytes.Equal(left[idx], right[idx]) {
			return false
		}
	}

	return true
}

func hasNewline(b []byte) bool {
	return slices.Contains(b, '\n')
}
