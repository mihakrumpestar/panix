package render

import (
	"testing"
)

func TestDiffNoChanges(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.WriteANSIString(0, 0, "Hello")

	prevBuf := NewCellBuf(10, 3)
	prevBuf.copyFrom(buf)

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 0 {
		t.Errorf("Diff with identical buffers returned %d diffs, want 0", len(diffs))
	}
}

func TestDiffLineVersionSkip(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.WriteANSIString(0, 0, "AAAAA")
	buf.WriteANSIString(0, 1, "BBBBB")
	buf.WriteANSIString(0, 2, "CCCCC")

	prevBuf := NewCellBuf(10, 3)
	prevBuf.copyFrom(buf)

	buf.WriteANSIString(0, 1, "XXXXX")

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	if diffs[0].Y != 1 {
		t.Errorf("diff.Y = %d, want 1", diffs[0].Y)
	}
}

func TestDiffSingleCharChange(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.WriteANSIString(0, 0, "AAAAA")

	prevBuf := NewCellBuf(10, 3)
	prevBuf.copyFrom(buf)

	buf.SetCell(4, 0, Cell{Content: "B", Width: 1})

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	if diffs[0].Y != 0 {
		t.Errorf("diff.Y = %d, want 0", diffs[0].Y)
	}
}

func TestDiffMultipleLines(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.WriteANSIString(0, 0, "line1")
	buf.WriteANSIString(0, 1, "line2")
	buf.WriteANSIString(0, 2, "line3")

	prevBuf := NewCellBuf(10, 3)
	prevBuf.copyFrom(buf)

	buf.SetCell(0, 0, Cell{Content: "X", Width: 1})
	buf.SetCell(0, 1, Cell{Content: "Y", Width: 1})
	buf.SetCell(0, 2, Cell{Content: "Z", Width: 1})

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 3 {
		t.Fatalf("expected 3 diffs, got %d", len(diffs))
	}
}

func TestDiffDifferentWidths(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	prevBuf := NewCellBuf(5, 3)

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 3 {
		t.Errorf("different widths should cause full redraw: got %d diffs, want 3", len(diffs))
	}
}

func TestDiffNewBufTaller(t *testing.T) {
	t.Parallel()

	prevBuf := NewCellBuf(10, 3)
	prevBuf.WriteANSIString(0, 0, "old1")
	prevBuf.WriteANSIString(0, 1, "old2")
	prevBuf.WriteANSIString(0, 2, "old3")

	buf := NewCellBuf(10, 5)

	for y := range 3 {
		for x := range 10 {
			buf.SetCell(x, y, prevBuf.CellAt(x, y))
		}

		buf.SetLineVersion(y, prevBuf.LineVersion(y))
	}

	buf.WriteANSIString(0, 3, "new4")
	buf.WriteANSIString(0, 4, "new5")

	diffs := Diff(buf, prevBuf)
	foundNewLines := false

	for _, d := range diffs {
		if d.Y >= 3 {
			foundNewLines = true
		}
	}

	if !foundNewLines {
		t.Error("new lines (3,4) should appear in diffs")
	}
}

func TestDiffStyleChange(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 1)
	buf.WriteANSIString(0, 0, "Hello")

	prevBuf := NewCellBuf(10, 1)
	prevBuf.copyFrom(buf)

	cell := buf.CellAt(0, 0)
	cell.Fg = NewColor(255, 0, 0)
	buf.SetCell(0, 0, cell)

	diffs := Diff(buf, prevBuf)
	if len(diffs) == 0 {
		t.Error("style change should produce diffs")
	}
}

func TestLineChangedIdentical(t *testing.T) {
	t.Parallel()

	line := []Cell{
		{Content: "A", Width: 1},
		{Content: "B", Width: 1},
		{Content: "C", Width: 1},
	}

	if lineChanged(line, line) {
		t.Error("identical lines should not be changed")
	}
}

