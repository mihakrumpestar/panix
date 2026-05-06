// Based on charm.land/lipgloss/v2/table — Copyright (c) 2021-2026 Charmbracelet, Inc.
// Licensed under the MIT License. See pkg/LICENSE for details.

package table

import (
	"fmt"
	"strconv"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

const HeaderRow = -1

type Table struct {
	width        int
	border       style.Border
	borderSty    style.Style
	headers      []string
	rows         [][]string
	columnStyles []style.Style
	wrap         bool
	borderTop    bool
	borderRight  bool
	borderBottom bool
	borderLeft   bool
	borderColumn bool

	selectedIndex int
	selBgPrefix   string
	zonePrefix    string

	colWidths       []int
	colWidthsCached bool

	// Pre-computed column ANSI styles and alignment. Computed once,
	// reused for every cell render.
	colANSI   []style.ANSIStyle
	colAlign  []style.Position
	colANSIOK bool

	// Full output cache. When outDirty is false, String() returns
	// outCache immediately with zero allocations.
	outCache string
	outDirty bool

	// Reusable byte buffers. rowBuf is used by buildRow; outBuf is
	// used by String().
	rowBuf []byte
	outBuf []byte

	// Pre-computed zone markers per row. Built once when rows or
	// zone prefix change, reused every render. nil means "needs
	// recomputation".
	zoneStarts []string
	zoneEnds   []string

	// Per-row render cache: cached rendered bytes and the data that
	// produced them. On selection-only changes, only the 2 affected
	// rows are re-rendered instead of all N.
	rowCacheBytes [][]byte
	rowCacheData  [][]string
	rowCacheSel   int

	// Reusable buffer for distributeWidths: partitioned into regions
	// for contentWidths, fixedWidths, distributed, and medians.
	// Grows once, reused forever. Eliminates 4 make([]int) per call.
	widthsBuf []int
}

func New() *Table {
	return &Table{
		borderTop:     true,
		borderRight:   true,
		borderBottom:  true,
		borderLeft:    true,
		borderColumn:  true,
		selectedIndex: -1,
		outDirty:      true,
		rowCacheSel:   -2, // force mismatch on first render
	}
}

func (t *Table) Width(w int) *Table {
	if t.width == w {
		return t
	}

	t.width = w
	t.colWidthsCached = false
	t.outDirty = true
	t.rowCacheBytes = nil // structural change, invalidate all

	return t
}

func (t *Table) Border(b style.Border) *Table {
	t.border = b
	t.colWidthsCached = false
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

func (t *Table) Borders(top, right, bottom, left bool) *Table {
	t.borderTop = top
	t.borderRight = right
	t.borderBottom = bottom
	t.borderLeft = left
	t.colWidthsCached = false
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

func (t *Table) BorderColumn(v bool) *Table {
	t.borderColumn = v
	t.colWidthsCached = false
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

func (t *Table) BorderStyle(s style.Style) *Table {
	t.borderSty = s
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

func (t *Table) Headers(h ...string) *Table {
	t.headers = h
	t.colWidthsCached = false
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

func (t *Table) Row(cells ...string) *Table {
	t.rows = append(t.rows, cells)
	t.colWidthsCached = false
	t.outDirty = true
	t.zoneStarts = nil
	t.rowCacheBytes = nil

	return t
}

func (t *Table) Rows(rows ...[]string) *Table {
	t.rows = append(t.rows, rows...)
	t.colWidthsCached = false
	t.outDirty = true
	t.zoneStarts = nil
	t.rowCacheBytes = nil

	return t
}

func (t *Table) ColumnStyles(styles []style.Style) *Table {
	t.columnStyles = styles
	t.colANSIOK = false
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

func (t *Table) Wrap(v bool) *Table {
	t.wrap = v
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

func (t *Table) SelectionBackground(c style.Color) *Table {
	t.selBgPrefix = style.ColorToBgPrefix(c)
	t.outDirty = true
	t.rowCacheBytes = nil

	return t
}

// SetRows replaces all row data. Selection is preserved. Detects
// identical data and skips cache invalidation, avoiding all
// allocations for the common "same data, re-render" pattern.
func (t *Table) SetRows(rows [][]string) *Table {
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
	t.zoneStarts = nil
	t.rowCacheBytes = nil

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

func (t *Table) ZonePrefix() string {
	return t.zonePrefix
}

func (t *Table) SetZonePrefix(prefix string) *Table {
	t.zonePrefix = prefix
	t.outDirty = true
	t.zoneStarts = nil
	t.rowCacheBytes = nil

	return t
}

// HandleMouseClick checks if a mouse click landed on a data row and
// updates the selection accordingly. Clicking outside any row zone
// deselects the current selection. Returns true if the selection state
// was changed.
func (t *Table) HandleMouseClick(msg zeroterm.MouseClickMsg) bool {
	if t.zonePrefix == "" || len(t.rows) == 0 {
		return false
	}

	lines := zeroterm.CurrentLines()
	if msg.Y < 0 || msg.Y >= len(lines) {
		if t.selectedIndex >= 0 {
			t.selectedIndex = -1
			t.outDirty = true

			return true
		}

		return false
	}

	for idx := range len(t.rows) {
		zoneName := fmt.Sprintf("%s-%d", t.zonePrefix, idx)
		if zeroterm.IsZoneAtLine(lines[msg.Y], msg.X, zoneName) {
			if t.selectedIndex != idx {
				t.selectedIndex = idx
				t.outDirty = true

				return true
			}

			return false
		}
	}

	if t.selectedIndex >= 0 {
		t.selectedIndex = -1
		t.outDirty = true

		return true
	}

	return false
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

//nolint:cyclop,funlen
func (t *Table) String() string {
	if !t.outDirty && t.outCache != "" {
		return t.outCache
	}

	numCols := t.numCols()
	if numCols == 0 {
		t.outCache = ""
		t.outDirty = false

		return ""
	}

	var colWidths []int

	if t.colWidthsCached && t.colWidths != nil {
		colWidths = t.colWidths
	} else {
		colWidths = t.distributeWidths(numCols)
		t.colWidths = colWidths
		t.colWidthsCached = true
	}

	if !t.colANSIOK {
		t.updateColumnANSI()
	}

	hasBorder := t.border.Vertical != ""

	bfg := t.borderSty.FgPrefix()
	if bfg == "" {
		bfg = t.borderSty.BgPrefix()
	}

	borderReset := ""
	if bfg != "" {
		borderReset = style.ANSIReset()
	}

	if t.zonePrefix != "" && (t.zoneStarts == nil || len(t.zoneStarts) != len(t.rows)) {
		t.updateZoneMarkers()
	}

	// Per-row diff: re-render only rows whose data or selection
	// state changed. On selection-only changes, just 2 rows are
	// re-rendered instead of all N.
	t.syncRowCache(colWidths, hasBorder, bfg, borderReset)

	// Assemble output from cached row bytes.
	t.outBuf = t.outBuf[:0]

	hasContent := len(t.headers) > 0 || len(t.rows) > 0

	if hasBorder && t.borderTop && hasContent {
		t.writeHorizontalBorder(
			t.border.TopLeft, t.border.TopMid, t.border.TopRight,
			colWidths, bfg, borderReset,
		)
	}

	if len(t.headers) > 0 {
		t.buildRow(t.headers, colWidths, HeaderRow, hasBorder, bfg, borderReset)
		t.outBuf = append(t.outBuf, t.rowBuf...)
		t.outBuf = append(t.outBuf, '\n')

		if hasBorder && t.borderColumn {
			t.writeHorizontalBorder(
				t.border.LeftMid, t.border.MidMid, t.border.RightMid,
				colWidths, bfg, borderReset,
			)
		}
	}

	for rowIdx, rowBytes := range t.rowCacheBytes {
		if t.zonePrefix != "" && rowIdx < len(t.zoneStarts) {
			t.outBuf = append(t.outBuf, t.zoneStarts[rowIdx]...)
		}

		t.outBuf = append(t.outBuf, rowBytes...)

		if t.zonePrefix != "" && rowIdx < len(t.zoneEnds) {
			t.outBuf = append(t.outBuf, t.zoneEnds[rowIdx]...)
		}

		t.outBuf = append(t.outBuf, '\n')
	}

	if hasBorder && t.borderBottom && hasContent {
		t.writeHorizontalBorder(
			t.border.BottomLeft, t.border.BottomMid, t.border.BottomRight,
			colWidths, bfg, borderReset,
		)
	}

	if len(t.outBuf) > 0 && t.outBuf[len(t.outBuf)-1] == '\n' {
		t.outBuf = t.outBuf[:len(t.outBuf)-1]
	}

	// Convert buffer to string. This is the ONE allocation in the
	// hot path — unavoidable because Go's string([]byte) always
	// copies. The alternative (unsafe.String + buffer donation)
	// eliminates the copy but forces a new buffer allocation next
	// frame, creating GC pressure. Buffer reuse + copy is better.
	t.outCache = string(t.outBuf)
	t.outDirty = false

	return t.outCache
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
	numCols := len(t.columnStyles)
	if numCols == 0 {
		t.colANSI = t.colANSI[:0]
		t.colAlign = t.colAlign[:0]
		t.colANSIOK = true

		return
	}

	if cap(t.colANSI) < numCols {
		t.colANSI = make([]style.ANSIStyle, numCols)
		t.colAlign = make([]style.Position, numCols)
	} else {
		t.colANSI = t.colANSI[:numCols]
		t.colAlign = t.colAlign[:numCols]
	}

	for i, sty := range t.columnStyles {
		t.colANSI[i] = style.NewANSIStyle(sty)
		t.colAlign[i] = sty.GetAlign()
	}

	t.colANSIOK = true
}

func (t *Table) updateZoneMarkers() {
	numRows := len(t.rows)
	if cap(t.zoneStarts) < numRows {
		t.zoneStarts = make([]string, numRows)
		t.zoneEnds = make([]string, numRows)
	} else {
		t.zoneStarts = t.zoneStarts[:numRows]
		t.zoneEnds = t.zoneEnds[:numRows]
	}

	for rowIdx := range numRows {
		zoneName := t.zonePrefix + "-" + strconv.Itoa(rowIdx)
		id := zeroterm.EnsureZone(zoneName)
		t.zoneStarts[rowIdx] = "\x1b[" + strconv.Itoa(int(id)) + "z"
		t.zoneEnds[rowIdx] = "\x1b[/" + strconv.Itoa(int(id)) + "z"
	}
}

// syncRowCache ensures rowCacheBytes[i] contains the rendered bytes for
// rows[i]. Only rows whose data or selection state changed since the
// last render are re-rendered; all others reuse their cached bytes.
func (t *Table) syncRowCache(colWidths []int, hasBorder bool, bfg, borderReset string) {
	numRows := len(t.rows)
	selChanged := t.rowCacheSel != t.selectedIndex

	// Full rebuild: structural change (row count, width, etc.)
	if t.rowCacheBytes == nil || len(t.rowCacheBytes) != numRows ||
		len(t.rowCacheData) != numRows {
		t.fullRowCacheRebuild(colWidths, numRows, hasBorder, bfg, borderReset)

		return
	}

	// Incremental: re-render only dirty rows.
	t.incrementalRowCacheUpdate(colWidths, selChanged, hasBorder, bfg, borderReset)
}

func (t *Table) fullRowCacheRebuild(colWidths []int, numRows int, hasBorder bool, bfg, borderReset string) {
	if cap(t.rowCacheBytes) < numRows {
		t.rowCacheBytes = make([][]byte, numRows)
		t.rowCacheData = make([][]string, numRows)
	} else {
		t.rowCacheBytes = t.rowCacheBytes[:numRows]
		t.rowCacheData = t.rowCacheData[:numRows]
	}

	for rowIdx, row := range t.rows {
		t.buildRow(row, colWidths, rowIdx, hasBorder, bfg, borderReset)
		buf := make([]byte, len(t.rowBuf))
		copy(buf, t.rowBuf)
		t.rowCacheBytes[rowIdx] = buf
		t.rowCacheData[rowIdx] = row
	}

	t.rowCacheSel = t.selectedIndex
}

func (t *Table) incrementalRowCacheUpdate(colWidths []int, selChanged bool, hasBorder bool, bfg, borderReset string) {
	for rowIdx, row := range t.rows {
		dirty := !cellsEqual(t.rowCacheData[rowIdx], row)

		if !dirty && selChanged {
			// Selection affects rendering only for the previously-
			// selected and newly-selected rows.
			dirty = rowIdx == t.rowCacheSel || rowIdx == t.selectedIndex
		}

		if dirty {
			t.buildRow(row, colWidths, rowIdx, hasBorder, bfg, borderReset)
			// Reuse existing byte slice if it fits, else allocate.
			if cap(t.rowCacheBytes[rowIdx]) < len(t.rowBuf) {
				t.rowCacheBytes[rowIdx] = make([]byte, len(t.rowBuf))
			} else {
				t.rowCacheBytes[rowIdx] = t.rowCacheBytes[rowIdx][:len(t.rowBuf)]
			}

			copy(t.rowCacheBytes[rowIdx], t.rowBuf)
			t.rowCacheData[rowIdx] = row
		}
	}

	t.rowCacheSel = t.selectedIndex
}

// buildRow renders a single row into t.rowBuf using inline cell
// rendering that bypasses the heavy Style.Render pipeline.
//
// Selection background is injected inline: after every ANSI reset
// inside the inner content, selBgPrefix is re-emitted so the
// background spans uninterrupted between the outer borders.
func (t *Table) buildRow(cells []string, colWidths []int,
	rowIdx int, hasBorder bool, bfg, borderReset string,
) {
	selBg := ""
	if rowIdx >= 0 && rowIdx == t.selectedIndex {
		selBg = t.selBgPrefix
	}

	t.rowBuf = t.rowBuf[:0]

	if hasBorder && t.borderLeft {
		t.rowBuf = append(t.rowBuf, bfg...)
		t.rowBuf = append(t.rowBuf, t.border.Vertical...)
		t.rowBuf = append(t.rowBuf, borderReset...)
	}

	if selBg != "" {
		t.rowBuf = append(t.rowBuf, selBg...)
	}

	t.rowBuf = t.renderRowCells(cells, colWidths, hasBorder, bfg, borderReset, selBg, rowIdx)

	if selBg != "" {
		t.rowBuf = append(t.rowBuf, style.ANSIReset()...)
	}

	if hasBorder && t.borderRight {
		t.rowBuf = append(t.rowBuf, bfg...)
		t.rowBuf = append(t.rowBuf, t.border.Vertical...)
		t.rowBuf = append(t.rowBuf, borderReset...)
	}
}

func (t *Table) renderRowCells(cells []string, colWidths []int, hasBorder bool, bfg, borderReset, selBg string, rowIdx int) []byte {
	buf := t.rowBuf

	for colIdx, colWidth := range colWidths {
		if colIdx > 0 && hasBorder && t.borderColumn {
			buf = append(buf, bfg...)
			buf = append(buf, t.border.Vertical...)
			buf = append(buf, borderReset...)

			if selBg != "" {
				buf = append(buf, selBg...)
			}
		}

		cell := ""
		if colIdx < len(cells) {
			cell = cells[colIdx]
		}

		prefix, reset, align := t.cellANSI(rowIdx, colIdx)
		buf = t.renderCell(buf, cell, colWidth, colIdx, prefix, reset, align, selBg, rowIdx)
	}

	return buf
}

// cellANSI returns the ANSI prefix, reset, and alignment for a cell.
func (t *Table) cellANSI(rowIdx, colIdx int) (string, string, style.Position) {
	if rowIdx == HeaderRow || colIdx >= len(t.colANSI) {
		return "", "", 0
	}

	return t.colANSI[colIdx].Prefix(), t.colANSI[colIdx].Reset(), t.colAlign[colIdx]
}

// renderCell appends a single cell's content to buf, handling alignment,
// truncation, wrapping, and ANSI sequences.
func (t *Table) renderCell(
	buf []byte, cell string, colWidth, colIdx int,
	prefix, reset string, align style.Position, selBg string, rowIdx int,
) []byte {
	if hasNewline(cell) {
		var sty style.Style
		if rowIdx == HeaderRow {
			sty = style.NewStyle()
		} else {
			sty = t.columnStyle(colIdx)
		}

		sty = t.cellStyle(sty, colWidth)
		rendered := sty.Render(cell)
		buf = append(buf, rendered...)

		return buf
	}

	if prefix != "" {
		buf = append(buf, prefix...)
	}

	buf = t.alignCellContent(buf, cell, colWidth, align)

	if reset != "" {
		buf = append(buf, reset...)

		if selBg != "" {
			buf = append(buf, selBg...)
		}
	}

	return buf
}

// alignCellContent appends cell content to buf with proper alignment and truncation.
func (t *Table) alignCellContent(buf []byte, cell string, colWidth int, align style.Position) []byte {
	cellWidth := style.CellWidth(cell)

	switch {
	case cellWidth < colWidth:
		pad := colWidth - cellWidth

		switch {
		case align >= style.Right:
			buf = appendPad(buf, pad)
			buf = append(buf, cell...)
		case align == style.Center:
			left := pad / 2 //nolint:mnd
			right := pad - left
			buf = appendPad(buf, left)
			buf = append(buf, cell...)
			buf = appendPad(buf, right)
		default:
			buf = append(buf, cell...)
			buf = appendPad(buf, pad)
		}

	case cellWidth > colWidth:
		truncated := style.TruncateToWidth(cell, colWidth, !t.wrap)
		buf = append(buf, truncated...)

		remaining := colWidth - style.CellWidth(truncated)
		if remaining > 0 {
			buf = appendPad(buf, remaining)
		}

	default:
		buf = append(buf, cell...)
	}

	return buf
}

// cellStyle configures width and truncation for a cell that contains newlines.
func (t *Table) cellStyle(sty style.Style, colWidth int) style.Style {
	if t.wrap {
		return sty.Width(colWidth)
	}

	return sty.Width(colWidth).MaxWidth(colWidth).TruncateEllipsis(true)
}

func appendPad(buf []byte, n int) []byte {
	for range n {
		buf = append(buf, ' ')
	}

	return buf
}

func (t *Table) numCols() int {
	if len(t.headers) > 0 {
		return len(t.headers)
	}

	if len(t.rows) > 0 {
		return len(t.rows[0])
	}

	return 0
}

func (t *Table) columnStyle(col int) style.Style {
	if col >= 0 && col < len(t.columnStyles) {
		return t.columnStyles[col]
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

	for i, h := range t.headers {
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

	if t.width <= 0 {
		return t.distributeWidthsNoTableWidth(numCols, contentWidths, fixedWidths)
	}

	availableWidth := max(t.width-t.totalBorderWidth(numCols), 0)

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
	if nonFixedContent == nonFixedAvailable {
		result := make([]int, numCols)
		copy(result, distributed)

		return result
	}

	if nonFixedContent < nonFixedAvailable {
		t.growNonFixedColumns(distributed, fixedWidths, nonFixedAvailable)
	} else {
		medians := buf[3*numCols : 4*numCols]
		t.shrinkNonFixedColumns(distributed, contentWidths, fixedWidths, medians, nonFixedAvailable)
	}

	result := make([]int, numCols)
	copy(result, distributed)

	return result
}

// distributeWidthsNoTableWidth returns column widths when no table width is set.
func (t *Table) distributeWidthsNoTableWidth(numCols int, contentWidths, fixedWidths []int) []int {
	result := make([]int, numCols)
	for i := range numCols {
		if fixedWidths[i] > 0 {
			result[i] = fixedWidths[i]
		} else {
			result[i] = contentWidths[i]
		}
	}

	return result
}

func (t *Table) totalBorderWidth(numCols int) int {
	if t.border.Vertical == "" {
		return 0
	}

	borderWidth := 0

	if t.borderLeft {
		borderWidth++
	}

	if t.borderRight {
		borderWidth++
	}

	if t.borderColumn {
		borderWidth += numCols - 1
	}

	return borderWidth
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
			medians[i] = max(contentWidths[i]/2, 1) //nolint:mnd
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

func (t *Table) writeHorizontalBorder(left, mid, right string,
	colWidths []int, fg, reset string,
) {
	t.outBuf = append(t.outBuf, fg...)
	t.outBuf = append(t.outBuf, left...)

	for colIdx, colWidth := range colWidths {
		if colIdx > 0 && t.borderColumn {
			t.outBuf = append(t.outBuf, mid...)
		}

		if colWidth > 0 {
			t.outBuf = appendRepeatStr(t.outBuf, t.border.Horizontal, colWidth)
		}
	}

	t.outBuf = append(t.outBuf, right...)
	t.outBuf = append(t.outBuf, reset...)
	t.outBuf = append(t.outBuf, '\n')
}

func appendRepeatStr(buf []byte, s string, n int) []byte {
	for n > 0 {
		buf = append(buf, s...)
		n--
	}

	return buf
}

func cellsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}

	return true
}

func hasNewline(s string) bool {
	for i := range len(s) {
		if s[i] == '\n' {
			return true
		}
	}

	return false
}
