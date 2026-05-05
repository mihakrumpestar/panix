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
		borderTop:    true,
		borderRight:  true,
		borderBottom: true,
		borderLeft:   true,
		borderColumn: true,
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
	case "right":
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
	}

	return false
}

func (t *Table) updateColumnANSI() {
	n := len(t.columnStyles)
	if n == 0 {
		t.colANSI = t.colANSI[:0]
		t.colAlign = t.colAlign[:0]
		t.colANSIOK = true

		return
	}

	if cap(t.colANSI) < n {
		t.colANSI = make([]style.ANSIStyle, n)
		t.colAlign = make([]style.Position, n)
	} else {
		t.colANSI = t.colANSI[:n]
		t.colAlign = t.colAlign[:n]
	}

	for i, sty := range t.columnStyles {
		t.colANSI[i] = style.NewANSIStyle(sty)
		t.colAlign[i] = sty.GetAlign()
	}

	t.colANSIOK = true
}

func (t *Table) updateZoneMarkers() {
	n := len(t.rows)
	if cap(t.zoneStarts) < n {
		t.zoneStarts = make([]string, n)
		t.zoneEnds = make([]string, n)
	} else {
		t.zoneStarts = t.zoneStarts[:n]
		t.zoneEnds = t.zoneEnds[:n]
	}

	for i := range n {
		zoneName := t.zonePrefix + "-" + strconv.Itoa(i)
		id := zeroterm.EnsureZone(zoneName)
		t.zoneStarts[i] = "\x1b[" + strconv.Itoa(int(id)) + "z"
		t.zoneEnds[i] = "\x1b[/" + strconv.Itoa(int(id)) + "z"
	}
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

	for i, rowBytes := range t.rowCacheBytes {
		if t.zonePrefix != "" && i < len(t.zoneStarts) {
			t.outBuf = append(t.outBuf, t.zoneStarts[i]...)
		}

		t.outBuf = append(t.outBuf, rowBytes...)

		if t.zonePrefix != "" && i < len(t.zoneEnds) {
			t.outBuf = append(t.outBuf, t.zoneEnds[i]...)
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

// syncRowCache ensures rowCacheBytes[i] contains the rendered bytes for
// rows[i]. Only rows whose data or selection state changed since the
// last render are re-rendered; all others reuse their cached bytes.
func (t *Table) syncRowCache(colWidths []int, hasBorder bool, bfg, borderReset string) {
	n := len(t.rows)
	selChanged := t.rowCacheSel != t.selectedIndex

	// Full rebuild: structural change (row count, width, etc.)
	if t.rowCacheBytes == nil || len(t.rowCacheBytes) != n ||
		len(t.rowCacheData) != n {
		if cap(t.rowCacheBytes) < n {
			t.rowCacheBytes = make([][]byte, n)
			t.rowCacheData = make([][]string, n)
		} else {
			t.rowCacheBytes = t.rowCacheBytes[:n]
			t.rowCacheData = t.rowCacheData[:n]
		}

		for i, row := range t.rows {
			t.buildRow(row, colWidths, i, hasBorder, bfg, borderReset)
			buf := make([]byte, len(t.rowBuf))
			copy(buf, t.rowBuf)
			t.rowCacheBytes[i] = buf
			t.rowCacheData[i] = row
		}

		t.rowCacheSel = t.selectedIndex

		return
	}

	// Incremental: re-render only dirty rows.
	for i, row := range t.rows {
		dirty := !cellsEqual(t.rowCacheData[i], row)

		if !dirty && selChanged {
			// Selection affects rendering only for the previously-
			// selected and newly-selected rows.
			dirty = i == t.rowCacheSel || i == t.selectedIndex
		}

		if dirty {
			t.buildRow(row, colWidths, i, hasBorder, bfg, borderReset)
			// Reuse existing byte slice if it fits, else allocate.
			if cap(t.rowCacheBytes[i]) < len(t.rowBuf) {
				t.rowCacheBytes[i] = make([]byte, len(t.rowBuf))
			} else {
				t.rowCacheBytes[i] = t.rowCacheBytes[i][:len(t.rowBuf)]
			}

			copy(t.rowCacheBytes[i], t.rowBuf)
			t.rowCacheData[i] = row
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

	for i, w := range colWidths {
		if i > 0 && hasBorder && t.borderColumn {
			t.rowBuf = append(t.rowBuf, bfg...)
			t.rowBuf = append(t.rowBuf, t.border.Vertical...)
			t.rowBuf = append(t.rowBuf, borderReset...)

			if selBg != "" {
				t.rowBuf = append(t.rowBuf, selBg...)
			}
		}

		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}

		var prefix, reset string
		var align style.Position

		if rowIdx == HeaderRow || i >= len(t.colANSI) {
			// No ANSI wrapping for headers or beyond column styles.
		} else {
			prefix = t.colANSI[i].Prefix()
			reset = t.colANSI[i].Reset()
			align = t.colAlign[i]
		}

		if hasNewline(cell) {
			var sty style.Style
			if rowIdx == HeaderRow {
				sty = style.NewStyle()
			} else {
				sty = t.columnStyle(i)
			}

			if t.wrap {
				sty = sty.Width(w)
			} else {
				sty = sty.Width(w).MaxWidth(w).TruncateEllipsis(true)
			}

			rendered := sty.Render(cell)
			t.rowBuf = append(t.rowBuf, rendered...)

			continue
		}

		if prefix != "" {
			t.rowBuf = append(t.rowBuf, prefix...)
		}

		cw := style.CellWidth(cell)

		switch {
		case cw < w:
			pad := w - cw
			switch {
			case align >= style.Right:
				t.rowBuf = appendPad(t.rowBuf, pad)
				t.rowBuf = append(t.rowBuf, cell...)
			case align == style.Center:
				left := pad / 2
				right := pad - left
				t.rowBuf = appendPad(t.rowBuf, left)
				t.rowBuf = append(t.rowBuf, cell...)
				t.rowBuf = appendPad(t.rowBuf, right)
			default:
				t.rowBuf = append(t.rowBuf, cell...)
				t.rowBuf = appendPad(t.rowBuf, pad)
			}

		case cw > w:
			truncated := style.TruncateToWidth(cell, w, !t.wrap)
			t.rowBuf = append(t.rowBuf, truncated...)

			remaining := w - style.CellWidth(truncated)
			if remaining > 0 {
				t.rowBuf = appendPad(t.rowBuf, remaining)
			}

		default:
			t.rowBuf = append(t.rowBuf, cell...)
		}

		if reset != "" {
			t.rowBuf = append(t.rowBuf, reset...)

			if selBg != "" {
				t.rowBuf = append(t.rowBuf, selBg...)
			}
		}
	}

	if selBg != "" {
		t.rowBuf = append(t.rowBuf, style.ANSIReset()...)
	}

	if hasBorder && t.borderRight {
		t.rowBuf = append(t.rowBuf, bfg...)
		t.rowBuf = append(t.rowBuf, t.border.Vertical...)
		t.rowBuf = append(t.rowBuf, borderReset...)
	}
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
	need := numCols * 4
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

	for i := range numCols {
		fw := t.columnStyle(i).GetWidth()
		if fw > 0 {
			fixedWidths[i] = fw
		}
	}

	if t.width <= 0 {
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

	borderCharsWidth := t.totalBorderWidth(numCols)
	availableWidth := max(t.width-borderCharsWidth, 0)

	copy(distributed, contentWidths)

	fixedTotal := 0

	for i := range numCols {
		if fixedWidths[i] > 0 {
			distributed[i] = fixedWidths[i]
			fixedTotal += fixedWidths[i]
		}
	}

	nonFixedAvailable := max(availableWidth-fixedTotal, 0)

	nonFixedContent := 0

	for i := range numCols {
		if fixedWidths[i] == 0 {
			nonFixedContent += distributed[i]
		}
	}

	if nonFixedContent == nonFixedAvailable {
		result := make([]int, numCols)
		copy(result, distributed)

		return result
	}

	if nonFixedContent < nonFixedAvailable {
		for {
			total := 0

			for i := range numCols {
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
	} else {
		medians := buf[3*numCols : 4*numCols]
		t.shrinkNonFixedColumns(distributed, contentWidths, fixedWidths, medians, nonFixedAvailable)
	}

	result := make([]int, numCols)
	copy(result, distributed)

	return result
}

func (t *Table) totalBorderWidth(numCols int) int {
	if t.border.Vertical == "" {
		return 0
	}

	w := 0

	if t.borderLeft {
		w++
	}

	if t.borderRight {
		w++
	}

	if t.borderColumn {
		w += numCols - 1
	}

	return w
}

//nolint:cyclop
func (t *Table) shrinkNonFixedColumns(distributed, contentWidths, fixedWidths, medians []int, availableWidth int) {
	numCols := len(distributed)

	for {
		total := 0

		for i, w := range distributed {
			if fixedWidths[i] == 0 {
				total += w
			}
		}

		if total <= availableWidth {
			break
		}

		bigIdx := -1
		bigW := -1

		for i, w := range distributed {
			if fixedWidths[i] == 0 && w >= availableWidth/2 && w > bigW {
				bigW = w
				bigIdx = i
			}
		}

		if bigIdx < 0 || distributed[bigIdx] == 0 {
			break
		}

		distributed[bigIdx]--
	}

	for i := range numCols {
		if fixedWidths[i] == 0 {
			medians[i] = max(contentWidths[i]/2, 1)
		}
	}

	for {
		total := 0

		for i, w := range distributed {
			if fixedWidths[i] == 0 {
				total += w
			}
		}

		if total <= availableWidth {
			break
		}

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

		if bigDiffIdx < 0 || distributed[bigDiffIdx] == 0 {
			break
		}

		distributed[bigDiffIdx]--
	}

	for {
		total := 0

		for i, w := range distributed {
			if fixedWidths[i] == 0 {
				total += w
			}
		}

		if total <= availableWidth {
			break
		}

		bigIdx := -1
		bigW := -1

		for i, w := range distributed {
			if fixedWidths[i] == 0 && w > bigW {
				bigW = w
				bigIdx = i
			}
		}

		if bigIdx < 0 || distributed[bigIdx] == 0 {
			break
		}

		distributed[bigIdx]--
	}
}

func (t *Table) writeHorizontalBorder(left, mid, right string,
	colWidths []int, fg, reset string,
) {
	t.outBuf = append(t.outBuf, fg...)
	t.outBuf = append(t.outBuf, left...)

	for i, w := range colWidths {
		if i > 0 && t.borderColumn {
			t.outBuf = append(t.outBuf, mid...)
		}

		if w > 0 {
			t.outBuf = appendRepeatStr(t.outBuf, t.border.Horizontal, w)
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

func cellsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func hasNewline(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return true
		}
	}

	return false
}
