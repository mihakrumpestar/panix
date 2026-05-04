package table

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	tbl := New()

	if tbl.width != 0 {
		t.Errorf("Default width = %d, want 0", tbl.width)
	}

	if tbl.borderSet {
		t.Error("Default borderSet should be false")
	}

	if !tbl.borderTop || !tbl.borderRight || !tbl.borderBottom || !tbl.borderLeft || !tbl.borderColumn {
		t.Error("Default border sides should all be true")
	}

	if tbl.SelectedIndex() != -1 {
		t.Errorf("Default SelectedIndex = %d, want -1", tbl.SelectedIndex())
	}
}

func TestTable_Empty(t *testing.T) {
	t.Parallel()

	got := New().String()
	if got != "" {
		t.Errorf("Empty table String() = %q, want \"\"", got)
	}
}

func TestTable_HeadersOnly_NoBorder(t *testing.T) {
	t.Parallel()

	tbl := New().Headers("Name", "Value")
	got := tbl.String()

	if !strings.Contains(got, "Name") || !strings.Contains(got, "Value") {
		t.Errorf("Headers-only no border = %q, missing header text", got)
	}
}

func TestTable_HeadersAndRows_WithBorder(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Headers("Name", "Value").
		Row("key1", "val1").
		Row("key2", "val2")
	got := tbl.String()

	if !strings.Contains(got, "┌") {
		t.Errorf("Missing top-left corner ┌: %s", got)
	}

	if !strings.Contains(got, "└") {
		t.Errorf("Missing bottom-left corner └: %s", got)
	}

	if !strings.Contains(got, "┬") {
		t.Errorf("Missing header separator ┬: %s", got)
	}
}

func TestTable_RowsOnly_WithBorder(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Row("a", "b").
		Row("c", "d")
	got := tbl.String()

	if !strings.Contains(got, "┌") {
		t.Errorf("Missing top border ┌: %s", got)
	}

	if !strings.Contains(got, "└") {
		t.Errorf("Missing bottom border └: %s", got)
	}
}

func TestTable_NoBorder(t *testing.T) {
	t.Parallel()

	tbl := New().Headers("A", "B").Row("1", "2")
	got := tbl.String()

	if strings.Contains(got, "┌") || strings.Contains(got, "│") {
		t.Errorf("No-border table should not contain border chars:\n%s", got)
	}
}

func TestTable_Borders_Selective(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Borders(false, true, true, true).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	if strings.Contains(got, "┌") || strings.Contains(got, "┬") {
		t.Errorf("Top border should not appear when Borders(false,...):\n%s", got)
	}

	if !strings.Contains(got, "│") {
		t.Errorf("Vertical border should still appear:\n%s", got)
	}
}

func TestTable_Borders_AllOff(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Borders(false, false, false, false).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	if strings.Contains(got, "┌") || strings.Contains(got, "└") {
		t.Errorf("Outer border corners should not appear:\n%s", got)
	}
}

func TestTable_WidthExpandsToFill(t *testing.T) {
	t.Parallel()

	// Table width=30, content "A"=1 + "B"=1 = 2 content chars.
	// Border chars: │(left) + │(mid) + │(right) = 3.
	// Available = 30 - 3 = 27. Content = 2. Expand by 25.
	tbl := New().Width(30).Border(style.NormalBorder()).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth(line)
		if lineWidth != 30 {
			t.Errorf("Line width = %d, want 30. Line: %q", lineWidth, line)
		}
	}
}

func TestTable_WidthShrinksToShrink(t *testing.T) {
	t.Parallel()

	// Content is wider than table width — columns must shrink
	tbl := New().Width(12).Border(style.NormalBorder()).
		Headers("LongHeader1", "LongHeader2").
		Row("longcontent1", "longcontent2")
	got := tbl.String()

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth(line)
		if lineWidth > 14 {
			t.Errorf("Line too wide: width=%d, line: %q", lineWidth, line)
		}
	}
}

