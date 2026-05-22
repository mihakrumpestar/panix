package table

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

func strLines(s ...string) [][]byte {
	out := make([][]byte, len(s))
	for i, v := range s {
		out[i] = []byte(v)
	}

	return out
}

func strRows(rows ...[]string) [][][]byte {
	out := make([][][]byte, len(rows))
	for i, row := range rows {
		out[i] = make([][]byte, len(row))
		for j, cell := range row {
			out[i][j] = []byte(cell)
		}
	}

	return out
}

func stripANSI(s string) string {
	return string(style.StripANSI([]byte(s)))
}

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})

	assert.Equal(t, 0, tbl.cfg.Width, "Default width should be 0")
	assert.False(t, tbl.bordered, "Default bordered should be false")
	assert.Equal(t, -1, tbl.SelectedIndex(), "Default SelectedIndex should be -1")
}

func TestTable_Empty(t *testing.T) {
	t.Parallel()

	got := buffer.LinesBufToStringForTests(New(Config{}).Render())
	assert.Empty(t, got, "Empty table should return empty string")
}

func TestTable_HeadersOnly_NoBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Headers: strLines("Name", "Value")})
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, got, "Name", "Headers-only table should contain header text")
	assert.Contains(t, got, "Value", "Headers-only table should contain header text")
}

func TestTable_HeadersAndRows_WithBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 20, Border: style.NormalBorder(), Headers: strLines("Name", "Value")})
	tbl.SetRows(strRows([]string{"key1", "val1"}, []string{"key2", "val2"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, got, "┌", "Bordered table missing top-left corner")
	assert.Contains(t, got, "└", "Bordered table missing bottom-left corner")
	assert.Contains(t, got, "┬", "Bordered table missing header separator")
}

func TestTable_RowsOnly_WithBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 20, Border: style.NormalBorder()})
	tbl.SetRows(strRows([]string{"a", "b"}, []string{"c", "d"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, got, "┌", "Bordered table missing top border")
	assert.Contains(t, got, "└", "Bordered table missing bottom border")
}

func TestTable_NoBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Headers: strLines("A", "B")})
	tbl.SetRows(strRows([]string{"1", "2"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.NotContains(t, got, "┌", "No-border table should not contain border chars")
	assert.NotContains(t, got, "│", "No-border table should not contain border chars")
}

func TestTable_WidthExpandsToFill(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 30, Border: style.NormalBorder(), Headers: strLines("A", "B")})
	tbl.SetRows(strRows([]string{"1", "2"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth([]byte(line))
		assert.Equal(t, 30, lineWidth)
	}
}

func TestTable_WidthShrinksToShrink(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 12, Border: style.NormalBorder(), Headers: strLines("LongHeader1", "LongHeader2")})
	tbl.SetRows(strRows([]string{"longcontent1", "longcontent2"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth([]byte(line))
		assert.LessOrEqual(t, lineWidth, 14, "Line too wide: width=%d", lineWidth)
	}
}

func TestTable_WidthNoBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 20, Headers: strLines("A", "B")})
	tbl.SetRows(strRows([]string{"1", "2"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth([]byte(line))
		assert.Equal(t, 20, lineWidth, "Line width mismatch")
	}
}

func TestTable_ColumnStyles(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:   20,
		Border:  style.NormalBorder(),
		Headers: strLines("A", "B"),
		ColumnStyles: []style.Style{
			style.NewStyle().Foreground(style.Color("#8BE9FD")),
			style.NewStyle().Foreground(style.Color("#FF5555")),
		},
	})
	tbl.SetRows(strRows([]string{"x", "y"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, got, "\x1b[38;2;139;233;253m", "Column 0 missing fg color")
	assert.Contains(t, got, "\x1b[38;2;255;85;85m", "Column 1 missing fg color")
}

func TestTable_Rows(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Headers: strLines("A")})
	tbl.SetRows(strRows([]string{"1"}, []string{"2"}, []string{"3"}))

	assert.Len(t, tbl.rows, 3, "Rows count mismatch")
}

func TestTable_Wrap(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Wrap: true})
	assert.True(t, tbl.cfg.Wrap, "Wrap(true) should set wrap = true")
}

