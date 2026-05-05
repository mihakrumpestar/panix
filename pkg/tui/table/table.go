package table

import (
	"fmt"
	"strings"

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
	borderSet    bool
	borderTop    bool
	borderRight  bool
	borderBottom bool
	borderLeft   bool
	borderColumn bool

	selectedIndex int
	selBg         style.Color
	selBgPrefix   string
	zonePrefix    string

	colWidths       []int
	colWidthsCached bool

	rowCache          []string
	rowCacheData      [][]string
	rowCacheSelIdx    int
	rowCacheWidth     int
	rowCacheColWidths []int
}

func New() *Table {
	return &Table{
		borderTop:     true,
		borderRight:   true,
		borderBottom:  true,
		borderLeft:    true,
		borderColumn:  true,
		selectedIndex: -1,
	}
}

func (t *Table) Width(w int) *Table {
	if t.width == w {
		return t
	}

	t.width = w
	t.colWidthsCached = false

	return t
}

func (t *Table) Border(b style.Border) *Table {
	t.border = b
	t.borderSet = true
	t.colWidthsCached = false

	return t
}

func (t *Table) Borders(top, right, bottom, left bool) *Table {
	t.borderTop = top
	t.borderRight = right
	t.borderBottom = bottom
	t.borderLeft = left
	t.colWidthsCached = false

	return t
}

func (t *Table) BorderColumn(v bool) *Table {
	t.borderColumn = v
	t.colWidthsCached = false

	return t
}

func (t *Table) BorderStyle(s style.Style) *Table {
	t.borderSty = s

	return t
}

func (t *Table) Headers(h ...string) *Table {
	t.headers = h
	t.colWidthsCached = false

	return t
}

func (t *Table) Row(cells ...string) *Table {
	t.rows = append(t.rows, cells)
	t.colWidthsCached = false

	return t
}

func (t *Table) Rows(rows ...[]string) *Table {
	t.rows = append(t.rows, rows...)
	t.colWidthsCached = false

	return t
}

func (t *Table) ColumnStyles(styles []style.Style) *Table {
	t.columnStyles = styles

	return t
}

func (t *Table) Wrap(v bool) *Table {
	t.wrap = v

	return t
}

func (t *Table) SelectionBackground(c style.Color) *Table {
	t.selBg = c
	t.selBgPrefix = style.ColorToBgPrefix(c)

	return t
}

// SetRows replaces all row data. Selection is preserved. The render
// cache is diffed — only rows whose data actually changed are
// re-rendered on the next String() call.
func (t *Table) SetRows(rows [][]string) *Table {
	t.rows = rows
	t.colWidthsCached = false

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

	t.selectedIndex = idx
}

func (t *Table) Deselect() {
	t.selectedIndex = -1
}

func (t *Table) ZonePrefix() string {
	return t.zonePrefix
}

func (t *Table) SetZonePrefix(prefix string) *Table {
	t.zonePrefix = prefix

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

			return true
		}

		return false
	}

	for idx := range len(t.rows) {
		zoneName := fmt.Sprintf("%s-%d", t.zonePrefix, idx)
		if zeroterm.IsZoneAtLine(lines[msg.Y], msg.X, zoneName) {
			if t.selectedIndex != idx {
				t.selectedIndex = idx

				return true
			}

			return false
		}
	}

	// Click was not inside any row zone — deselect
	if t.selectedIndex >= 0 {
		t.selectedIndex = -1

		return true
	}

	return false
}

// HandleNavigation processes left/right key navigation. Returns true if
// the navigation was consumed. Allows initial selection with left/right
// when nothing is selected.
func (t *Table) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(t.rows) == 0 || t.selectedIndex == -1 {
		return false
	}

	switch key {
	case "left":
		if t.selectedIndex > 0 {
			t.selectedIndex--

			return true
		}

		if t.selectedIndex < 0 && len(t.rows) > 0 {
			t.selectedIndex = 0

			return true
		}
	case "right":
		if t.selectedIndex < 0 && len(t.rows) > 0 {
			t.selectedIndex = 0

			return true
		}

		if t.selectedIndex < len(t.rows)-1 {
			t.selectedIndex++

			return true
		}
	}

	return false
}

