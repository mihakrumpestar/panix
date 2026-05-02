package render

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type Style struct {
	Fg    Color
	Bg    Color
	Attrs Attr
}

var DefaultStyle = Style{}

func NewStyleFromLipgloss(s lipgloss.Style) Style {
	st := Style{}

	fg := s.GetForeground()
	if fg != nil {
		if _, isNoColor := fg.(lipgloss.NoColor); !isNoColor {
			st.Fg = colorFromLipgloss(fg)
		}
	}

	bg := s.GetBackground()
	if bg != nil {
		if _, isNoColor := bg.(lipgloss.NoColor); !isNoColor {
			st.Bg = colorFromLipgloss(bg)
		}
	}

	if s.GetBold() {
		st.Attrs |= AttrBold
	}

	if s.GetItalic() {
		st.Attrs |= AttrItalic
	}

	if s.GetUnderline() {
		st.Attrs |= AttrUnderline
	}

	if s.GetBlink() {
		st.Attrs |= AttrBlink
	}

	if s.GetReverse() {
		st.Attrs |= AttrReverse
	}

	return st
}

func colorFromLipgloss(c color.Color) Color {
	switch v := c.(type) {
	case ansi.BasicColor:
		return NewColor16(uint8(v))
	case ansi.ExtendedColor:
		idx := uint8(v)
		if idx < 16 {
			return NewColor16(idx)
		}

		return NewColor256(idx)
	default:
		r, g, b, a := c.RGBA()

		return ColorFromRGBA(r, g, b, a)
	}
}

func StyleFromColor(c color.Color) Style {
	st := Style{}

	if c != nil {
		r, g, b, a := c.RGBA()
		st.Fg = ColorFromRGBA(r, g, b, a)
	}

	return st
}

func FgStyle(c Color) Style {
	return Style{Fg: c}
}

func (s Style) WithFg(c Color) Style {
	s.Fg = c

	return s
}
func (s Style) WithBg(c Color) Style {
	s.Bg = c

	return s
}
func (s Style) WithBold(on bool) Style {
	if on {
		s.Attrs |= AttrBold
	} else {
		s.Attrs &^= AttrBold
	}

	return s
}
func (s Style) WithItalic(on bool) Style {
	if on {
		s.Attrs |= AttrItalic
	} else {
		s.Attrs &^= AttrItalic
	}

	return s
}

func (s Style) IsDefault() bool {
	return s.Fg.IsDefault() && s.Bg.IsDefault() && s.Attrs == 0
}