func TestTable_WrapFalse_Truncates(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 10, Border: style.NormalBorder(), Headers: strLines("A")})
	tbl.SetRows(strRows([]string{"abcdefghijXYZ"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		content := strings.Trim(line, "│ ")
		assert.NotEqual(t, "abcdefghijXYZ", content, "Content should have been truncated")
	}
}

func TestTable_BorderStyle(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:       20,
		Border:      style.NormalBorder(),
		BorderStyle: style.NewStyle().Foreground(style.Color("#FF0000")),
		Headers:     strLines("A"),
	})
	tbl.SetRows(strRows([]string{"x"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, got, "\x1b[38;2;255;0;0m", "BorderStyle color not applied to border")
}

func TestTable_CalculateColumnWidths(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Headers: strLines("Name", "Value")})
	tbl.SetRows(strRows([]string{"longkey", "v"}))

	widths := make([]int, 2)
	tbl.contentWidths(2, widths)

	assert.GreaterOrEqual(t, widths[0], 7, "Col 0 width too small")
	assert.GreaterOrEqual(t, widths[1], 5, "Col 1 width too small")
}

func TestTable_HiddenBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Border: style.HiddenBorder(), Headers: strLines("A", "B")})
	tbl.SetRows(strRows([]string{"1", "2"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.NotContains(t, got, "│", "HiddenBorder should not render vertical bars")
}

func TestTable_HeaderRowConstant(t *testing.T) {
	t.Parallel()

	assert.Equal(t, -1, HeaderRow, "HeaderRow constant mismatch")
}

func TestTable_SelectionBackground(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New(Config{
		Width:               20,
		Border:              style.NormalBorder(),
		SelectionBackground: selBg,
		Headers:             strLines("A", "B"),
	})
	tbl.SetRows(strRows([]string{"x", "y"}, []string{"z", "w"}))
	tbl.Select(0)
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, got, "\x1b[48;2;51;51;51m", "Selected row missing background color")
}

func TestTable_HandleNavigation(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}, []string{"c"}))
	tbl.Select(1)

	assert.True(t, tbl.HandleNavigation("right", false), "right should succeed")
	assert.Equal(t, 2, tbl.SelectedIndex(), "After right: SelectedIndex mismatch")
	assert.True(t, tbl.HandleNavigation("left", false), "left should succeed")
	assert.Equal(t, 1, tbl.SelectedIndex(), "After left: SelectedIndex mismatch")
}

func TestTable_HandleNavigation_InitialSelection(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}, []string{"c"}))

	assert.True(t, tbl.HandleNavigation("right", false), "right with no selection should select first row")
	assert.Equal(t, 0, tbl.SelectedIndex(), "After initial right: SelectedIndex mismatch")
}

func TestTable_HandleNavigation_Boundary(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}))
	tbl.Select(0)

	assert.False(t, tbl.HandleNavigation("left", false), "left at index 0 should fail")

	tbl.Select(1)

	assert.False(t, tbl.HandleNavigation("right", false), "right at last index should fail")
}

func TestTable_HandleNavigation_NoRows(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})

	assert.False(t, tbl.HandleNavigation("right", false), "Navigation on empty table should fail")
}

func TestTable_HandleNavigation_ActiveInnerViewport(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}))
	tbl.Select(0)

	assert.False(t, tbl.HandleNavigation("right", true), "Navigation with active inner viewport should fail")
}

func TestTable_ZoneMarkersInOutput(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 10}).SetZonePrefix("test-tbl")
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, got, "z", "Zone markers should appear in output")
}

func TestTable_SelectionBackgroundNoOuterBorderBg(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New(Config{
		Width:               20,
		Border:              style.NormalBorder(),
		SelectionBackground: selBg,
		Headers:             strLines("A", "B"),
	})
	tbl.SetRows(strRows([]string{"x", "y"}))
	tbl.Select(0)
	got := buffer.LinesBufToStringForTests(tbl.Render())

	bgPrefix := "\x1b[48;2;51;51;51m"

	lines := strings.SplitSeq(got, "\n")
	for line := range lines {
		visible := stripANSI(line)
		if !strings.Contains(visible, "x") || !strings.Contains(visible, "y") {
			continue
		}

		before, _, ok := strings.Cut(line, "│")
		if ok && strings.Contains(before, bgPrefix) {
			t.Errorf("Selection bg must not appear before left border:\n%s", line)
		}

		rightBorderIdx := strings.LastIndex(line, "│")
		if rightBorderIdx >= 0 && strings.Contains(line[rightBorderIdx:], bgPrefix) {
			t.Errorf("Selection bg must not appear after right border:\n%s", line)
		}

		break
	}
}