func TestLineChangedFromEmptyAllEmpty(t *testing.T) {
	t.Parallel()

	line := make([]Cell, 3)
	for i := range line {
		line[i] = EmptyCell
	}

	if lineChangedFromEmpty(line) {
		t.Error("all empty line should not be changed from empty")
	}
}

func TestLineChangedFromEmptyWithContent(t *testing.T) {
	t.Parallel()

	line := []Cell{
		{Content: "A", Width: 1},
		EmptyCell,
	}

	if !lineChangedFromEmpty(line) {
		t.Error("line with content should be changed from empty")
	}
}

func TestFullRedraw(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)

	diffs := fullRedraw(buf)
	if len(diffs) != 3 {
		t.Fatalf("fullRedraw: got %d diffs, want 3", len(diffs))
	}

	for i, d := range diffs {
		if d.Y != i {
			t.Errorf("diff[%d].Y = %d, want %d", i, d.Y, i)
		}
	}
}

func TestLineChangedNewLineLongerWithContent(t *testing.T) {
	t.Parallel()

	oldLine := []Cell{
		{Content: "A", Width: 1},
		{Content: "B", Width: 1},
	}
	newLine := []Cell{
		{Content: "A", Width: 1},
		{Content: "B", Width: 1},
		{Content: "X", Width: 1},
	}

	if !lineChanged(newLine, oldLine) {
		t.Error("new line with extra content should be changed")
	}
}

func TestLineChangedNewLineLongerAllEmpty(t *testing.T) {
	t.Parallel()

	oldLine := []Cell{
		{Content: "A", Width: 1},
	}
	newLine := []Cell{
		{Content: "A", Width: 1},
		EmptyCell,
	}

	if lineChanged(newLine, oldLine) {
		t.Error("extra empty cell should produce no diff")
	}
}

func TestDiffOldBufTaller(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 2)
	prevBuf := NewCellBuf(10, 3)

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 0 {
		t.Errorf("old buf taller should not produce extra diffs, got %d", len(diffs))
	}
}

func TestDiffSameHeightNoChanges(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.WriteANSIString(0, 0, "Hello")

	prevBuf := NewCellBuf(10, 3)
	prevBuf.copyFrom(buf)

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 0 {
		t.Errorf("identical buffers should have 0 diffs, got %d", len(diffs))
	}
}

func TestLineChangedDetectsShrinkingLine(t *testing.T) {
	t.Parallel()

	oldLine := make([]Cell, 79)
	oldLine[0] = Cell{Content: "A", Width: 1}
	oldLine[1] = Cell{Content: "B", Width: 1}

	oldLine[2] = Cell{Content: "C", Width: 1}
	for i := 3; i < 78; i++ {
		oldLine[i] = EmptyCell
	}

	oldLine[78] = Cell{Content: "│", Width: 1, Fg: NewColor16(7)}

	newLine := make([]Cell, 79)

	newLine[0] = Cell{Content: "X", Width: 1}
	for i := 1; i < 78; i++ {
		newLine[i] = EmptyCell
	}

	newLine[78] = Cell{Content: "│", Width: 1, Fg: NewColor16(7)}

	if !lineChanged(newLine, oldLine) {
		t.Error("line with changed cells should be detected as changed")
	}
}

func TestDiffLineVersionOptimizationWorks(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.WriteANSIString(0, 0, "line1")
	buf.WriteANSIString(0, 1, "line2")
	buf.WriteANSIString(0, 2, "line3")

	prevBuf := NewCellBuf(10, 3)
	prevBuf.copyFrom(buf)

	// Only modify line 1 — line 0 and 2 should be skipped via lineVersion
	buf.SetCell(5, 1, Cell{Content: "X", Width: 1})

	diffs := Diff(buf, prevBuf)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	if diffs[0].Y != 1 {
		t.Errorf("diff.Y = %d, want 1", diffs[0].Y)
	}
}