func TestTable_WidthNoBorder(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth(line)
		if lineWidth != 20 {
			t.Errorf("Line width = %d, want 20. Line: %q", lineWidth, line)
		}
	}
}

func TestTable_ColumnStyles(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Headers("A", "B").
		Row("x", "y").
		ColumnStyles([]style.Style{
			style.NewStyle().Foreground(style.Color("#8BE9FD")),
			style.NewStyle().Foreground(style.Color("#FF5555")),
		})
	got := tbl.String()

	if !strings.Contains(got, "\x1b[38;2;139;233;253m") {
		t.Errorf("Column 0 missing fg color: %s", got)
	}

	if !strings.Contains(got, "\x1b[38;2;255;85;85m") {
		t.Errorf("Column 1 missing fg color: %s", got)
	}
}

func TestTable_Rows(t *testing.T) {
	t.Parallel()

	tbl := New().Headers("A").
		Rows([]string{"1"}, []string{"2"}, []string{"3"})

	if len(tbl.rows) != 3 {
		t.Errorf("Rows count = %d, want 3", len(tbl.rows))
	}
}

func TestTable_Wrap(t *testing.T) {
	t.Parallel()

	tbl := New().Wrap(true)
	if !tbl.wrap {
		t.Error("Wrap(true) should set wrap = true")
	}
}

func TestTable_WrapFalse_Truncates(t *testing.T) {
	t.Parallel()

	tbl := New().Width(10).Border(style.NormalBorder()).
		Headers("A").
		Row("abcdefghijXYZ")
	got := tbl.String()

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		content := strings.Trim(line, "│ ")
		if content == "abcdefghijXYZ" {
			t.Errorf("Content should have been truncated, got: %q", content)
		}
	}
}

func TestTable_BorderStyle(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		BorderStyle(style.NewStyle().Foreground(style.Color("#FF0000"))).
		Headers("A").
		Row("x")
	got := tbl.String()

	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Errorf("BorderStyle color not applied to border:\n%s", got)
	}
}

func TestTable_CalculateColumnWidths(t *testing.T) {
	t.Parallel()

	tbl := New().Headers("Name", "Value").
		Row("longkey", "v")
	widths := tbl.contentWidths(2)

	if widths[0] < 7 {
		t.Errorf("Col 0 width = %d, want >= 7", widths[0])
	}

	if widths[1] < 5 {
		t.Errorf("Col 1 width = %d, want >= 5", widths[1])
	}
}

func TestTable_HiddenBorder(t *testing.T) {
	t.Parallel()

	tbl := New().Border(style.HiddenBorder()).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	if strings.Contains(got, "│") {
		t.Errorf("HiddenBorder should not render vertical bars:\n%s", got)
	}
}

func TestTable_HeaderRowConstant(t *testing.T) {
	t.Parallel()

	if HeaderRow != -1 {
		t.Errorf("HeaderRow = %d, want -1", HeaderRow)
	}
}

func TestTable_SelectionBackground(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New().Width(20).Border(style.NormalBorder()).
		SelectionBackground(selBg).
		Headers("A", "B").
		Row("x", "y").
		Row("z", "w")
	tbl.Select(0)
	got := tbl.String()

	if !strings.Contains(got, "\x1b[48;2;51;51;51m") {
		t.Errorf("Selected row missing background color:\n%s", got)
	}
}

func TestTable_HandleNavigation(t *testing.T) {
	t.Parallel()

	tbl := New().Row("a").Row("b").Row("c")
	tbl.Select(1)

	if !tbl.HandleNavigation("right", false) {
		t.Error("right should succeed")
	}

	if tbl.SelectedIndex() != 2 {
		t.Errorf("After right: SelectedIndex = %d, want 2", tbl.SelectedIndex())
	}

	if !tbl.HandleNavigation("left", false) {
		t.Error("left should succeed")
	}

	if tbl.SelectedIndex() != 1 {
		t.Errorf("After left: SelectedIndex = %d, want 1", tbl.SelectedIndex())
	}
}

