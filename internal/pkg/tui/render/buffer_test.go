package render

import (
	"testing"
)

func TestNewCellBuf(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 5)
	if buf.Width() != 10 {
		t.Errorf("Width() = %d, want 10", buf.Width())
	}

	if buf.Height() != 5 {
		t.Errorf("Height() = %d, want 5", buf.Height())
	}

	for y := range 5 {
		for x := range 10 {
			cell := buf.CellAt(x, y)
			if cell.Content != " " {
				t.Errorf("CellAt(%d,%d).Content = %q, want %q", x, y, cell.Content, " ")
			}
		}
	}
}

func TestNewCellBufZeroSize(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(0, 0)
	if buf.Width() != 0 || buf.Height() != 0 {
		t.Errorf("zero-size buffer: width=%d height=%d", buf.Width(), buf.Height())
	}
}

func TestCellAtBounds(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)

	// Out of bounds should return EmptyCell
	cell := buf.CellAt(-1, 0)
	if cell.Content != EmptyCell.Content {
		t.Error("negative x should return EmptyCell")
	}

	cell = buf.CellAt(0, -1)
	if cell.Content != EmptyCell.Content {
		t.Error("negative y should return EmptyCell")
	}

	cell = buf.CellAt(5, 0)
	if cell.Content != EmptyCell.Content {
		t.Error("x >= width should return EmptyCell")
	}

	cell = buf.CellAt(0, 3)
	if cell.Content != EmptyCell.Content {
		t.Error("y >= height should return EmptyCell")
	}
}

func TestSetCellBasic(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	c := Cell{Content: "X", Width: 1, Fg: NewColor(255, 0, 0)}
	buf.SetCell(5, 1, c)

	got := buf.CellAt(5, 1)
	if got.Content != "X" {
		t.Errorf("CellAt(5,1).Content = %q, want %q", got.Content, "X")
	}

	if got.Fg != NewColor(255, 0, 0) {
		t.Error("CellAt(5,1).Fg mismatch")
	}
}

func TestSetCellOutOfBoundsIsNoop(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	c := Cell{Content: "X", Width: 1}
	buf.SetCell(-1, 0, c)
	buf.SetCell(10, 0, c)
	buf.SetCell(0, -1, c)
	buf.SetCell(0, 3, c)

	// All cells should still be empty
	for y := range 3 {
		for x := range 10 {
			cell := buf.CellAt(x, y)
			if cell.Content != " " {
				t.Errorf("CellAt(%d,%d).Content = %q, expected empty", x, y, cell.Content)
			}
		}
	}
}

func TestSetCellNoChangeSkipsVersionBump(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.SetCell(5, 1, Cell{Content: "X", Width: 1})

	v := buf.Version()
	lv := buf.LineVersion(1)

	// Setting same cell again should not bump versions
	buf.SetCell(5, 1, Cell{Content: "X", Width: 1})

	if buf.Version() != v {
		t.Error("version should not change when setting identical cell")
	}

	if buf.LineVersion(1) != lv {
		t.Error("lineVersion should not change when setting identical cell")
	}
}

func TestSetCellVersionIncrement(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	v0 := buf.Version()
	lv0 := buf.LineVersion(0)

	buf.SetCell(0, 0, Cell{Content: "A", Width: 1})

	if buf.Version() == v0 {
		t.Error("version should increment after SetCell with different content")
	}

	if buf.LineVersion(0) == lv0 {
		t.Error("lineVersion should increment after SetCell on that line")
	}
	// Other lines should not be affected
	if buf.LineVersion(1) != 0 {
		t.Error("lineVersion for unmodified line should be 0")
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)
	buf.SetCell(0, 0, Cell{Content: "X", Width: 1, Fg: NewColor(255, 0, 0)})
	buf.SetCell(5, 1, Cell{Content: "Y", Width: 1, Attrs: AttrBold})

	buf.Clear()

	for y := range 3 {
		for x := range 10 {
			cell := buf.CellAt(x, y)
			if cell.Content != " " {
				t.Errorf("CellAt(%d,%d) after clear: Content=%q, want %q", x, y, cell.Content, " ")
			}

			if cell.Fg != DefaultColor {
				t.Errorf("CellAt(%d,%d) after clear: Fg should be default", x, y)
			}
		}
	}
	// After clear, version should have incremented
	if buf.Version() == 0 {
		t.Error("version should increment after Clear()")
	}
}

func TestResizeGrow(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)
	buf.SetCell(0, 0, Cell{Content: "A", Width: 1})
	buf.SetCell(4, 2, Cell{Content: "B", Width: 1})

	buf.Resize(10, 5)

	if buf.Width() != 10 || buf.Height() != 5 {
		t.Fatalf("Resize: width=%d height=%d, want 10x5", buf.Width(), buf.Height())
	}

	// Old content should be preserved
	if buf.CellAt(0, 0).Content != "A" {
		t.Error("old cell (0,0) should be preserved")
	}

	if buf.CellAt(4, 2).Content != "B" {
		t.Error("old cell (4,2) should be preserved")
	}

	// New cells should be empty
	if buf.CellAt(9, 4).Content != " " {
		t.Error("new cell should be empty")
	}
}

func TestResizeShrink(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 5)
	buf.SetCell(0, 0, Cell{Content: "A", Width: 1})
	buf.SetCell(9, 4, Cell{Content: "B", Width: 1})

	buf.Resize(5, 3)

	if buf.Width() != 5 || buf.Height() != 3 {
		t.Fatalf("Resize: width=%d height=%d, want 5x3", buf.Width(), buf.Height())
	}

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("cell (0,0) should be preserved")
	}
	// (9,4) was outside the new buffer
	cell := buf.CellAt(9, 4)
	if cell.Content != " " {
		t.Error("cell outside new bounds should return EmptyCell")
	}
}

func TestResizeSameSizeIsNoop(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 5)
	buf.SetCell(0, 0, Cell{Content: "X", Width: 1})
	v := buf.Version()

	buf.Resize(10, 5)

	if buf.Version() != v {
		t.Error("Resize with same dimensions should not bump version")
	}

	if buf.CellAt(0, 0).Content != "X" {
		t.Error("Resize with same dimensions should not change content")
	}
}

