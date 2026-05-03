package render

import (
	"errors"
	"io"
	"strconv"
)

const (
	clearLineRight = "\x1b[K"
	sgrReset       = "\x1b[0m"
	sgrFgDefault   = "\x1b[39m"
	sgrBgDefault   = "\x1b[49m"
	sgrBoldOn      = "\x1b[1m"
	sgrBoldOff     = "\x1b[22m"
	sgrDimOn       = "\x1b[2m"
	sgrDimOff      = "\x1b[22m"
	sgrItalicOn    = "\x1b[3m"
	sgrItalicOff   = "\x1b[23m"
	sgrUnderlineOn = "\x1b[4m"
	sgrUnderlineOff = "\x1b[24m"
	sgrBlinkOn     = "\x1b[5m"
	sgrBlinkOff    = "\x1b[25m"
	sgrReverseOn   = "\x1b[7m"
	sgrReverseOff  = "\x1b[27m"
	sgrHiddenOn    = "\x1b[8m"
	sgrHiddenOff   = "\x1b[28m"
	sgrStrikeOn    = "\x1b[9m"
	sgrStrikeOff   = "\x1b[29m"
)

type Writer struct {
	w       io.Writer
	buf     []byte
	curFg   Color
	curBg   Color
	curAttr Attr
	curX    int
	curY    int
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:   w,
		buf: make([]byte, 0, 4096),
	}
}

func (w *Writer) Flush() error {
	if len(w.buf) == 0 {
		return nil
	}

	n, err := w.w.Write(w.buf)
	w.buf = w.buf[:0]

	if err != nil {
		return err
	}

	if n == 0 {
		return errors.New("write returned 0 bytes")
	}

	return nil
}

func (w *Writer) WriteDiff(diffs []LineDiff, buf *CellBuf) {
	width := buf.Width()
	prevY := -2

	for i, d := range diffs {
		// Sequential line optimization: after writing a full line the
		// cursor is at (width, prevY). \r moves to column 0, \n moves
		// down one line → (0, prevY+1). This replaces a 6-10 byte
		// moveCursor sequence with just 2 bytes.
		if i > 0 && d.Y == prevY+1 && w.curX == width {
			w.buf = append(w.buf, '\r', '\n')
			w.curX = 0
			w.curY = d.Y
		} else {
			w.moveCursor(0, d.Y)
		}

		line := buf.Line(d.Y)
		if line == nil {
			prevY = d.Y
			continue
		}

		for x := range width {
			cell := line[x]
			if cell.Width == 0 {
				continue
			}

			w.setStyle(cell.Fg, cell.Bg, cell.Attrs)

			if cell.Content == "" || cell.Content == " " {
				w.buf = append(w.buf, ' ')
			} else {
				w.buf = append(w.buf, cell.Content...)
			}

			w.curX = x + int(cell.Width)
			w.curY = d.Y
		}

		w.setStyle(DefaultColor, DefaultColor, 0)
		w.buf = append(w.buf, clearLineRight...)
		prevY = d.Y
	}
}

func (w *Writer) Reset() {
	w.buf = w.buf[:0]
	w.curFg = DefaultColor
	w.curBg = DefaultColor
	w.curAttr = 0
	w.curX = -1
	w.curY = -1
}

func (w *Writer) moveCursor(x, y int) {
	if w.curY == y && w.curX == x {
		return
	}

	if w.curY == y && x == w.curX+1 {
		w.curX = x

		return
	}

	w.buf = append(w.buf, "\x1b["...)
	w.buf = appendInt(w.buf, y+1)
	w.buf = append(w.buf, ';')
	w.buf = appendInt(w.buf, x+1)
	w.buf = append(w.buf, 'H')
	w.curX = x
	w.curY = y
}