func TestTable_HandleNavigation_InitialSelection(t *testing.T) {
	t.Parallel()

	// Should allow initial selection when nothing is selected
	tbl := New().Row("a").Row("b").Row("c")

	if !tbl.HandleNavigation("right", false) {
		t.Error("right with no selection should select first row")
	}

	if tbl.SelectedIndex() != 0 {
		t.Errorf("After initial right: SelectedIndex = %d, want 0", tbl.SelectedIndex())
	}
}

func TestTable_HandleNavigation_Boundary(t *testing.T) {
	t.Parallel()

	tbl := New().Row("a").Row("b")
	tbl.Select(0)

	if tbl.HandleNavigation("left", false) {
		t.Error("left at index 0 should fail")
	}

	tbl.Select(1)

	if tbl.HandleNavigation("right", false) {
		t.Error("right at last index should fail")
	}
}

func TestTable_HandleNavigation_NoRows(t *testing.T) {
	t.Parallel()

	tbl := New()

	if tbl.HandleNavigation("right", false) {
		t.Error("Navigation on empty table should fail")
	}
}

func TestTable_HandleNavigation_ActiveInnerViewport(t *testing.T) {
	t.Parallel()

	tbl := New().Row("a").Row("b")
	tbl.Select(0)

	if tbl.HandleNavigation("right", true) {
		t.Error("Navigation with active inner viewport should fail")
	}
}

func TestTable_ZonePrefix(t *testing.T) {
	t.Parallel()

	tbl := New().SetZonePrefix("test-table").Row("a").Row("b")

	if tbl.ZonePrefix() != "test-table" {
		t.Errorf("ZonePrefix = %q, want \"test-table\"", tbl.ZonePrefix())
	}
}

func TestTable_ZoneMarkersInOutput(t *testing.T) {
	t.Parallel()

	tbl := New().SetZonePrefix("test-tbl").Width(10).Row("a").Row("b")
	got := tbl.String()

	if !strings.Contains(got, "z") {
		t.Errorf("Zone markers should appear in output:\n%s", got)
	}
}