func TestLine(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)
	buf.SetCell(0, 1, Cell{Content: "H", Width: 1})
	buf.SetCell(1, 1, Cell{Content: "i", Width: 1})

	line := buf.Line(1)
	if len(line) != 5 {
		t.Fatalf("Line(1) length = %d, want 5", len(line))
	}

	if line[0].Content != "H" {
		t.Errorf("Line(1)[0].Content = %q, want %q", line[0].Content, "H")
	}

	if line[1].Content != "i" {
		t.Errorf("Line(1)[1].Content = %q, want %q", line[1].Content, "i")
	}
}

func TestLineOutOfBounds(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)
	if line := buf.Line(-1); line != nil {
		t.Errorf("Line(-1) = %v, want nil", line)
	}

	if line := buf.Line(3); line != nil {
		t.Errorf("Line(3) = %v, want nil", line)
	}
}

func TestSetLineVersion(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)
	buf.SetLineVersion(1, 42)

	if buf.LineVersion(1) != 42 {
		t.Errorf("LineVersion(1) = %d, want 42", buf.LineVersion(1))
	}

	buf.SetLineVersion(-1, 99)
	buf.SetLineVersion(3, 99)
}

func TestCopyFrom(t *testing.T) {
	t.Parallel()

	src := NewCellBuf(5, 3)
	src.SetCell(0, 0, Cell{Content: "X", Width: 1, Fg: NewColor(255, 0, 0)})
	src.SetCell(3, 2, Cell{Content: "Y", Width: 1, Attrs: AttrBold})

	dst := NewCellBuf(5, 3)
	dst.copyFrom(src)

	if dst.CellAt(0, 0).Content != "X" {
		t.Error("copyFrom should copy content")
	}

	if dst.CellAt(0, 0).Fg != NewColor(255, 0, 0) {
		t.Error("copyFrom should copy fg color")
	}

	if dst.CellAt(3, 2).Content != "Y" {
		t.Error("copyFrom should copy content at (3,2)")
	}

	if dst.CellAt(3, 2).Attrs != AttrBold {
		t.Error("copyFrom should copy attrs")
	}

	if dst.Version() != src.Version() {
		t.Error("copyFrom should copy version")
	}
}

func TestCopyFromDifferentSize(t *testing.T) {
	t.Parallel()

	src := NewCellBuf(10, 5)
	src.SetCell(9, 4, Cell{Content: "Z", Width: 1})

	dst := NewCellBuf(5, 3)
	dst.copyFrom(src)

	if dst.Width() != 10 || dst.Height() != 5 {
		t.Errorf("copyFrom should resize dst: width=%d height=%d", dst.Width(), dst.Height())
	}

	if dst.CellAt(9, 4).Content != "Z" {
		t.Error("copyFrom should copy content after resize")
	}
}

func TestWriteANSIStringPlain(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 5)
	endX, endY := buf.WriteANSIString(0, 0, "Hello")

	if endX != 5 {
		t.Errorf("endX = %d, want 5", endX)
	}

	if endY != 0 {
		t.Errorf("endY = %d, want 0", endY)
	}

	expected := []struct {
		x   int
		ch  string
		fg  Color
		bg  Color
		att Attr
	}{
		{0, "H", DefaultColor, DefaultColor, 0},
		{1, "e", DefaultColor, DefaultColor, 0},
		{2, "l", DefaultColor, DefaultColor, 0},
		{3, "l", DefaultColor, DefaultColor, 0},
		{4, "o", DefaultColor, DefaultColor, 0},
	}
	for _, e := range expected {
		cell := buf.CellAt(e.x, 0)
		if cell.Content != e.ch {
			t.Errorf("CellAt(%d,0).Content = %q, want %q", e.x, cell.Content, e.ch)
		}

		if cell.Fg != e.fg {
			t.Errorf("CellAt(%d,0).Fg mismatch", e.x)
		}
	}
}

func TestWriteANSIStringWithNewlines(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 5)
	endX, endY := buf.WriteANSIString(0, 0, "AB\nCD")

	if endX != 2 {
		t.Errorf("endX = %d, want 2", endX)
	}

	if endY != 1 {
		t.Errorf("endY = %d, want 1", endY)
	}

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("first char")
	}

	if buf.CellAt(0, 1).Content != "C" {
		t.Error("second line first char")
	}

	if buf.CellAt(1, 1).Content != "D" {
		t.Error("second line second char")
	}
}

func TestWriteANSIStringWithCarriageReturn(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "ABC\rX")

	if buf.CellAt(0, 0).Content != "X" {
		t.Errorf("CellAt(0,0).Content = %q, want %q (CR should move cursor to col 0)", buf.CellAt(0, 0).Content, "X")
	}

	if !buf.CellAt(1, 0).VisualEqual(EmptyCell) {
		t.Errorf("CellAt(1,0) should be EmptyCell after line padding, got Content=%q", buf.CellAt(1, 0).Content)
	}
}

func TestWriteANSIStringWithTab(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	endX, _ := buf.WriteANSIString(0, 0, "A\tB")

	if endX != 9 {
		t.Errorf("endX = %d, want 9 (tab stops at 8)", endX)
	}

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("first char")
	}

	if buf.CellAt(8, 0).Content != "B" {
		t.Errorf("CellAt(8,0).Content = %q, want B", buf.CellAt(8, 0).Content)
	}
}