func TestTable_SelectionBackgroundWithFgColor(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New(Config{
		Width:               20,
		Border:              style.NormalBorder(),
		SelectionBackground: selBg,
		Headers:             strLines("A", "B"),
		ColumnStyles: []style.Style{
			style.NewStyle().Foreground(style.Color("#8BE9FD")),
			style.NewStyle(),
		},
	})
	tbl.SetRows(strRows([]string{"x", "y"}))
	tbl.Select(0)
	got := buffer.LinesBufToStringForTests(tbl.Render())

	lines := strings.Split(got, "\n")
	selLine := ""

	for _, line := range lines {
		if strings.Contains(stripANSI(line), "x") && strings.Contains(stripANSI(line), "y") {
			selLine = line

			break
		}
	}

	require.NotEmpty(t, selLine, "Could not find selected data row")

	bgPrefix := "\x1b[48;2;51;51;51m"

	count := strings.Count(selLine, bgPrefix)
	assert.GreaterOrEqual(t, count, 2, "Selection bg should be re-emitted after cell resets")
}

func TestTable_SelectionBackgroundCoversPadding(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New(Config{
		Width:               40,
		Border:              style.NormalBorder(),
		SelectionBackground: selBg,
		Headers:             strLines("A", "B"),
		ColumnStyles: []style.Style{
			style.NewStyle().Width(10),
			style.NewStyle().Width(15),
		},
	})
	tbl.SetRows(strRows([]string{"x", "yyy"}))
	tbl.Select(0)
	got := buffer.LinesBufToStringForTests(tbl.Render())

	lines := strings.Split(got, "\n")

	selLine := ""

	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "x") && strings.Contains(plain, "yyy") {
			selLine = line

			break
		}
	}

	require.NotEmpty(t, selLine, "Could not find selected data row")

	bgPrefix := "\x1b[48;2;51;51;51m"

	cellXPortion := selLine
	if idx := strings.LastIndex(cellXPortion, "x"); idx >= 0 {
		cellXPortion = cellXPortion[:idx+1]
	}

	nextAfterX := selLine[len(cellXPortion):]
	if !strings.Contains(nextAfterX, bgPrefix) {
		t.Errorf("Padding after 'x' cell missing selection bg.\nFull line: %s\nAfter x: %q", selLine, nextAfterX)
	}
}

func TestTable_SelectionBackgroundCoversInnerSeparators(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New(Config{
		Width:               30,
		Border:              style.NormalBorder(),
		SelectionBackground: selBg,
		Headers:             strLines("A", "B", "C"),
	})
	tbl.SetRows(strRows([]string{"x", "y", "z"}))
	tbl.Select(0)
	got := buffer.LinesBufToStringForTests(tbl.Render())

	bgPrefix := "\x1b[48;2;51;51;51m"

	lines := strings.SplitSeq(got, "\n")

	for line := range lines {
		plain := stripANSI(line)
		if !strings.Contains(plain, "x") || !strings.Contains(plain, "y") || !strings.Contains(plain, "z") {
			continue
		}

		parts := strings.Split(line, "│")
		require.GreaterOrEqual(t, len(parts), 3, "Expected at least 3 parts split by │")

		leftBorder := parts[0]
		if strings.Contains(leftBorder, bgPrefix) {
			t.Errorf("Left outer border should NOT have sel bg:\n%s", line)
		}

		rightBorder := parts[len(parts)-1]
		if strings.Contains(rightBorder, bgPrefix) {
			t.Errorf("Right outer border should NOT have sel bg:\n%s", line)
		}

		innerPart := strings.Join(parts[1:len(parts)-1], "│")

		innerSeps := strings.Count(innerPart, "│")
		if innerSeps > 0 {
			innerBgCount := strings.Count(innerPart, bgPrefix)
			assert.GreaterOrEqual(t, innerBgCount, innerSeps, "Inner separators should have sel bg")
		}

		break
	}
}

func TestTable_HandleMouseClick_DeselectOutsideReturnsTrue(t *testing.T) {
	t.Parallel()

	tbl := New(Config{}).SetZonePrefix("test")
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}))
	tbl.Select(0)

	buf := buffer.NewLinesBufDiff()
	buf.WriteLine([]byte("no zones here"))
	buf.WriteLine([]byte("another line"))

	changed := tbl.HandleMouseClick(zeroterm.MouseClickMsg{X: 0, Y: 0, Lines: buf})

	assert.True(t, changed, "HandleMouseClick should return true when deselecting via outside click")
	assert.Equal(t, -1, tbl.SelectedIndex(), "SelectedIndex after outside click")
}

func TestTable_HandleNavigation_UpDownIgnored(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}, []string{"c"}))
	tbl.Select(1)

	assert.False(t, tbl.HandleNavigation("up", false), "up key should not be handled by table navigation")
	assert.False(t, tbl.HandleNavigation("down", false), "down key should not be handled by table navigation")
	assert.Equal(t, 1, tbl.SelectedIndex(), "SelectedIndex should remain 1")
}

