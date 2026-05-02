package render

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestDefaultStyle(t *testing.T) {
	t.Parallel()

	if !DefaultStyle.IsDefault() {
		t.Error("DefaultStyle should be default")
	}

	if DefaultStyle.Fg != DefaultColor {
		t.Error("DefaultStyle.Fg should be DefaultColor")
	}

	if DefaultStyle.Bg != DefaultColor {
		t.Error("DefaultStyle.Bg should be DefaultColor")
	}

	if DefaultStyle.Attrs != 0 {
		t.Error("DefaultStyle.Attrs should be 0")
	}
}

func TestStyleIsDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		style Style
		want  bool
	}{
		{"default", Style{}, true},
		{"with fg", Style{Fg: NewColor(255, 0, 0)}, false},
		{"with bg", Style{Bg: NewColor(0, 255, 0)}, false},
		{"with attrs", Style{Attrs: AttrBold}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.style.IsDefault() != tt.want {
				t.Errorf("IsDefault() = %v, want %v", tt.style.IsDefault(), tt.want)
			}
		})
	}
}

func TestFgStyle(t *testing.T) {
	t.Parallel()

	c := NewColor(255, 0, 0)

	s := FgStyle(c)
	if s.Fg != c {
		t.Error("FgStyle should set fg")
	}

	if s.Bg != DefaultColor {
		t.Error("FgStyle should not set bg")
	}
}

func TestStyleWithFg(t *testing.T) {
	t.Parallel()

	s := Style{}

	s2 := s.WithFg(NewColor(255, 0, 0))
	if s2.Fg != NewColor(255, 0, 0) {
		t.Error("WithFg should set fg")
	}

	if s.Fg != DefaultColor {
		t.Error("original should be unchanged")
	}
}

func TestStyleWithBg(t *testing.T) {
	t.Parallel()

	s := Style{}

	s2 := s.WithBg(NewColor(0, 255, 0))
	if s2.Bg != NewColor(0, 255, 0) {
		t.Error("WithBg should set bg")
	}
}

func TestStyleWithBold(t *testing.T) {
	t.Parallel()

	s := Style{}

	sOn := s.WithBold(true)
	if sOn.Attrs&AttrBold == 0 {
		t.Error("WithBold(true) should set bold")
	}

	sOff := sOn.WithBold(false)
	if sOff.Attrs&AttrBold != 0 {
		t.Error("WithBold(false) should clear bold")
	}
}

func TestStyleWithItalic(t *testing.T) {
	t.Parallel()

	s := Style{}

	sOn := s.WithItalic(true)
	if sOn.Attrs&AttrItalic == 0 {
		t.Error("WithItalic(true) should set italic")
	}

	sOff := sOn.WithItalic(false)
	if sOff.Attrs&AttrItalic != 0 {
		t.Error("WithItalic(false) should clear italic")
	}
}

func TestNewStyleFromLipglossFg(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	s := NewStyleFromLipgloss(lg)

	if s.Fg.IsDefault() {
		t.Error("fg should not be default after lipgloss conversion")
	}

	if s.Fg.R() != 255 {
		t.Errorf("fg.R = %d, want 255", s.Fg.R())
	}
}

func TestNewStyleFromLipglossBg(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle().Background(lipgloss.Color("#00FF00"))
	s := NewStyleFromLipgloss(lg)

	if s.Bg.IsDefault() {
		t.Error("bg should not be default after lipgloss conversion")
	}

	if s.Bg.G() != 255 {
		t.Errorf("bg.G = %d, want 255", s.Bg.G())
	}
}

func TestNewStyleFromLipglossBold(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle().Bold(true)
	s := NewStyleFromLipgloss(lg)

	if s.Attrs&AttrBold == 0 {
		t.Error("bold should be set from lipgloss Bold(true)")
	}
}

func TestNewStyleFromLipglossItalic(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle().Italic(true)
	s := NewStyleFromLipgloss(lg)

	if s.Attrs&AttrItalic == 0 {
		t.Error("italic should be set from lipgloss Italic(true)")
	}
}

func TestNewStyleFromLipglossUnderline(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle().Underline(true)
	s := NewStyleFromLipgloss(lg)

	if s.Attrs&AttrUnderline == 0 {
		t.Error("underline should be set from lipgloss Underline(true)")
	}
}

func TestNewStyleFromLipglossBlink(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle().Blink(true)
	s := NewStyleFromLipgloss(lg)

	if s.Attrs&AttrBlink == 0 {
		t.Error("blink should be set from lipgloss Blink(true)")
	}
}

func TestNewStyleFromLipglossReverse(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle().Reverse(true)
	s := NewStyleFromLipgloss(lg)

	if s.Attrs&AttrReverse == 0 {
		t.Error("reverse should be set from lipgloss Reverse(true)")
	}
}

func TestNewStyleFromLipglossEmpty(t *testing.T) {
	t.Parallel()

	lg := lipgloss.NewStyle()
	s := NewStyleFromLipgloss(lg)

	if s.Fg != DefaultColor {
		t.Error("empty lipgloss style fg should be DefaultColor")
	}

	if s.Bg != DefaultColor {
		t.Error("empty lipgloss style bg should be DefaultColor")
	}

	if s.Attrs != 0 {
		t.Error("empty lipgloss style attrs should be 0")
	}
}

func TestStyleFromColor(t *testing.T) {
	t.Parallel()

	c := color.NRGBA{R: 128, G: 64, B: 32, A: 255}
	s := StyleFromColor(c)

	if s.Fg.IsDefault() {
		t.Error("fg should not be default from color")
	}

	if s.Bg != DefaultColor {
		t.Error("bg should be default (StyleFromColor only sets fg)")
	}
}

func TestStyleFromColorNil(t *testing.T) {
	t.Parallel()

	s := StyleFromColor(nil)
	if !s.IsDefault() {
		t.Error("nil color should produce default style")
	}
}
