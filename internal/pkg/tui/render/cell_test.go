package render

import (
	"testing"
)

func TestNewColor(t *testing.T) {
	t.Parallel()

	c := NewColor(255, 128, 64)
	if c.R() != 255 {
		t.Errorf("R() = %d, want 255", c.R())
	}

	if c.G() != 128 {
		t.Errorf("G() = %d, want 128", c.G())
	}

	if c.B() != 64 {
		t.Errorf("B() = %d, want 64", c.B())
	}

	if c.ColorType() != colorTypeTrue {
		t.Errorf("ColorType() = %d, want %d", c.ColorType(), colorTypeTrue)
	}
}

func TestNewColorZero(t *testing.T) {
	t.Parallel()

	c := NewColor(0, 0, 0)
	if c.R() != 0 || c.G() != 0 || c.B() != 0 {
		t.Errorf("NewColor(0,0,0) components: R=%d G=%d B=%d", c.R(), c.G(), c.B())
	}

	if !c.IsRGB() {
		t.Error("NewColor(0,0,0) should be RGB")
	}
}

func TestColorFromRGBA(t *testing.T) {
	t.Parallel()

	// RGBA values from color.Color.RGBA() are 16-bit (0-65535)
	c := ColorFromRGBA(0xFFFF, 0x8000, 0x0000, 0xFFFF)
	if c.R() != 255 {
		t.Errorf("R() = %d, want 255", c.R())
	}

	if c.G() != 128 {
		t.Errorf("G() = %d, want 128", c.G())
	}

	if c.B() != 0 {
		t.Errorf("B() = %d, want 0", c.B())
	}
}

func TestDefaultColor(t *testing.T) {
	t.Parallel()

	if !DefaultColor.IsDefault() {
		t.Error("DefaultColor should be default")
	}

	if DefaultColor.IsRGB() {
		t.Error("DefaultColor should not be RGB")
	}

	if DefaultColor.R() != 0 || DefaultColor.G() != 0 || DefaultColor.B() != 0 {
		t.Errorf("DefaultColor components should be 0: R=%d G=%d B=%d", DefaultColor.R(), DefaultColor.G(), DefaultColor.B())
	}
}

func TestColorIsDefaultAndIsRGB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		color     Color
		isDefault bool
		isRGB     bool
	}{
		{"zero value", 0, true, false},
		{"NewColor", NewColor(1, 1, 1), false, true},
		{"only alpha set", Color(0xFF), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.color.IsDefault() != tt.isDefault {
				t.Errorf("IsDefault() = %v, want %v", tt.color.IsDefault(), tt.isDefault)
			}

			if tt.color.IsRGB() != tt.isRGB {
				t.Errorf("IsRGB() = %v, want %v", tt.color.IsRGB(), tt.isRGB)
			}
		})
	}
}

func TestAttrBitmask(t *testing.T) {
	t.Parallel()

	var a Attr

	a |= AttrBold
	if a&AttrBold == 0 {
		t.Error("AttrBold should be set")
	}

	a |= AttrItalic
	if a&AttrItalic == 0 {
		t.Error("AttrItalic should be set")
	}

	a &^= AttrBold
	if a&AttrBold != 0 {
		t.Error("AttrBold should be cleared")
	}

	if a&AttrItalic == 0 {
		t.Error("AttrItalic should still be set")
	}
}

func TestAttrAllBits(t *testing.T) {
	t.Parallel()

	var all = AttrBold | AttrDim | AttrItalic | AttrUnderline | AttrBlink | AttrReverse | AttrStrikethrough | AttrHidden
	if all&AttrBold == 0 {
		t.Error("AttrBold not in all")
	}

	if all&AttrDim == 0 {
		t.Error("AttrDim not in all")
	}

	if all&AttrItalic == 0 {
		t.Error("AttrItalic not in all")
	}

	if all&AttrUnderline == 0 {
		t.Error("AttrUnderline not in all")
	}

	if all&AttrBlink == 0 {
		t.Error("AttrBlink not in all")
	}

	if all&AttrReverse == 0 {
		t.Error("AttrReverse not in all")
	}

	if all&AttrStrikethrough == 0 {
		t.Error("AttrStrikethrough not in all")
	}

	if all&AttrHidden == 0 {
		t.Error("AttrHidden not in all")
	}
}

