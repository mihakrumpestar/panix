package render

import (
	"errors"
	"fmt"
	"io"
)

const (
	csiCursorPos    = "\x1b[%d;%dH"
	sgrFg24bit      = "\x1b[38;2;%d;%d;%dm"
	sgrBg24bit      = "\x1b[48;2;%d;%d;%dm"
	sgrFg256        = "\x1b[38;5;%dm"
	sgrBg256        = "\x1b[48;5;%dm"
	sgrFgDefault    = "\x1b[39m"
	sgrBgDefault    = "\x1b[49m"
	sgrReset        = "\x1b[0m"
	sgrBoldOn       = "\x1b[1m"
	sgrBoldOff      = "\x1b[22m"
	sgrDimOn        = "\x1b[2m"
	sgrDimOff       = "\x1b[22m"
	sgrItalicOn     = "\x1b[3m"
	sgrItalicOff    = "\x1b[23m"
	sgrUnderlineOn  = "\x1b[4m"
	sgrUnderlineOff = "\x1b[24m"
	sgrBlinkOn      = "\x1b[5m"
	sgrBlinkOff     = "\x1b[25m"
	sgrReverseOn    = "\x1b[7m"
	sgrReverseOff   = "\x1b[27m"
	sgrHiddenOn     = "\x1b[8m"
	sgrHiddenOff    = "\x1b[28m"
	sgrStrikeOn     = "\x1b[9m"
	sgrStrikeOff    = "\x1b[29m"
	clearLineRight  = "\x1b[K"
	sgrFg16         = "\x1b[%dm"
	sgrBg16         = "\x1b[%dm"
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

	for _, d := range diffs {
		w.moveCursor(0, d.Y)

		for x := range width {
			cell := buf.CellAt(x, d.Y)
			if cell.Width == 0 {
				continue
			}

			w.setStyle(cell.Fg, cell.Bg, cell.Attrs)

			if cell.Content == "" || cell.Content == " " {
				w.writeContent(" ")
			} else {
				w.writeContent(cell.Content)
			}

			w.curX = x + int(cell.Width)
			w.curY = d.Y
		}

		w.setStyle(DefaultColor, DefaultColor, 0)
		w.buf = append(w.buf, clearLineRight...)
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

	w.buf = fmt.Appendf(w.buf, csiCursorPos, y+1, x+1)
	w.curX = x
	w.curY = y
}

//nolint:cyclop
func (w *Writer) setStyle(fg, bg Color, attrs Attr) {
	if fg == w.curFg && bg == w.curBg && attrs == w.curAttr {
		return
	}

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

	if attrs&AttrBold != 0 {
		w.buf = append(w.buf, sgrBoldOn...)
		w.curAttr |= AttrBold
	}

	if attrs&AttrDim != 0 {
		w.buf = append(w.buf, sgrDimOn...)
		w.curAttr |= AttrDim
	}

	if attrs&AttrItalic != 0 {
		w.buf = append(w.buf, sgrItalicOn...)
		w.curAttr |= AttrItalic
	}

	if attrs&AttrUnderline != 0 {
		w.buf = append(w.buf, sgrUnderlineOn...)
		w.curAttr |= AttrUnderline
	}

	if attrs&AttrBlink != 0 {
		w.buf = append(w.buf, sgrBlinkOn...)
		w.curAttr |= AttrBlink
	}

	if attrs&AttrReverse != 0 {
		w.buf = append(w.buf, sgrReverseOn...)
		w.curAttr |= AttrReverse
	}

	if attrs&AttrHidden != 0 {
		w.buf = append(w.buf, sgrHiddenOn...)
		w.curAttr |= AttrHidden
	}

	if attrs&AttrStrikethrough != 0 {
		w.buf = append(w.buf, sgrStrikeOn...)
		w.curAttr |= AttrStrikethrough
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

		w.buf = fmt.Appendf(w.buf, sgrFg16, code)
	case colorType256:
		w.buf = fmt.Appendf(w.buf, sgrFg256, c.PaletteIndex())
	default:
		w.buf = fmt.Appendf(w.buf, sgrFg24bit, c.R(), c.G(), c.B())
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

		w.buf = fmt.Appendf(w.buf, sgrBg16, code)
	case colorType256:
		w.buf = fmt.Appendf(w.buf, sgrBg256, c.PaletteIndex())
	default:
		w.buf = fmt.Appendf(w.buf, sgrBg24bit, c.R(), c.G(), c.B())
	}
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
