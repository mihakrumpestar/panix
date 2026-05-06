package table

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})

	if tbl.cfg.Width != 0 {
		t.Errorf("Default width = %d, want 0", tbl.cfg.Width)
	}

	if tbl.bordered {
		t.Error("Default bordered should be false")
	}

	if tbl.SelectedIndex() != -1 {
		t.Errorf("Default SelectedIndex = %d, want -1", tbl.SelectedIndex())
	}
}

func TestTable_Empty(t *testing.T) {
	t.Parallel()

	got := New(Config{}).String()
	if got != "" {
		t.Errorf("Empty table String() = %q, want \"\"", got)
	}
}

func TestTable_HeadersOnly_NoBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Headers: []string{"Name", "Value"}})
	got := tbl.String()

	if !strings.Contains(got, "Name") || !strings.Contains(got, "Value") {
		t.Errorf("Headers-only no border = %q, missing header text", got)
	}
}

func TestTable_HeadersAndRows_WithBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 20, Border: style.NormalBorder(), Headers: []string{"Name", "Value"}})
	tbl.SetRows([][]string{{"key1", "val1"}, {"key2", "val2"}})
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

	tbl := New(Config{Width: 20, Border: style.NormalBorder()})
	tbl.SetRows([][]string{{"a", "b"}, {"c", "d"}})
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

	tbl := New(Config{Headers: []string{"A", "B"}})
	tbl.SetRows([][]string{{"1", "2"}})
	got := tbl.String()

	if strings.Contains(got, "┌") || strings.Contains(got, "│") {
		t.Errorf("No-border table should not contain border chars:\n%s", got)
	}
}

func TestTable_WidthExpandsToFill(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 30, Border: style.NormalBorder(), Headers: []string{"A", "B"}})
	tbl.SetRows([][]string{{"1", "2"}})
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

	tbl := New(Config{Width: 12, Border: style.NormalBorder(), Headers: []string{"LongHeader1", "LongHeader2"}})
	tbl.SetRows([][]string{{"longcontent1", "longcontent2"}})
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

	tbl := New(Config{Width: 20, Headers: []string{"A", "B"}})
	tbl.SetRows([][]string{{"1", "2"}})
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

	tbl := New(Config{
		Width:   20,
		Border:  style.NormalBorder(),
		Headers: []string{"A", "B"},
		ColumnStyles: []style.Style{
			style.NewStyle().Foreground(style.Color("#8BE9FD")),
			style.NewStyle().Foreground(style.Color("#FF5555")),
		},
	})
	tbl.SetRows([][]string{{"x", "y"}})
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

	tbl := New(Config{Headers: []string{"A"}})
	tbl.SetRows([][]string{{"1"}, {"2"}, {"3"}})

	if len(tbl.rows) != 3 {
		t.Errorf("Rows count = %d, want 3", len(tbl.rows))
	}
}

func TestTable_Wrap(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Wrap: true})
	if !tbl.cfg.Wrap {
		t.Error("Wrap(true) should set wrap = true")
	}
}

func TestTable_WrapFalse_Truncates(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 10, Border: style.NormalBorder(), Headers: []string{"A"}})
	tbl.SetRows([][]string{{"abcdefghijXYZ"}})
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

	tbl := New(Config{
		Width:       20,
		Border:      style.NormalBorder(),
		BorderStyle: style.NewStyle().Foreground(style.Color("#FF0000")),
		Headers:     []string{"A"},
	})
	tbl.SetRows([][]string{{"x"}})
	got := tbl.String()

	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Errorf("BorderStyle color not applied to border:\n%s", got)
	}
}

func TestTable_CalculateColumnWidths(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Headers: []string{"Name", "Value"}})
	tbl.SetRows([][]string{{"longkey", "v"}})

	widths := make([]int, 2)
	tbl.contentWidths(2, widths)

	if widths[0] < 7 {
		t.Errorf("Col 0 width = %d, want >= 7", widths[0])
	}

	if widths[1] < 5 {
		t.Errorf("Col 1 width = %d, want >= 5", widths[1])
	}
}

func TestTable_HiddenBorder(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Border: style.HiddenBorder(), Headers: []string{"A", "B"}})
	tbl.SetRows([][]string{{"1", "2"}})
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
	tbl := New(Config{
		Width:               20,
		Border:              style.NormalBorder(),
		SelectionBackground: selBg,
		Headers:             []string{"A", "B"},
	})
	tbl.SetRows([][]string{{"x", "y"}, {"z", "w"}})
	tbl.Select(0)
	got := tbl.String()

	if !strings.Contains(got, "\x1b[48;2;51;51;51m") {
		t.Errorf("Selected row missing background color:\n%s", got)
	}
}

func TestTable_HandleNavigation(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}})
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

	tbl := New(Config{})
	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}})

	if !tbl.HandleNavigation("right", false) {
		t.Error("right with no selection should select first row")
	}

	if tbl.SelectedIndex() != 0 {
		t.Errorf("After initial right: SelectedIndex = %d, want 0", tbl.SelectedIndex())
	}
}

func TestTable_HandleNavigation_Boundary(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows([][]string{{"a"}, {"b"}})
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

	tbl := New(Config{})

	if tbl.HandleNavigation("right", false) {
		t.Error("Navigation on empty table should fail")
	}
}

func TestTable_HandleNavigation_ActiveInnerViewport(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows([][]string{{"a"}, {"b"}})
	tbl.Select(0)

	if tbl.HandleNavigation("right", true) {
		t.Error("Navigation with active inner viewport should fail")
	}
}

func TestTable_ZoneMarkersInOutput(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 10}).SetZonePrefix("test-tbl")
	tbl.SetRows([][]string{{"a"}, {"b"}})
	got := tbl.String()

	if !strings.Contains(got, "z") {
		t.Errorf("Zone markers should appear in output:\n%s", got)
	}
}