func TestTable_FixedWidthColumns(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:   20,
		Border:  style.NormalBorder(),
		Headers: strLines("A", "B", "C"),
		ColumnStyles: []style.Style{
			style.NewStyle().Width(5),
			style.NewStyle(),
			style.NewStyle().Width(10),
		},
	})
	tbl.SetRows(strRows([]string{"a", "bb", "c"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth([]byte(line))
		assert.Equal(t, 20, lineWidth, "Line width mismatch")
	}
}

func TestTable_FixedWidthColumnsNotShrunk(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:   10,
		Border:  style.NormalBorder(),
		Headers: strLines("A", "B", "C"),
		ColumnStyles: []style.Style{
			style.NewStyle().Width(5),
			style.NewStyle().Width(5),
			style.NewStyle().Width(5),
		},
	})
	tbl.SetRows(strRows([]string{"a", "b", "c"}))

	got := buffer.LinesBufToStringForTests(tbl.Render())
	assert.NotEmpty(t, got, "Table should render even with overflow")
}

func TestTable_FixedWidthRespected(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:   20,
		Border:  style.NormalBorder(),
		Headers: strLines("Idx", "Name"),
		ColumnStyles: []style.Style{
			style.NewStyle().Width(4).Align(style.Right),
			style.NewStyle(),
		},
	})
	tbl.SetRows(strRows([]string{"1", "test"}))
	got := buffer.LinesBufToStringForTests(tbl.Render())

	visible := stripANSI(got)
	lines := strings.SplitSeq(visible, "\n")

	for line := range lines {
		if strings.Contains(line, "1") && strings.Contains(line, "test") {
			assert.Contains(t, line, "   1", "Fixed-width column should be right-aligned")
		}
	}
}

func TestTable_SetRows_PreservesSelection(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}, []string{"c"}))
	tbl.Select(1)

	require.Equal(t, 1, tbl.SelectedIndex(), "Select(1) → SelectedIndex mismatch")

	tbl.SetRows(strRows([]string{"x"}, []string{"y"}))

	assert.Equal(t, 1, tbl.SelectedIndex(), "SetRows should preserve selection")
	assert.Len(t, tbl.rows, 2, "After SetRows: len mismatch")
}

func TestTable_SetRows_PerRowCacheDiff(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 20, Border: style.NormalBorder(), Headers: strLines("A", "B")})
	tbl.SetRows(strRows([]string{"a", "b"}, []string{"c", "d"}, []string{"e", "f"}))

	result1 := buffer.LinesBufToStringForTests(tbl.Render())

	tbl.SetRows(strRows([]string{"a", "b"}, []string{"c", "d"}, []string{"e", "f"}))
	result2 := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Equal(t, result1, result2, "Same data should produce same output")

	tbl.SetRows(strRows([]string{"a", "b"}, []string{"CHANGED", "d"}, []string{"e", "f"}))
	result3 := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, result3, "CHANGED", "Changed row should appear in output")
}

func TestTable_SetRows_RowCountChange_FullRebuild(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 20, Border: style.NormalBorder(), Headers: strLines("A")})
	tbl.SetRows(strRows([]string{"a"}, []string{"b"}))
	_ = buffer.LinesBufToStringForTests(tbl.Render())

	tbl.SetRows(strRows([]string{"a"}, []string{"b"}, []string{"c"}))
	result := buffer.LinesBufToStringForTests(tbl.Render())

	assert.Contains(t, result, "c", "Full rebuild should include new row data")
}

func TestTable_SetRows_ColWidthChange_InvalidatesAllRows(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:        20,
		Border:       style.NormalBorder(),
		Headers:      strLines("A", "B"),
		ColumnStyles: []style.Style{{}, {}},
	})

	tbl.SetRows(strRows([]string{"WWWWWWWWWW", "x"}, []string{"y", "z"}))
	_ = buffer.LinesBufToStringForTests(tbl.Render())

	tbl.SetRows(strRows([]string{"a", "x"}, []string{"y", "z"}))
	result := buffer.LinesBufToStringForTests(tbl.Render())

	visible := stripANSI(result)
	lines := strings.SplitSeq(visible, "\n")

	for line := range lines {
		if line == "" {
			continue
		}

		lineWidth := style.CellWidth([]byte(line))
		assert.Equal(t, 20, lineWidth, "Line width mismatch")
	}
}