// setStyle updates the terminal style using differential SGR codes.
// Instead of always resetting and re-emitting all attributes, it only
// emits the specific changes needed. This significantly reduces SGR
// output bytes and CPU overhead for style transitions.
//
//nolint:cyclop
func (w *Writer) setStyle(fg, bg Color, attrs Attr) {
	if fg == w.curFg && bg == w.curBg && attrs == w.curAttr {
		return
	}

	fgChanged := fg != w.curFg
	bgChanged := bg != w.curBg
	attrChanged := attrs != w.curAttr

	// Check if we need a full reset. This happens when:
	// - Both fg AND bg changed (complex color transition)
	// - Attrs were removed that don't have specific "off" codes (bold+dim share SGR 22)
	attrRemoved := attrChanged && (w.curAttr&^attrs) != 0
	bothColorsChanged := fgChanged && bgChanged

	if bothColorsChanged || (attrRemoved && (fgChanged || bgChanged)) {
		// Full reset path: emit SGR 0 then set all desired attributes.
		w.buf = append(w.buf, sgrReset...)
		w.curFg = DefaultColor
		w.curBg = DefaultColor
		w.curAttr = 0

		if !fg.IsDefault() {
			w.emitFg(fg)
			w.curFg = fg
		}

		if !bg.IsDefault() {
			w.emitBg(bg)
			w.curBg = bg
		}

		w.emitAttrsOn(attrs)
		w.curAttr = attrs

		return
	}

	// Differential path: emit only the changed SGR codes.

	if fgChanged {
		if fg.IsDefault() {
			w.buf = append(w.buf, sgrFgDefault...)
		} else {
			w.emitFg(fg)
		}

		w.curFg = fg
	}

	if bgChanged {
		if bg.IsDefault() {
			w.buf = append(w.buf, sgrBgDefault...)
		} else {
			w.emitBg(bg)
		}

		w.curBg = bg
	}

	if attrChanged {
		// Emit "off" codes for removed attributes.
		removed := w.curAttr &^ attrs
		added := attrs &^ w.curAttr

		if removed&AttrBold != 0 && removed&AttrDim == 0 {
			// SGR 22 turns off both bold AND dim. If only bold was removed
			// but dim is still on, we need a full attr reset.
			if attrs&AttrDim != 0 {
				// Can't selectively turn off bold without affecting dim.
				// Reset attrs and re-emit.
				w.buf = append(w.buf, sgrReset...)
				w.curFg = DefaultColor
				w.curBg = DefaultColor
				w.curAttr = 0

				if !w.curFg.IsDefault() || !fg.IsDefault() {
					if fg.IsDefault() {
						w.buf = append(w.buf, sgrFgDefault...)
					} else {
						w.emitFg(fg)
					}

					w.curFg = fg
				}

				if !w.curBg.IsDefault() || !bg.IsDefault() {
					if bg.IsDefault() {
						w.buf = append(w.buf, sgrBgDefault...)
					} else {
						w.emitBg(bg)
					}

					w.curBg = bg
				}

				w.emitAttrsOn(attrs)
				w.curAttr = attrs

				return
			}

			w.buf = append(w.buf, sgrBoldOff...)
		} else if removed&(AttrBold|AttrDim) != 0 {
			// SGR 22 turns off both bold and dim.
			w.buf = append(w.buf, sgrBoldOff...)

			// If dim was supposed to stay on, re-emit it.
			if attrs&AttrDim != 0 {
				w.buf = append(w.buf, sgrDimOn...)
			}
		}

		if removed&AttrItalic != 0 {
			w.buf = append(w.buf, sgrItalicOff...)
		}

		if removed&AttrUnderline != 0 {
			w.buf = append(w.buf, sgrUnderlineOff...)
		}

		if removed&AttrBlink != 0 {
			w.buf = append(w.buf, sgrBlinkOff...)
		}

		if removed&AttrReverse != 0 {
			w.buf = append(w.buf, sgrReverseOff...)
		}

		if removed&AttrHidden != 0 {
			w.buf = append(w.buf, sgrHiddenOff...)
		}

		if removed&AttrStrikethrough != 0 {
			w.buf = append(w.buf, sgrStrikeOff...)
		}

		// Emit "on" codes for added attributes.
		if added&AttrBold != 0 {
			w.buf = append(w.buf, sgrBoldOn...)
		}

		if added&AttrDim != 0 {
			w.buf = append(w.buf, sgrDimOn...)
		}

		if added&AttrItalic != 0 {
			w.buf = append(w.buf, sgrItalicOn...)
		}

		if added&AttrUnderline != 0 {
			w.buf = append(w.buf, sgrUnderlineOn...)
		}

		if added&AttrBlink != 0 {
			w.buf = append(w.buf, sgrBlinkOn...)
		}

		if added&AttrReverse != 0 {
			w.buf = append(w.buf, sgrReverseOn...)
		}

		if added&AttrHidden != 0 {
			w.buf = append(w.buf, sgrHiddenOn...)
		}

		if added&AttrStrikethrough != 0 {
			w.buf = append(w.buf, sgrStrikeOn...)
		}

		w.curAttr = attrs
	}
}