func TestTable_SelectionBackgroundNoOuterBorderBg(t *testing.T) {
	t.Parallel()

	selBg := style.Color("#333333")
	tbl := New(Config{
		Width:               20,
		Border:              style.NormalBorder(),
		SelectionBackground: selBg,
		Headers:             []string{"A", "B"},
	})
	tbl.SetRows([][]string{{"x", "y"}})
	tbl.Select(0)
	got := tbl.String()

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
		Headers:             []string{"A", "B"},
		ColumnStyles: []style.Style{
			style.NewStyle().Foreground(style.Color("#8BE9FD")),
			style.NewStyle(),
		},
	})
	tbl.SetRows([][]string{{"x", "y"}})
	tbl.Select(0)
	got := tbl.String()

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

	bgPrefix := "\x1b[48;2;51;51;51m"

	count := strings.Count(selLine, bgPrefix)
	if count < 2 {
		t.Errorf("Selection bg should be re-emitted after cell resets, found %d occurrences in:\n%s", count, selLine)
	}
}

func TestTable_HandleMouseClick_DeselectOutsideReturnsTrue(t *testing.T) {
	t.Parallel()

	tbl := New(Config{}).SetZonePrefix("test")
	tbl.SetRows([][]string{{"a"}, {"b"}})
	tbl.Select(0)

	zeroterm.SetCurrentLines([]string{"no zones here", "another line"})

	changed := tbl.HandleMouseClick(zeroterm.MouseClickMsg{X: 0, Y: 0})

	if !changed {
		t.Error("HandleMouseClick should return true when deselecting via outside click")
	}

	if tbl.SelectedIndex() != -1 {
		t.Errorf("SelectedIndex = %d, want -1 after outside click", tbl.SelectedIndex())
	}
}

func TestTable_HandleNavigation_UpDownIgnored(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}})
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

func TestTable_FixedWidthColumns(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:   20,
		Border:  style.NormalBorder(),
		Headers: []string{"A", "B", "C"},
		ColumnStyles: []style.Style{
			style.NewStyle().Width(5),
			style.NewStyle(),
			style.NewStyle().Width(10),
		},
	})
	tbl.SetRows([][]string{{"a", "bb", "c"}})
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

	tbl := New(Config{
		Width:   10,
		Border:  style.NormalBorder(),
		Headers: []string{"A", "B", "C"},
		ColumnStyles: []style.Style{
			style.NewStyle().Width(5),
			style.NewStyle().Width(5),
			style.NewStyle().Width(5),
		},
	})
	tbl.SetRows([][]string{{"a", "b", "c"}})

	got := tbl.String()
	if got == "" {
		t.Error("Table should render even with overflow")
	}
}

func TestTable_FixedWidthRespected(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:   20,
		Border:  style.NormalBorder(),
		Headers: []string{"Idx", "Name"},
		ColumnStyles: []style.Style{
			style.NewStyle().Width(4).Align(style.Right),
			style.NewStyle(),
		},
	})
	tbl.SetRows([][]string{{"1", "test"}})
	got := tbl.String()

	visible := stripANSI(got)
	lines := strings.SplitSeq(visible, "\n")

	for line := range lines {
		if strings.Contains(line, "1") && strings.Contains(line, "test") {
			if !strings.Contains(line, "   1") {
				t.Errorf("Fixed-width column should be right-aligned: %q", line)
			}
		}
	}
}

func TestTable_SetRows_PreservesSelection(t *testing.T) {
	t.Parallel()

	tbl := New(Config{})
	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}})
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

	tbl := New(Config{Width: 20, Border: style.NormalBorder(), Headers: []string{"A", "B"}})
	tbl.SetRows([][]string{{"a", "b"}, {"c", "d"}, {"e", "f"}})

	result1 := tbl.String()

	tbl.SetRows([][]string{{"a", "b"}, {"c", "d"}, {"e", "f"}})
	result2 := tbl.String()

	if result1 != result2 {
		t.Error("Same data should produce same output")
	}

	tbl.SetRows([][]string{{"a", "b"}, {"CHANGED", "d"}, {"e", "f"}})
	result3 := tbl.String()

	if !strings.Contains(result3, "CHANGED") {
		t.Error("Changed row should appear in output")
	}
}

func TestTable_SetRows_RowCountChange_FullRebuild(t *testing.T) {
	t.Parallel()

	tbl := New(Config{Width: 20, Border: style.NormalBorder(), Headers: []string{"A"}})
	tbl.SetRows([][]string{{"a"}, {"b"}})
	_ = tbl.String()

	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}})
	result := tbl.String()

	if !strings.Contains(result, "c") {
		t.Errorf("Full rebuild should include new row data, got: %s", result)
	}
}

func TestTable_SetRows_ColWidthChange_InvalidatesAllRows(t *testing.T) {
	t.Parallel()

	tbl := New(Config{
		Width:        20,
		Border:       style.NormalBorder(),
		Headers:      []string{"A", "B"},
		ColumnStyles: []style.Style{{}, {}},
	})

	tbl.SetRows([][]string{{"WWWWWWWWWW", "x"}, {"y", "z"}})
	_ = tbl.String()

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

func stripANSI(str string) string {
	var builder strings.Builder

	pos := 0

	for pos < len(str) {
		if str[pos] == '\x1b' {
			pos++

			for pos < len(str) && (str[pos] < 'A' || str[pos] > 'Z') && (str[pos] < 'a' || str[pos] > 'z') {
				pos++
			}

			if pos < len(str) {
				pos++
			}

			continue
		}

		builder.WriteByte(str[pos])
		pos++
	}

	return builder.String()
}