func TestVisualEqualIdentical(t *testing.T) {
	t.Parallel()

	c1 := Cell{Content: "A", Width: 1, Fg: NewColor(255, 0, 0), Bg: DefaultColor, Attrs: AttrBold}

	c2 := Cell{Content: "A", Width: 1, Fg: NewColor(255, 0, 0), Bg: DefaultColor, Attrs: AttrBold}
	if !c1.VisualEqual(c2) {
		t.Error("identical cells should be visually equal")
	}
}

func TestVisualEqualDifferentContent(t *testing.T) {
	t.Parallel()

	c1 := Cell{Content: "A", Width: 1, Fg: NewColor(255, 0, 0), Bg: DefaultColor}

	c2 := Cell{Content: "B", Width: 1, Fg: NewColor(255, 0, 0), Bg: DefaultColor}
	if c1.VisualEqual(c2) {
		t.Error("different content should not be equal")
	}
}

func TestVisualEqualDifferentFg(t *testing.T) {
	t.Parallel()

	c1 := Cell{Content: "A", Width: 1, Fg: NewColor(255, 0, 0)}

	c2 := Cell{Content: "A", Width: 1, Fg: NewColor(0, 255, 0)}
	if c1.VisualEqual(c2) {
		t.Error("different fg should not be equal")
	}
}

func TestVisualEqualDifferentBg(t *testing.T) {
	t.Parallel()

	c1 := Cell{Content: "A", Width: 1, Bg: NewColor(255, 0, 0)}

	c2 := Cell{Content: "A", Width: 1, Bg: NewColor(0, 255, 0)}
	if c1.VisualEqual(c2) {
		t.Error("different bg should not be equal")
	}
}

func TestVisualEqualDifferentAttrs(t *testing.T) {
	t.Parallel()

	c1 := Cell{Content: "A", Width: 1, Attrs: AttrBold}

	c2 := Cell{Content: "A", Width: 1, Attrs: AttrItalic}
	if c1.VisualEqual(c2) {
		t.Error("different attrs should not be equal")
	}
}

func TestVisualEqualDifferentWidth(t *testing.T) {
	t.Parallel()

	c1 := Cell{Content: "世", Width: 2}

	c2 := Cell{Content: "世", Width: 1}
	if c1.VisualEqual(c2) {
		t.Error("different width should not be equal")
	}
}

func TestVisualEqualIgnoresZoneID(t *testing.T) {
	t.Parallel()

	c1 := Cell{Content: "A", Width: 1, ZoneID: 5}

	c2 := Cell{Content: "A", Width: 1, ZoneID: 10}
	if !c1.VisualEqual(c2) {
		t.Error("VisualEqual should ignore ZoneID")
	}
}

func TestEmptyCell(t *testing.T) {
	t.Parallel()

	if EmptyCell.Content != " " {
		t.Errorf("EmptyCell.Content = %q, want %q", EmptyCell.Content, " ")
	}

	if EmptyCell.Width != 1 {
		t.Errorf("EmptyCell.Width = %d, want 1", EmptyCell.Width)
	}

	if EmptyCell.Fg != DefaultColor {
		t.Errorf("EmptyCell.Fg should be DefaultColor")
	}

	if EmptyCell.Bg != DefaultColor {
		t.Errorf("EmptyCell.Bg should be DefaultColor")
	}

	if EmptyCell.Attrs != 0 {
		t.Errorf("EmptyCell.Attrs = %d, want 0", EmptyCell.Attrs)
	}

	if EmptyCell.ZoneID != 0 {
		t.Errorf("EmptyCell.ZoneID = %d, want 0", EmptyCell.ZoneID)
	}
}