// emitAttrsOn emits SGR "on" codes for all set attribute bits.
func (w *Writer) emitAttrsOn(attrs Attr) {
	if attrs&AttrBold != 0 {
		w.buf = append(w.buf, sgrBoldOn...)
	}

	if attrs&AttrDim != 0 {
		w.buf = append(w.buf, sgrDimOn...)
	}

	if attrs&AttrItalic != 0 {
		w.buf = append(w.buf, sgrItalicOn...)
	}

	if attrs&AttrUnderline != 0 {
		w.buf = append(w.buf, sgrUnderlineOn...)
	}

	if attrs&AttrBlink != 0 {
		w.buf = append(w.buf, sgrBlinkOn...)
	}

	if attrs&AttrReverse != 0 {
		w.buf = append(w.buf, sgrReverseOn...)
	}

	if attrs&AttrHidden != 0 {
		w.buf = append(w.buf, sgrHiddenOn...)
	}

	if attrs&AttrStrikethrough != 0 {
		w.buf = append(w.buf, sgrStrikeOn...)
	}
}

//nolint:mnd
func (w *Writer) emitFg(c Color) {
	switch c.ColorType() {
	case colorType16:
		idx := c.PaletteIndex()

		var code int
		if idx < 8 {
			code = 30 + int(idx)
		} else {
			code = 90 + int(idx) - 8
		}

		w.buf = append(w.buf, "\x1b["...)
		w.buf = appendInt(w.buf, code)
		w.buf = append(w.buf, 'm')
	case colorType256:
		w.buf = append(w.buf, "\x1b[38;5;"...)
		w.buf = appendInt(w.buf, int(c.PaletteIndex()))
		w.buf = append(w.buf, 'm')
	default:
		w.buf = append(w.buf, "\x1b[38;2;"...)
		w.buf = appendInt(w.buf, int(c.R()))
		w.buf = append(w.buf, ';')
		w.buf = appendInt(w.buf, int(c.G()))
		w.buf = append(w.buf, ';')
		w.buf = appendInt(w.buf, int(c.B()))
		w.buf = append(w.buf, 'm')
	}
}

//nolint:mnd
func (w *Writer) emitBg(c Color) {
	switch c.ColorType() {
	case colorType16:
		idx := c.PaletteIndex()

		var code int
		if idx < 8 {
			code = 40 + int(idx)
		} else {
			code = 100 + int(idx) - 8
		}

		w.buf = append(w.buf, "\x1b["...)
		w.buf = appendInt(w.buf, code)
		w.buf = append(w.buf, 'm')
	case colorType256:
		w.buf = append(w.buf, "\x1b[48;5;"...)
		w.buf = appendInt(w.buf, int(c.PaletteIndex()))
		w.buf = append(w.buf, 'm')
	default:
		w.buf = append(w.buf, "\x1b[48;2;"...)
		w.buf = appendInt(w.buf, int(c.R()))
		w.buf = append(w.buf, ';')
		w.buf = appendInt(w.buf, int(c.G()))
		w.buf = append(w.buf, ';')
		w.buf = appendInt(w.buf, int(c.B()))
		w.buf = append(w.buf, 'm')
	}
}

// appendInt appends the decimal representation of n to b.
// Fast path for small numbers (< 1000) which covers all terminal coordinates
// and color values. Falls back to strconv.AppendInt for larger numbers.
func appendInt(b []byte, n int) []byte {
	if n < 0 {
		b = append(b, '-')
		n = -n
	}

	if n < 10 {
		return append(b, byte('0'+n))
	}

	if n < 100 {
		return append(b, byte('0'+n/10), byte('0'+n%10))
	}

	if n < 1000 {
		b = append(b, byte('0'+n/100))
		b = append(b, byte('0'+(n/10)%10))

		return append(b, byte('0'+n%10))
	}

	return strconv.AppendInt(b, int64(n), 10)
}

func (w *Writer) writeContent(s string) {
	w.buf = append(w.buf, s...)
}

func (w *Writer) WriteRaw(data []byte) {
	w.buf = append(w.buf, data...)
}

func (w *Writer) WriteClearScreen() {
	w.buf = append(w.buf, "\x1b[2J"...)
	w.buf = append(w.buf, "\x1b[H"...)
	w.curX = 0
	w.curY = 0
}