func TestWriteANSIStringSGRReset(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[1;38;2;255;0;0mRed\x1b[0mPlain")

	if buf.CellAt(0, 0).Fg != NewColor(255, 0, 0) {
		t.Error("first char should be red")
	}

	if buf.CellAt(0, 0).Attrs&AttrBold == 0 {
		t.Error("first char should be bold")
	}

	if buf.CellAt(3, 0).Fg != DefaultColor {
		t.Errorf("after reset, fg should be default, got %v", buf.CellAt(3, 0).Fg)
	}

	if buf.CellAt(3, 0).Attrs != 0 {
		t.Error("after reset, attrs should be 0")
	}
}

func TestWriteANSIStringSGR16ColorFg(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[31mR\x1b[32mG\x1b[34mB\x1b[0m")

	if buf.CellAt(0, 0).Fg.IsDefault() {
		t.Error("SGR 31 should set fg to red")
	}

	if buf.CellAt(0, 0).Fg.ColorType() != colorType16 {
		t.Errorf("SGR 31 fg type = %d, want colorType16", buf.CellAt(0, 0).Fg.ColorType())
	}

	if buf.CellAt(0, 0).Fg.PaletteIndex() != 1 {
		t.Errorf("SGR 31 fg palette = %d, want 1", buf.CellAt(0, 0).Fg.PaletteIndex())
	}

	if buf.CellAt(1, 0).Fg.PaletteIndex() != 2 {
		t.Errorf("SGR 32 fg palette = %d, want 2", buf.CellAt(1, 0).Fg.PaletteIndex())
	}

	if buf.CellAt(2, 0).Fg.PaletteIndex() != 4 {
		t.Errorf("SGR 34 fg palette = %d, want 4", buf.CellAt(2, 0).Fg.PaletteIndex())
	}

	if !buf.CellAt(3, 0).Fg.IsDefault() {
		t.Error("after reset, fg should be default")
	}
}

func TestWriteANSIStringSGR16ColorBg(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[41mX\x1b[0m")

	if buf.CellAt(0, 0).Bg.IsDefault() {
		t.Error("SGR 41 should set bg to red")
	}

	if buf.CellAt(0, 0).Bg.ColorType() != colorType16 {
		t.Errorf("SGR 41 bg type = %d, want colorType16", buf.CellAt(0, 0).Bg.ColorType())
	}

	if buf.CellAt(0, 0).Bg.PaletteIndex() != 1 {
		t.Errorf("SGR 41 bg palette = %d, want 1", buf.CellAt(0, 0).Bg.PaletteIndex())
	}
}

func TestWriteANSIStringSGRBrightColorFg(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[91mX\x1b[0m")

	if buf.CellAt(0, 0).Fg.IsDefault() {
		t.Error("SGR 91 should set bright red fg")
	}

	if buf.CellAt(0, 0).Fg.ColorType() != colorType16 {
		t.Errorf("SGR 91 fg type = %d, want colorType16", buf.CellAt(0, 0).Fg.ColorType())
	}

	if buf.CellAt(0, 0).Fg.PaletteIndex() != 9 {
		t.Errorf("SGR 91 fg palette = %d, want 9 (bright red)", buf.CellAt(0, 0).Fg.PaletteIndex())
	}
}

func TestWriteANSIStringSGRBrightColorBg(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[104mX\x1b[0m")

	if buf.CellAt(0, 0).Bg.IsDefault() {
		t.Error("SGR 104 should set bright blue bg")
	}

	if buf.CellAt(0, 0).Bg.ColorType() != colorType16 {
		t.Errorf("SGR 104 bg type = %d, want colorType16", buf.CellAt(0, 0).Bg.ColorType())
	}

	if buf.CellAt(0, 0).Bg.PaletteIndex() != 12 {
		t.Errorf("SGR 104 bg palette = %d, want 12 (bright blue)", buf.CellAt(0, 0).Bg.PaletteIndex())
	}
}

func TestWriteANSIStringSGR256Color(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[38;5;196mX")

	cell := buf.CellAt(0, 0)
	if cell.Fg.IsDefault() {
		t.Error("fg should not be default after 256-color SGR")
	}
}

func TestWriteANSIStringSGR24bitColor(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[38;2;128;64;32mX")

	cell := buf.CellAt(0, 0)
	if cell.Fg.R() != 128 {
		t.Errorf("fg.R = %d, want 128", cell.Fg.R())
	}

	if cell.Fg.G() != 64 {
		t.Errorf("fg.G = %d, want 64", cell.Fg.G())
	}

	if cell.Fg.B() != 32 {
		t.Errorf("fg.B = %d, want 32", cell.Fg.B())
	}
}

func TestWriteANSIStringSGRBg24bit(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[48;2;10;20;30mX")

	cell := buf.CellAt(0, 0)
	if cell.Bg.R() != 10 {
		t.Errorf("bg.R = %d, want 10", cell.Bg.R())
	}

	if cell.Bg.G() != 20 {
		t.Errorf("bg.G = %d, want 20", cell.Bg.G())
	}

	if cell.Bg.B() != 30 {
		t.Errorf("bg.B = %d, want 30", cell.Bg.B())
	}
}

func TestWriteANSIStringAllAttrs(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[1;2;3;4;5;7;8;9mX")

	cell := buf.CellAt(0, 0)
	if cell.Attrs&AttrBold == 0 {
		t.Error("bold not set")
	}

	if cell.Attrs&AttrDim == 0 {
		t.Error("dim not set")
	}

	if cell.Attrs&AttrItalic == 0 {
		t.Error("italic not set")
	}

	if cell.Attrs&AttrUnderline == 0 {
		t.Error("underline not set")
	}

	if cell.Attrs&AttrBlink == 0 {
		t.Error("blink not set")
	}

	if cell.Attrs&AttrReverse == 0 {
		t.Error("reverse not set")
	}

	if cell.Attrs&AttrHidden == 0 {
		t.Error("hidden not set")
	}

	if cell.Attrs&AttrStrikethrough == 0 {
		t.Error("strikethrough not set")
	}
}

func TestWriteANSIStringAttrOff(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[1;3mX\x1b[22mY\x1b[23mZ")

	if buf.CellAt(0, 0).Attrs&AttrBold == 0 {
		t.Error("X should be bold")
	}

	if buf.CellAt(0, 0).Attrs&AttrItalic == 0 {
		t.Error("X should be italic")
	}

	if buf.CellAt(1, 0).Attrs&AttrBold != 0 {
		t.Error("Y should not be bold (SGR 22 turns off bold+dim)")
	}

	if buf.CellAt(1, 0).Attrs&AttrItalic == 0 {
		t.Error("Y should still be italic")
	}

	if buf.CellAt(2, 0).Attrs&AttrItalic != 0 {
		t.Error("Z should not be italic (SGR 23)")
	}
}

func TestWriteANSIStringFgDefault(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[38;2;255;0;0mR\x1b[39mD")

	if buf.CellAt(0, 0).Fg != NewColor(255, 0, 0) {
		t.Error("R should be red")
	}

	if !buf.CellAt(1, 0).Fg.IsDefault() {
		t.Error("D should have default fg (SGR 39)")
	}
}

func TestWriteANSIStringBgDefault(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[48;2;0;255;0mR\x1b[49mD")

	if buf.CellAt(0, 0).Bg != NewColor(0, 255, 0) {
		t.Error("R should have green bg")
	}

	if !buf.CellAt(1, 0).Bg.IsDefault() {
		t.Error("D should have default bg (SGR 49)")
	}
}

func TestWriteANSIStringZoneMarker(t *testing.T) {
	t.Parallel()

	ResetZones()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, Mark("test-zone", "Hello"))

	cell := buf.CellAt(0, 0)
	if cell.ZoneID == 0 {
		t.Error("cell should have zoneID set by zone marker")
	}

	if !IsZoneAt(buf, 0, 0, "test-zone") {
		t.Error("IsZoneAt should find the zone")
	}
}

func TestWriteANSIStringZoneEndMarker(t *testing.T) {
	t.Parallel()

	ResetZones()

	buf := NewCellBuf(20, 3)
	content := Mark("myzone", "AB")
	buf.WriteANSIString(0, 0, content+"XY")

	// Cells within zone should have ZoneID
	cellA := buf.CellAt(0, 0)

	cellB := buf.CellAt(1, 0)
	if cellA.ZoneID == 0 || cellB.ZoneID == 0 {
		t.Error("cells within zone should have ZoneID")
	}
	// Cells after zone end should not
	cellX := buf.CellAt(2, 0)
	if cellX.ZoneID != 0 {
		t.Error("cells after zone end should have ZoneID=0")
	}
}

func TestWriteANSIStringWrapToNextLine(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)
	buf.WriteANSIString(0, 0, "ABCDEFGH")

	// First 5 chars on line 0, next 3 on line 1
	if buf.CellAt(4, 0).Content != "E" {
		t.Errorf("CellAt(4,0) = %q, want E", buf.CellAt(4, 0).Content)
	}

	if buf.CellAt(0, 1).Content != "F" {
		t.Errorf("CellAt(0,1) = %q, want F (wrapped)", buf.CellAt(0, 1).Content)
	}
}

func TestWriteANSIStringStopsAtBottom(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 2)
	_, endY := buf.WriteANSIString(0, 0, "AAAAABBBBBCCCCC")

	// Content should only appear on rows 0 and 1 (height=2)
	// endY may be 2 (cursor moved past the last row), which is correct
	if buf.CellAt(0, 1).Content != "B" {
		t.Errorf("CellAt(0,1) = %q, want B", buf.CellAt(0, 1).Content)
	}
	// Verify no out-of-bounds write occurred (cell at row 2+ shouldn't exist)
	cell := buf.CellAt(0, 2)
	if cell.Content != " " {
		t.Errorf("CellAt(0,2) = %q, should be empty (out of bounds)", cell.Content)
	}

	_ = endY
}

func TestWriteANSIStringControlChars(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	// Control chars 0x01-0x1F are skipped (except \n, \r, \t)
	// 0x00 is not skipped by the "r != 0" guard (edge case)
	buf.WriteANSIString(0, 0, "A\x01B\x07C")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be written")
	}
	// SOH (0x01) is skipped, so B should be at position 1
	if buf.CellAt(1, 0).Content != "B" {
		t.Errorf("B should be at position 1, got %q", buf.CellAt(1, 0).Content)
	}
	// BEL (0x07) is skipped, so C should be at position 2
	if buf.CellAt(2, 0).Content != "C" {
		t.Errorf("C should be at position 2, got %q", buf.CellAt(2, 0).Content)
	}
}

func TestWriteANSIStringOSC(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	// OSC sequence should be skipped
	buf.WriteANSIString(0, 0, "\x1b]0;title\x07Hello")

	if buf.CellAt(0, 0).Content != "H" {
		t.Errorf("CellAt(0,0) = %q, want H (OSC should be skipped)", buf.CellAt(0, 0).Content)
	}
}

func TestWriteANSIStringOSCTerminatedByST(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b]0;title\x1b\\Hello")

	if buf.CellAt(0, 0).Content != "H" {
		t.Errorf("CellAt(0,0) = %q, want H (OSC with ST terminator should be skipped)", buf.CellAt(0, 0).Content)
	}
}

func TestWriteANSIStringIncompleteEscape(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "A\x1b")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be written")
	}
}

func TestWriteANSIStringWideChar(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	endX, endY := buf.WriteANSIString(0, 0, "A世B")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(1, 0).Content != "世" {
		t.Errorf("世 should be at (1,0), got %q", buf.CellAt(1, 0).Content)
	}

	if buf.CellAt(1, 0).Width != 2 {
		t.Errorf("世 width = %d, want 2", buf.CellAt(1, 0).Width)
	}

	if buf.CellAt(2, 0).Content != "" {
		t.Errorf("continuation cell at (2,0) should be empty, got %q", buf.CellAt(2, 0).Content)
	}

	if buf.CellAt(2, 0).Width != 0 {
		t.Errorf("continuation cell width should be 0, got %d", buf.CellAt(2, 0).Width)
	}

	if buf.CellAt(3, 0).Content != "B" {
		t.Errorf("B should be at (3,0), got %q", buf.CellAt(3, 0).Content)
	}

	if endX != 4 || endY != 0 {
		t.Errorf("endX=%d endY=%d, want 4,0", endX, endY)
	}
}

func TestWriteANSIStringWideCharWrap(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)
	buf.WriteANSIString(0, 0, "A世")

	// A at (0,0), 世 at (1,0) and (2,0) as continuation
	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(1, 0).Content != "世" {
		t.Error("世 should be at (1,0)")
	}
}

func TestWriteStyledText(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	fg := NewColor(0, 128, 255)
	bg := NewColor(40, 40, 40)
	endX, endY := buf.WriteStyledText(0, 0, "Hi", fg, bg, AttrBold, 5)

	if endX != 2 || endY != 0 {
		t.Errorf("endX=%d endY=%d, want 2,0", endX, endY)
	}

	cell := buf.CellAt(0, 0)
	if cell.Fg != fg {
		t.Error("fg mismatch")
	}

	if cell.Bg != bg {
		t.Error("bg mismatch")
	}

	if cell.Attrs != AttrBold {
		t.Errorf("attrs = %d, want AttrBold", cell.Attrs)
	}

	if cell.ZoneID != 5 {
		t.Errorf("zoneID = %d, want 5", cell.ZoneID)
	}
}

func TestWriteStyledTextWithNewlines(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	endX, endY := buf.WriteStyledText(0, 0, "AB\nCD", DefaultColor, DefaultColor, 0, 0)

	if endX != 2 || endY != 1 {
		t.Errorf("endX=%d endY=%d, want 2,1", endX, endY)
	}

	if buf.CellAt(0, 1).Content != "C" {
		t.Error("newline should move to next line")
	}
}

func TestColor16(t *testing.T) {
	t.Parallel()

	c := color16(0)
	if c.ColorType() != colorType16 {
		t.Errorf("color16(0).ColorType() = %d, want colorType16", c.ColorType())
	}

	if c.PaletteIndex() != 0 {
		t.Errorf("color16(0).PaletteIndex() = %d, want 0", c.PaletteIndex())
	}

	c = color16(1)
	if c.PaletteIndex() != 1 {
		t.Errorf("color16(1).PaletteIndex() = %d, want 1", c.PaletteIndex())
	}

	c = color16(15)
	if c.PaletteIndex() != 15 {
		t.Errorf("color16(15).PaletteIndex() = %d, want 15", c.PaletteIndex())
	}
}

func TestColor256Grayscale(t *testing.T) {
	t.Parallel()

	// Grayscale ramp starts at 232
	c := color256(232)
	if c.ColorType() != colorType256 {
		t.Errorf("color256(232) type = %d, want colorType256", c.ColorType())
	}

	if c.PaletteIndex() != 232 {
		t.Errorf("color256(232).PaletteIndex = %d, want 232", c.PaletteIndex())
	}

	c = color256(255)
	if c.PaletteIndex() != 255 {
		t.Errorf("color256(255).PaletteIndex = %d, want 255", c.PaletteIndex())
	}
}

func TestColor256ColorCube(t *testing.T) {
	t.Parallel()

	c := color256(16)
	if c.ColorType() != colorType256 {
		t.Errorf("color256(16) type = %d, want colorType256", c.ColorType())
	}

	if c.PaletteIndex() != 16 {
		t.Errorf("color256(16).PaletteIndex() = %d, want 16", c.PaletteIndex())
	}

	c = color256(196)
	if c.ColorType() != colorType256 {
		t.Errorf("color256(196) type = %d, want colorType256", c.ColorType())
	}

	if c.PaletteIndex() != 196 {
		t.Errorf("color256(196).PaletteIndex() = %d, want 196", c.PaletteIndex())
	}
}

func TestSplitParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  []string
	}{
		{"", []string{""}},
		{"0", []string{"0"}},
		{"1;31", []string{"1", "31"}},
		{"38;2;255;0;0", []string{"38", "2", "255", "0", "0"}},
		{"1;;3", []string{"1", "", "3"}},
	}
	for _, tt := range tests {
		got := splitParams(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitParams(%q) = %v, want %v", tt.input, got, tt.want)

			continue
		}

		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitParams(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestDecodeRune(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		pos   int
		r     rune
		size  int
	}{
		{"A", 0, 'A', 1},
		{"\xc3\xa9", 0, 0xe9, 2},            // é
		{"\xe4\xb8\x96", 0, 0x4e16, 3},      // 世
		{"\xf0\x9f\x93\xa6", 0, 0x1f4e6, 4}, // 📦
	}
	for _, tt := range tests {
		r, size := decodeRune(tt.input, tt.pos)
		if r != tt.r || size != tt.size {
			t.Errorf("decodeRune(%q, %d) = (%U, %d), want (%U, %d)", tt.input, tt.pos, r, size, tt.r, tt.size)
		}
	}
}

func TestDecodeRuneOutOfBounds(t *testing.T) {
	t.Parallel()

	r, size := decodeRune("AB", 5)
	if r != 0 || size != 0 {
		t.Errorf("out of bounds: got (%U, %d), want (0, 0)", r, size)
	}
}

func TestWriteANSIStringStartingOffset(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	endX, endY := buf.WriteANSIString(5, 1, "Hi")

	if endX != 7 || endY != 1 {
		t.Errorf("endX=%d endY=%d, want 7,1", endX, endY)
	}

	if buf.CellAt(5, 1).Content != "H" {
		t.Error("H should be at (5,1)")
	}

	if buf.CellAt(6, 1).Content != "i" {
		t.Error("i should be at (6,1)")
	}
}

func TestColor16OutOfRange(t *testing.T) {
	t.Parallel()

	c := color16(16)
	if !c.IsDefault() {
		t.Errorf("color16(16) should return DefaultColor for out-of-range, got %v", c)
	}
}

func TestParseColorMissingSubtype(t *testing.T) {
	t.Parallel()

	p := &ansiParser{fg: DefaultColor, bg: DefaultColor}
	parts := []string{"38", "9"}

	c, adv := p.parseColor(parts, 0)
	if adv != 0 {
		t.Errorf("parseColor with unknown subtype should return adv=0, got %d", adv)
	}

	if c != DefaultColor {
		t.Error("parseColor with unknown subtype should return DefaultColor")
	}
}

func TestParseColorInsufficientRGBParts(t *testing.T) {
	t.Parallel()

	p := &ansiParser{fg: DefaultColor, bg: DefaultColor}
	parts := []string{"38", "2", "255", "0"}

	c, adv := p.parseColor(parts, 0)
	if adv != 0 {
		t.Errorf("parseColor with insufficient RGB parts should return adv=0, got %d", adv)
	}

	if c != DefaultColor {
		t.Error("parseColor with insufficient RGB parts should return DefaultColor")
	}
}

func TestParseColorInsufficient256Parts(t *testing.T) {
	t.Parallel()

	p := &ansiParser{fg: DefaultColor, bg: DefaultColor}
	parts := []string{"48", "5"}

	c, adv := p.parseColor(parts, 0)
	if adv != 0 {
		t.Errorf("parseColor with insufficient 256 parts should return adv=0, got %d", adv)
	}

	if c != DefaultColor {
		t.Error("parseColor with insufficient 256 parts should return DefaultColor")
	}
}

func TestParseColorNoSubtype(t *testing.T) {
	t.Parallel()

	p := &ansiParser{fg: DefaultColor, bg: DefaultColor}
	parts := []string{"38"}

	c, adv := p.parseColor(parts, 0)
	if adv != 0 {
		t.Errorf("parseColor with no subtype should return adv=0, got %d", adv)
	}

	if c != DefaultColor {
		t.Error("parseColor with no subtype should return DefaultColor")
	}
}

func TestSkipOSCUnterminated(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b]0;no terminator here")

	if buf.CellAt(0, 0).Content != " " {
		t.Errorf("unterminated OSC should be skipped, got %q", buf.CellAt(0, 0).Content)
	}
}

func TestParseEscapeNonCSI(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "A\x1b?X")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(1, 0).Content != "?" {
		t.Errorf("ESC + ? (0x3F) should print ? as next char, got %q", buf.CellAt(1, 0).Content)
	}

	if buf.CellAt(2, 0).Content != "X" {
		t.Errorf("X should be at (2,0), got %q", buf.CellAt(2, 0).Content)
	}
}

func TestParseEscapeSingleESCAtEnd(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "A\x1b")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}
}

func TestParseEscapeTwoByteSequence(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "A\x1b\\B")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(1, 0).Content != "B" {
		t.Errorf("B should be at (1,0) after ESC + 0x5C, got %q", buf.CellAt(1, 0).Content)
	}
}

func TestWriteStyledTextWideChar(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	fg := NewColor(255, 0, 0)
	endX, _ := buf.WriteStyledText(0, 0, "A世B", fg, DefaultColor, 0, 0)

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(1, 0).Content != "世" {
		t.Error("世 should be at (1,0)")
	}

	if buf.CellAt(1, 0).Width != 2 {
		t.Error("世 should have width 2")
	}

	if buf.CellAt(2, 0).Content != "" {
		t.Error("continuation cell should be empty")
	}

	if buf.CellAt(3, 0).Content != "B" {
		t.Error("B should be at (3,0)")
	}

	if buf.CellAt(1, 0).Fg != fg {
		t.Error("styled text should have correct fg")
	}

	if endX != 4 {
		t.Errorf("endX = %d, want 4", endX)
	}
}

func TestWriteStyledTextOverflowWidth(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(3, 3)
	endX, _ := buf.WriteStyledText(0, 0, "ABCDE", DefaultColor, DefaultColor, 0, 0)

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(2, 0).Content != "C" {
		t.Error("C should be at (2,0)")
	}

	if endX != 5 {
		t.Errorf("endX = %d, want 5 (cursor advances past buffer)", endX)
	}
}

func TestWriteStyledTextStopsAtBottom(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(3, 1)
	_, endY := buf.WriteStyledText(0, 0, "ABC\nDEF", DefaultColor, DefaultColor, 0, 0)

	if endY > 1 {
		t.Errorf("endY=%d, should not go past buffer height", endY)
	}
}

func TestLineVersionOutOfBounds(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(5, 3)
	if buf.LineVersion(-1) != 0 {
		t.Error("LineVersion(-1) should return 0")
	}

	if buf.LineVersion(3) != 0 {
		t.Error("LineVersion(3) should return 0 for out-of-bounds")
	}
}

func TestParseCSIUnknownFinalByte(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[1qHello")

	if buf.CellAt(0, 0).Content != "H" {
		t.Errorf("unknown CSI final byte should skip sequence, got %q", buf.CellAt(0, 0).Content)
	}
}

func TestColor256Range(t *testing.T) {
	t.Parallel()

	c := color256(16)
	if c.ColorType() != colorType256 {
		t.Errorf("color256(16) type = %d, want colorType256", c.ColorType())
	}

	c = color256(232)
	if c.ColorType() != colorType256 {
		t.Errorf("color256(232) type = %d, want colorType256 (grayscale palette index)", c.ColorType())
	}

	if c.PaletteIndex() != 232 {
		t.Errorf("color256(232).PaletteIndex = %d, want 232", c.PaletteIndex())
	}
}

func TestRuneDisplayWidth(t *testing.T) {
	t.Parallel()

	if runeDisplayWidth('A') != 1 {
		t.Error("ASCII char should have width 1")
	}

	if runeDisplayWidth('世') < 1 {
		t.Error("CJK char should have width >= 1")
	}
}

func TestWriteANSIStringAttrBlinkOff(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[5mX\x1b[25mY")

	if buf.CellAt(0, 0).Attrs&AttrBlink == 0 {
		t.Error("X should be blinking")
	}

	if buf.CellAt(1, 0).Attrs&AttrBlink != 0 {
		t.Error("Y should not be blinking (SGR 25)")
	}
}

func TestWriteANSIStringAttrReverseOff(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[7mX\x1b[27mY")

	if buf.CellAt(0, 0).Attrs&AttrReverse == 0 {
		t.Error("X should be reversed")
	}

	if buf.CellAt(1, 0).Attrs&AttrReverse != 0 {
		t.Error("Y should not be reversed (SGR 27)")
	}
}

func TestWriteANSIStringAttrHiddenOff(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[8mX\x1b[28mY")

	if buf.CellAt(0, 0).Attrs&AttrHidden == 0 {
		t.Error("X should be hidden")
	}

	if buf.CellAt(1, 0).Attrs&AttrHidden != 0 {
		t.Error("Y should not be hidden (SGR 28)")
	}
}

func TestWriteANSIStringAttrStrikethroughOff(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[9mX\x1b[29mY")

	if buf.CellAt(0, 0).Attrs&AttrStrikethrough == 0 {
		t.Error("X should be strikethrough")
	}

	if buf.CellAt(1, 0).Attrs&AttrStrikethrough != 0 {
		t.Error("Y should not be strikethrough (SGR 29)")
	}
}

func TestWriteStyledTextCarriageReturn(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	endX, _ := buf.WriteStyledText(0, 0, "AB\rX", DefaultColor, DefaultColor, 0, 0)

	if buf.CellAt(0, 0).Content != "X" {
		t.Errorf("CR should overwrite first char, got %q", buf.CellAt(0, 0).Content)
	}

	if buf.CellAt(1, 0).Content != "B" {
		t.Error("B should still be at (1,0)")
	}

	if endX != 1 {
		t.Errorf("endX = %d, want 1", endX)
	}
}

func TestParseCSITruncatedAtEnd(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "A\x1b[1;2")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}
}

func TestApplySGRDimOff(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[1;2mXY\x1b[22mZ")

	if buf.CellAt(0, 0).Attrs&AttrBold == 0 {
		t.Error("X should be bold")
	}

	if buf.CellAt(0, 0).Attrs&AttrDim == 0 {
		t.Error("X should be dim")
	}

	if buf.CellAt(2, 0).Attrs&AttrBold != 0 {
		t.Error("Z should not be bold after SGR 22")
	}

	if buf.CellAt(2, 0).Attrs&AttrDim != 0 {
		t.Error("Z should not be dim after SGR 22")
	}
}

func TestApplySGRAllAttrOffCodes(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "\x1b[5mX\x1b[25mY")

	if buf.CellAt(0, 0).Attrs&AttrBlink == 0 {
		t.Error("X should blink")
	}

	if buf.CellAt(1, 0).Attrs&AttrBlink != 0 {
		t.Error("Y should not blink after SGR 25")
	}

	buf2 := NewCellBuf(20, 3)
	buf2.WriteANSIString(0, 0, "\x1b[7mX\x1b[27mY")

	if buf2.CellAt(0, 0).Attrs&AttrReverse == 0 {
		t.Error("X should be reversed")
	}

	if buf2.CellAt(1, 0).Attrs&AttrReverse != 0 {
		t.Error("Y should not be reversed after SGR 27")
	}

	buf3 := NewCellBuf(20, 3)
	buf3.WriteANSIString(0, 0, "\x1b[8mX\x1b[28mY")

	if buf3.CellAt(0, 0).Attrs&AttrHidden == 0 {
		t.Error("X should be hidden")
	}

	if buf3.CellAt(1, 0).Attrs&AttrHidden != 0 {
		t.Error("Y should not be hidden after SGR 28")
	}

	buf4 := NewCellBuf(20, 3)
	buf4.WriteANSIString(0, 0, "\x1b[9mX\x1b[29mY")

	if buf4.CellAt(0, 0).Attrs&AttrStrikethrough == 0 {
		t.Error("X should be strikethrough")
	}

	if buf4.CellAt(1, 0).Attrs&AttrStrikethrough != 0 {
		t.Error("Y should not be strikethrough after SGR 29")
	}
}

func TestColor256FullCubeRange(t *testing.T) {
	t.Parallel()

	c := color256(231)
	if c.ColorType() != colorType256 {
		t.Errorf("color256(231) type = %d, want colorType256", c.ColorType())
	}

	if c.PaletteIndex() != 231 {
		t.Errorf("color256(231).PaletteIndex() = %d, want 231", c.PaletteIndex())
	}
}

func TestColor256MidCube(t *testing.T) {
	t.Parallel()

	c := color256(196)
	if c.ColorType() != colorType256 {
		t.Errorf("color256(196) type = %d, want colorType256", c.ColorType())
	}

	if c.PaletteIndex() != 196 {
		t.Errorf("color256(196).PaletteIndex() = %d, want 196", c.PaletteIndex())
	}
}

func TestRuneDisplayWidthNegative(t *testing.T) {
	t.Parallel()

	w := runeDisplayWidth('\x00')
	if w < 0 {
		t.Errorf("runeDisplayWidth should not return negative for NUL, got %d", w)
	}
}

func TestWriteANSIStringEmojiSkinTone(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	// 👍🏽 = U+1F44D (thumbs up) + U+1F3FD (medium skin tone)
	// This is a single grapheme cluster, width 2.
	buf.WriteANSIString(0, 0, "👍🏽Hi")

	cell0 := buf.CellAt(0, 0)
	if cell0.Width != 2 {
		t.Errorf("emoji+skin tone cluster width = %d, want 2", cell0.Width)
	}
	// Continuation cell for the wide emoji
	cell1 := buf.CellAt(1, 0)
	if cell1.Width != 0 {
		t.Errorf("continuation cell width = %d, want 0", cell1.Width)
	}
	// "H" should start at column 2, not column 4
	cell2 := buf.CellAt(2, 0)
	if cell2.Content != "H" {
		t.Errorf("cell after emoji+skin tone = %q, want 'H'", cell2.Content)
	}
}

func TestWriteANSIStringEmojiZWJ(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	// 👨‍👩‍👧 = man + ZWJ + woman + ZWJ + girl — single grapheme, width 2
	buf.WriteANSIString(0, 0, "👨‍👩‍👧X")

	cell0 := buf.CellAt(0, 0)
	if cell0.Width != 2 {
		t.Errorf("ZWJ family emoji width = %d, want 2", cell0.Width)
	}

	cell2 := buf.CellAt(2, 0)
	if cell2.Content != "X" {
		t.Errorf("cell after ZWJ emoji = %q, want 'X'", cell2.Content)
	}
}

func TestWriteANSIStringEmojiVariationSelector(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	// ❤️ = U+2764 + U+FE0F (variation selector-16) — single grapheme, width 2
	buf.WriteANSIString(0, 0, "❤️ab")

	cell0 := buf.CellAt(0, 0)
	if cell0.Width != 2 {
		t.Errorf("heart+VS16 width = %d, want 2", cell0.Width)
	}

	cell2 := buf.CellAt(2, 0)
	if cell2.Content != "a" {
		t.Errorf("cell after heart+VS16 = %q, want 'a'", cell2.Content)
	}
}

func TestWriteStyledTextEmojiSkinTone(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(20, 3)
	endX, _ := buf.WriteStyledText(0, 0, "👍🏽Hi", DefaultColor, DefaultColor, 0, 0)

	cell0 := buf.CellAt(0, 0)
	if cell0.Width != 2 {
		t.Errorf("styled emoji+skin tone width = %d, want 2", cell0.Width)
	}

	cell2 := buf.CellAt(2, 0)
	if cell2.Content != "H" {
		t.Errorf("cell after styled emoji = %q, want 'H'", cell2.Content)
	}

	if endX != 4 {
		t.Errorf("endX = %d, want 4", endX)
	}
}

func TestWriteANSIStringPadsOnNewline(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)

	for x := range 10 {
		buf.SetCell(x, 0, Cell{Content: "X", Width: 1, Fg: NewColor(255, 0, 0)})
	}

	buf.WriteANSIString(0, 0, "AB\nCD")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(1, 0).Content != "B" {
		t.Error("B should be at (1,0)")
	}

	for x := 2; x < 10; x++ {
		cell := buf.CellAt(x, 0)
		if !cell.VisualEqual(EmptyCell) {
			t.Errorf("CellAt(%d,0) should be EmptyCell after \\n pads line, got Content=%q Fg=%v", x, cell.Content, cell.Fg)
		}
	}

	if buf.CellAt(0, 1).Content != "C" {
		t.Error("C should be at (0,1)")
	}
}

func TestWriteANSIStringCarriageReturnDoesNotPad(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)

	for x := range 10 {
		buf.SetCell(x, 0, Cell{Content: "X", Width: 1, Fg: NewColor(255, 0, 0)})
	}

	buf.WriteANSIString(0, 0, "AB\rXY")

	if buf.CellAt(0, 0).Content != "X" {
		t.Errorf("CellAt(0,0) = %q, want X (overwritten by \\r)", buf.CellAt(0, 0).Content)
	}

	if buf.CellAt(1, 0).Content != "Y" {
		t.Errorf("CellAt(1,0) = %q, want Y (overwritten by \\r)", buf.CellAt(1, 0).Content)
	}

	// After "AB\rXY", end-of-string padLineToWidth runs from curX=2 to width,
	// clearing columns 2-9 to EmptyCell. The key property being tested: \r
	// itself does NOT call padLineToWidth, so between \r and the end-of-string
	// pad, the old content at columns 2+ is still intact. The final state
	// shows EmptyCell because end-of-string pad clears it — NOT because \r did.
	for x := 2; x < 10; x++ {
		cell := buf.CellAt(x, 0)
		if !cell.VisualEqual(EmptyCell) {
			t.Errorf("CellAt(%d,0) should be EmptyCell after end-of-string padLineToWidth, got Content=%q", x, cell.Content)
		}
	}
}

func TestClearLinesBelow(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 5)

	buf.SetCell(0, 0, Cell{Content: "A", Width: 1})
	buf.SetCell(0, 1, Cell{Content: "B", Width: 1, Fg: NewColor(255, 0, 0)})
	buf.SetCell(0, 2, Cell{Content: "C", Width: 1, Attrs: AttrBold})
	buf.SetCell(0, 3, Cell{Content: "D", Width: 1})
	buf.SetCell(0, 4, Cell{Content: "E", Width: 1})

	buf.ClearLinesBelow(2)

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("row 0 should be untouched")
	}

	if buf.CellAt(0, 1).Content != "B" {
		t.Error("row 1 should be untouched")
	}

	if !buf.CellAt(0, 2).VisualEqual(EmptyCell) {
		t.Errorf("row 2 should be cleared, got Content=%q", buf.CellAt(0, 2).Content)
	}

	if !buf.CellAt(0, 3).VisualEqual(EmptyCell) {
		t.Errorf("row 3 should be cleared, got Content=%q", buf.CellAt(0, 3).Content)
	}

	if !buf.CellAt(0, 4).VisualEqual(EmptyCell) {
		t.Errorf("row 4 should be cleared, got Content=%q", buf.CellAt(0, 4).Content)
	}
}

func TestPadLineToWidthRange(t *testing.T) {
	t.Parallel()

	buf := NewCellBuf(10, 3)

	for x := range 10 {
		buf.SetCell(x, 0, Cell{Content: "X", Width: 1, Fg: NewColor(255, 0, 0)})
	}

	buf.WriteANSIString(0, 0, "ABC")

	if buf.CellAt(0, 0).Content != "A" {
		t.Error("A should be at (0,0)")
	}

	if buf.CellAt(1, 0).Content != "B" {
		t.Error("B should be at (1,0)")
	}

	if buf.CellAt(2, 0).Content != "C" {
		t.Error("C should be at (2,0)")
	}

	for x := 3; x < 10; x++ {
		cell := buf.CellAt(x, 0)
		if !cell.VisualEqual(EmptyCell) {
			t.Errorf("CellAt(%d,0) should be EmptyCell (padded by padLineToWidth from curX=3), got Content=%q Fg=%v", x, cell.Content, cell.Fg)
		}
	}
}