func TestTable_SelectionBackgroundNoOuterBorderBg(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New().Width(20).Border(style.NormalBorder()).
		SelectionBackground(selBg).
		Headers("A", "B").
		Row("x", "y")
	tbl.Select(0)
	got := tbl.String()

	bgPrefix := "\x1b[48;2;51;51;51m"

	lines := strings.SplitSeq(got, "\n")
	for line := range lines {
		visible := stripANSI(line)
		if !strings.Contains(visible, "x") || !strings.Contains(visible, "y") {
			continue
		}

		// The bg prefix must NOT appear before the left border │
		before, _, ok := strings.Cut(line, "│")
		if ok && strings.Contains(before, bgPrefix) {
			t.Errorf("Selection bg must not appear before left border:\n%s", line)
		}

		// The bg prefix must NOT appear after the right border │
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
	tbl := New().Width(20).Border(style.NormalBorder()).
		SelectionBackground(selBg).
		Headers("A", "B").
		Row("x", "y").
		ColumnStyles([]style.Style{
			style.NewStyle().Foreground(style.Color("#8BE9FD")),
			style.NewStyle(),
		})
	tbl.Select(0)
	got := tbl.String()

	// The selected row should have the bg color, and it should be
	// re-emitted after each cell's reset so the bg continues.
	lines := strings.Split(got, "\n")
	selLine := ""

	for _, line := range lines {
		if strings.Contains(stripANSI(line), "x") && strings.Contains(stripANSI(line), "y") {
			selLine = line

			break
		}
	}

	if selLine == "" {
		t.Fatal("Could not find selected data row")
	}

	// Count occurrences of the bg prefix — should appear at least twice
	// (after left border + after first cell reset + possibly after inner border)
	bgPrefix := "\x1b[48;2;51;51;51m"

	count := strings.Count(selLine, bgPrefix)
	if count < 2 {
		t.Errorf("Selection bg should be re-emitted after cell resets, found %d occurrences in:\n%s", count, selLine)
	}
}

func TestTable_HandleMouseClick_DeselectOutsideReturnsTrue(t *testing.T) {
	t.Parallel()

	tbl := New().SetZonePrefix("test").Row("a").Row("b")
	tbl.Select(0)

	// Simulate click on Y line that has no zone markers — should deselect and return true
	render.SetCurrentLines([]string{"no zones here", "another line"})

	changed := tbl.HandleMouseClick(render.MouseClickMsg{X: 0, Y: 0})

	if !changed {
		t.Error("HandleMouseClick should return true when deselecting via outside click")
	}

	if tbl.SelectedIndex() != -1 {
		t.Errorf("SelectedIndex = %d, want -1 after outside click", tbl.SelectedIndex())
	}
}

func TestTable_HandleNavigation_UpDownIgnored(t *testing.T) {
	t.Parallel()

	tbl := New().Row("a").Row("b").Row("c")
	tbl.Select(1)

	if tbl.HandleNavigation("up", false) {
		t.Error("up key should not be handled by table navigation")
	}

	if tbl.HandleNavigation("down", false) {
		t.Error("down key should not be handled by table navigation")
	}

	if tbl.SelectedIndex() != 1 {
		t.Errorf("SelectedIndex should remain 1, got %d", tbl.SelectedIndex())
	}
}

func TestTable_BorderColumn(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		BorderColumn(false).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	// With borderColumn=false, inner column separators should not appear
	if strings.Contains(got, "┬") || strings.Contains(got, "┼") {
		t.Errorf("Column separators should not appear with BorderColumn(false):\n%s", got)
	}
}

func TestTable_NoTopBorderWhenBorderTopFalse(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Borders(false, true, true, true).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	if strings.Contains(got, "┌") || strings.Contains(got, "┬") {
		t.Errorf("No top border should appear: found in output")
	}

	if !strings.Contains(got, "└") {
		t.Errorf("Bottom border should still appear:\n%s", got)
	}
}

func TestTable_NoBottomBorderWhenBorderBottomFalse(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Borders(true, true, false, true).
		Headers("A", "B").
		Row("1", "2")
	got := tbl.String()

	if strings.Contains(got, "└") || strings.Contains(got, "┴") {
		t.Errorf("No bottom border should appear:\n%s", got)
	}

	if !strings.Contains(got, "┌") {
		t.Errorf("Top border should still appear:\n%s", got)
	}
}

func TestTable_FixedWidthColumns(t *testing.T) {
	t.Parallel()

	// Columns 0 and 2 have fixed widths (5 and 10), column 1 has no fixed width.
	// The remaining space should go to column 1 only.
	tbl := New().Width(20).Border(style.NormalBorder()).
		Headers("A", "B", "C").
		Row("a", "bb", "c").
		ColumnStyles([]style.Style{
			style.NewStyle().Width(5),
			style.NewStyle(),
			style.NewStyle().Width(10),
		})
	got := tbl.String()

	visible := stripANSI(got)
	for line := range strings.SplitSeq(visible, "\n") {
		lineWidth := style.CellWidth(line)
		if lineWidth != 20 {
			t.Errorf("Line width = %d, want 20. Line: %q", lineWidth, line)
		}
	}
}

func TestTable_FixedWidthColumnsNotShrunk(t *testing.T) {
	t.Parallel()

	// Width=10, 3 columns each fixed at 5 = 15 content + 4 border = 19 total.
	// Fixed columns should NOT be shrunk even when total exceeds available.
	tbl := New().Width(10).Border(style.NormalBorder()).
		Headers("A", "B", "C").
		Row("a", "b", "c").
		ColumnStyles([]style.Style{
			style.NewStyle().Width(5),
			style.NewStyle().Width(5),
			style.NewStyle().Width(5),
		})

	// Should still build without panicking
	got := tbl.String()
	if got == "" {
		t.Error("Table should render even with overflow")
	}
}

func TestTable_FixedWidthRespected(t *testing.T) {
	t.Parallel()

	// Column 0 is fixed at 4. Total width = 20. Border chars = 3 (left, mid, right).
	// Available = 17. Fixed = 4. Remaining = 13 for column 1.
	tbl := New().Width(20).Border(style.NormalBorder()).
		Headers("Idx", "Name").
		Row("1", "test").
		ColumnStyles([]style.Style{
			style.NewStyle().Width(4).Align(style.Right),
			style.NewStyle(),
		})
	got := tbl.String()

	visible := stripANSI(got)
	lines := strings.SplitSeq(visible, "\n")

	// Data row should have the index column at width 4
	for line := range lines {
		if strings.Contains(line, "1") && strings.Contains(line, "test") {
			// The "1" should be right-aligned in a 4-char column
			if !strings.Contains(line, "   1") {
				t.Errorf("Fixed-width column should be right-aligned: %q", line)
			}
		}
	}
}

func TestTable_SetRows_PreservesSelection(t *testing.T) {
	t.Parallel()

	tbl := New().Row("a").Row("b").Row("c")
	tbl.Select(1)

	if tbl.SelectedIndex() != 1 {
		t.Fatalf("Select(1) → SelectedIndex = %d, want 1", tbl.SelectedIndex())
	}

	tbl.SetRows([][]string{{"x"}, {"y"}})

	if tbl.SelectedIndex() != 1 {
		t.Errorf("SetRows should preserve selection, got %d", tbl.SelectedIndex())
	}

	if len(tbl.rows) != 2 {
		t.Errorf("After SetRows: len = %d, want 2", len(tbl.rows))
	}
}

func TestTable_SetRows_PerRowCacheDiff(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Headers("A", "B").
		Row("a", "b").
		Row("c", "d").
		Row("e", "f")

	result1 := tbl.String()

	// SetRows with same data — should produce identical output
	tbl.SetRows([][]string{{"a", "b"}, {"c", "d"}, {"e", "f"}})
	result2 := tbl.String()

	if result1 != result2 {
		t.Error("Same data should produce same output")
	}

	// Change only row 1 — rows 0 and 2 should use cached renders
	tbl.SetRows([][]string{{"a", "b"}, {"CHANGED", "d"}, {"e", "f"}})
	result3 := tbl.String()

	if !strings.Contains(result3, "CHANGED") {
		t.Error("Changed row should appear in output")
	}
}

func TestTable_SetRows_RowCountChange_FullRebuild(t *testing.T) {
	t.Parallel()

	tbl := New().Width(20).Border(style.NormalBorder()).
		Headers("A").
		Row("a").Row("b")
	tbl.String()

	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}})
	tbl.String()

	if len(tbl.rowCache) != 3 {
		t.Errorf("rowCache len = %d, want 3", len(tbl.rowCache))
	}
}