//nolint:funlen,cyclop
func (t *Table) String() string {
	numCols := t.numCols()
	if numCols == 0 {
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

	hasBorder := t.borderSet && t.border.Vertical != ""

	bfg := t.borderSty.FgPrefix()
	if bfg == "" {
		bfg = t.borderSty.BgPrefix()
	}

	borderReset := ""
	if bfg != "" {
		borderReset = style.ANSIReset()
	}

	// Build or update per-row render cache.
	// A cached entry is valid when:
	//   1. The cache was rendered at the same width
	//   2. The row data matches what was cached (rowCacheData[i] == rows[i])
	//   3. The selection index matches (otherwise the row's sel bg is wrong)
	//
	// Strategy: iterate rows, find which need re-rendering, then
	// additionally re-render the previously-selected and newly-selected
	// rows if selection changed.
	widthMatch := t.rowCacheWidth == t.width &&
		len(t.rowCacheData) == len(t.rows) &&
		colWidthsMatch(colWidths, t.rowCacheColWidths)

	rowsToRerender := make(map[int]struct{})

	if !widthMatch {
		// Full rebuild — width changed or row count changed
		t.rowCache = make([]string, len(t.rows))
		t.rowCacheData = make([][]string, len(t.rows))

		for i := range t.rows {
			rowsToRerender[i] = struct{}{}
		}
	} else {
		// Diff per-row data
		for i, row := range t.rows {
			if !cellsEqual(t.rowCacheData[i], row) {
				rowsToRerender[i] = struct{}{}
			}
		}

		// Selection diff: if selection changed, the old and new selected
		// rows need re-rendering (their bg differs)
		if t.rowCacheSelIdx != t.selectedIndex {
			if t.rowCacheSelIdx >= 0 && t.rowCacheSelIdx < len(t.rows) {
				rowsToRerender[t.rowCacheSelIdx] = struct{}{}
			}

			if t.selectedIndex >= 0 && t.selectedIndex < len(t.rows) {
				rowsToRerender[t.selectedIndex] = struct{}{}
			}
		}
	}

	// Re-render dirty rows
	for i := range rowsToRerender {
		if i >= 0 && i < len(t.rows) {
			t.rowCache[i] = t.renderRow(t.rows[i], colWidths, i, hasBorder, bfg, borderReset)
			t.rowCacheData[i] = t.rows[i]
		}
	}

	t.rowCacheWidth = t.width
	t.rowCacheSelIdx = t.selectedIndex
	t.rowCacheColWidths = colWidths

	// Assemble output from cached row strings
	var b strings.Builder

	hasContent := len(t.headers) > 0 || len(t.rows) > 0

	if hasBorder && t.borderTop && hasContent {
		t.writeHorizontalBorder(&b, t.border.TopLeft, t.border.TopMid, t.border.TopRight,
			colWidths, bfg, borderReset)
	}

	if len(t.headers) > 0 {
		t.writeRow(&b, t.headers, colWidths, HeaderRow, hasBorder, bfg, borderReset)

		if hasBorder && t.borderColumn {
			t.writeHorizontalBorder(&b, t.border.LeftMid, t.border.MidMid, t.border.RightMid,
				colWidths, bfg, borderReset)
		}
	}

	for rowIdx, rowStr := range t.rowCache {
		if t.zonePrefix != "" {
			zoneName := fmt.Sprintf("%s-%d", t.zonePrefix, rowIdx)
			b.WriteString(zeroterm.Mark(zoneName, rowStr))
		} else {
			b.WriteString(rowStr)
		}
	}

	if hasBorder && t.borderBottom && hasContent {
		t.writeHorizontalBorder(&b, t.border.BottomLeft, t.border.BottomMid, t.border.BottomRight,
			colWidths, bfg, borderReset)
	}

	result := b.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result
}

//nolint:cyclop
func (t *Table) renderRow(cells []string, colWidths []int,
	rowIdx int, hasBorder bool, bfg, borderReset string,
) string {
	// Build the row in two parts:
	//   1. outer borders (left │, right │) — no selection bg
	//   2. inner content (cells + inner column borders │) — selection bg
	//
	// For selected rows, the inner content block is wrapped with the
	// selection background. Since Style.Render emits \x1b[m resets that
	// clear the bg, we re-emit selBgPrefix after every reset inside
	// the block so the bg spans uninterrupted.
	selBgPrefix := ""
	if rowIdx >= 0 && rowIdx == t.selectedIndex {
		selBgPrefix = t.selBgPrefix
	}

	// Build inner content (between outer borders)
	var inner strings.Builder

	for i, w := range colWidths {
		if i > 0 && hasBorder && t.borderColumn {
			inner.WriteString(bfg)
			inner.WriteString(t.border.Vertical)
			inner.WriteString(borderReset)
		}

		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}

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

		inner.WriteString(sty.Render(cell))
	}

	// Assemble row: left border + inner content + right border
	var b strings.Builder

	if hasBorder && t.borderLeft {
		b.WriteString(bfg)
		b.WriteString(t.border.Vertical)
		b.WriteString(borderReset)
	}

	innerStr := inner.String()
	if selBgPrefix != "" {
		// Re-emit selBg after every ANSI reset inside the content block
		innerStr = strings.ReplaceAll(innerStr, style.ANSIReset(), style.ANSIReset()+selBgPrefix)
		b.WriteString(selBgPrefix)
		b.WriteString(innerStr)
		b.WriteString(style.ANSIReset())
	} else {
		b.WriteString(innerStr)
	}

	if hasBorder && t.borderRight {
		b.WriteString(bfg)
		b.WriteString(t.border.Vertical)
		b.WriteString(borderReset)
	}

	b.WriteByte('\n')

	return b.String()
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

func (t *Table) contentWidths(numCols int) []int {
	widths := make([]int, numCols)

	for i, h := range t.headers {
		if i < numCols {
			w := style.CellWidth(h)
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	for _, row := range t.rows {
		for i, cell := range row {
			if i < numCols {
				w := style.CellWidth(cell)
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	return widths
}

// distributeWidths calculates column widths that exactly fill the table
// width. When content is narrower than the table, columns expand. When
// content is wider, columns shrink — prioritizing shrinking the widest
// columns first (matching lipgloss behavior).
func (t *Table) distributeWidths(numCols int) []int {
	if numCols == 0 {
		return nil
	}

	contentWidths := t.contentWidths(numCols)

	// Collect fixed widths from columnStyles. Columns with Width(N) set
	// in their style are locked — they won't be expanded or shrunk.
	fixedWidths := make([]int, numCols)

	for i := range numCols {
		sty := t.columnStyle(i)

		fw := sty.GetWidth()
		if fw > 0 {
			fixedWidths[i] = fw
		}
	}

	// No width set — use content widths, but respect fixed widths
	if t.width <= 0 {
		for i := range numCols {
			if fixedWidths[i] > 0 {
				contentWidths[i] = fixedWidths[i]
			}
		}

		return contentWidths
	}

	borderCharsWidth := t.totalBorderWidth(numCols)

	availableWidth := max(t.width-borderCharsWidth, 0)

	distributed := make([]int, numCols)
	copy(distributed, contentWidths)

	// Lock fixed columns — they contribute their fixed width and can't change
	fixedTotal := 0

	for i := range numCols {
		if fixedWidths[i] > 0 {
			distributed[i] = fixedWidths[i]
			fixedTotal += fixedWidths[i]
		}
	}

	// Calculate how much space the non-fixed columns need/have
	nonFixedAvailable := max(availableWidth-fixedTotal, 0)

	nonFixedContent := 0

	for i := range numCols {
		if fixedWidths[i] == 0 {
			nonFixedContent += distributed[i]
		}
	}

	if nonFixedContent == nonFixedAvailable {
		return distributed
	}

	if nonFixedContent < nonFixedAvailable {
		// Expand non-fixed columns to fill remaining width
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

			// Find shortest non-fixed column
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
		// Shrink non-fixed columns to fit remaining width
		t.shrinkNonFixedColumns(distributed, contentWidths, fixedWidths, nonFixedAvailable)
	}

	return distributed
}

func (t *Table) totalBorderWidth(numCols int) int {
	if !t.borderSet || t.border.Vertical == "" {
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
func (t *Table) shrinkNonFixedColumns(distributed, contentWidths, fixedWidths []int, availableWidth int) {
	numCols := len(distributed)

	// Phase 1: Shrink columns that are > half the available width (skip fixed)
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

	// Phase 2: Shrink columns with biggest difference from median (skip fixed)
	medians := make([]int, numCols)

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

	// Phase 3: Shrink the biggest non-fixed columns overall
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

func (t *Table) writeHorizontalBorder(b *strings.Builder, left, mid, right string,
	colWidths []int, fg, reset string,
) {
	b.WriteString(fg)
	b.WriteString(left)

	for i, w := range colWidths {
		if i > 0 && t.borderColumn {
			b.WriteString(mid)
		}

		if w > 0 {
			b.WriteString(strings.Repeat(t.border.Horizontal, w))
		}
	}

	b.WriteString(right)
	b.WriteString(reset)
	b.WriteByte('\n')
}

func (t *Table) writeRow(b *strings.Builder, cells []string, colWidths []int,
	rowIdx int, hasBorder bool, bfg, borderReset string,
) {
	b.WriteString(t.renderRow(cells, colWidths, rowIdx, hasBorder, bfg, borderReset))
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

func colWidthsMatch(a, b []int) bool {
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