func TestTable_SetRows_ColWidthChange_InvalidatesAllRows(t *testing.T) {
	t.Parallel()

	// Row 1 has wide content, row 2 has narrow content.
	// When row 1 is replaced with narrow content, column widths shrink,
	// and row 2's cached render (rendered at the old wider widths)
	// must be invalidated.
	tbl := New().Width(20).Border(style.NormalBorder()).
		Headers("A", "B").
		ColumnStyles([]style.Style{{}, {}})

	// First: wide row + narrow row
	tbl.SetRows([][]string{{"WWWWWWWWWW", "x"}, {"y", "z"}})
	tbl.String()

	// Replace wide row with narrow — column widths change
	tbl.SetRows([][]string{{"a", "x"}, {"y", "z"}})
	result := tbl.String()

	visible := stripANSI(result)
	lines := strings.SplitSeq(visible, "\n")

	for line := range lines {
		if line == "" {
			continue
		}

		lineWidth := style.CellWidth(line)
		if lineWidth != 20 {
			t.Errorf("Line width = %d, want 20. Line: %q", lineWidth, line)
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder

	i := 0

	for i < len(s) {
		if s[i] == '\x1b' {
			i++

			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}

			if i < len(s) {
				i++
			}

			continue
		}

		b.WriteByte(s[i])
		i++
	}

	return b.String()
}
