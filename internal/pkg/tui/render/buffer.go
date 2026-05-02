package render

import (
	"strconv"

	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
	"github.com/rivo/uniseg"
)

type CellBuf struct {
	cells        []Cell
	width        int
	height       int
	version      uint64
	lineVersions []uint64
}

func NewCellBuf(width, height int) *CellBuf {
	c := &CellBuf{
		width:        width,
		height:       height,
		lineVersions: make([]uint64, height),
	}

	n := width * height
	if n > 0 {
		c.cells = make([]Cell, n)
		for i := range c.cells {
			c.cells[i] = EmptyCell
		}
	}

	return c
}

func (b *CellBuf) Width() int      { return b.width }
func (b *CellBuf) Height() int     { return b.height }
func (b *CellBuf) Version() uint64 { return b.version }

func (b *CellBuf) Resize(width, height int) {
	if width == b.width && height == b.height {
		return
	}

	newCells := make([]Cell, width*height)
	for i := range newCells {
		newCells[i] = EmptyCell
	}

	minH := min(b.height, height)

	minW := min(b.width, width)

	for y := range minH {
		srcStart := y * b.width
		dstStart := y * width
		copy(newCells[dstStart:dstStart+minW], b.cells[srcStart:srcStart+minW])
	}

	newLineVersions := make([]uint64, height)
	copy(newLineVersions, b.lineVersions)
	b.cells = newCells
	b.width = width
	b.height = height
	b.lineVersions = newLineVersions
	b.version++
}

func (b *CellBuf) Clear() {
	for i := range b.cells {
		b.cells[i] = EmptyCell
	}

	for i := range b.lineVersions {
		b.lineVersions[i]++
	}

	b.version++
}

func (b *CellBuf) CellAt(x, y int) Cell {
	if x < 0 || x >= b.width || y < 0 || y >= b.height {
		return EmptyCell
	}

	return b.cells[y*b.width+x]
}

func (b *CellBuf) SetCell(x, y int, c Cell) {
	if x < 0 || x >= b.width || y < 0 || y >= b.height {
		return
	}

	idx := y*b.width + x

	old := b.cells[idx]
	if old.VisualEqual(c) && old.ZoneID == c.ZoneID {
		return
	}

	b.cells[idx] = c
	b.lineVersions[y]++
	b.version++
}

func (b *CellBuf) LineVersion(y int) uint64 {
	if y < 0 || y >= b.height {
		return 0
	}

	return b.lineVersions[y]
}

func (b *CellBuf) SetLineVersion(y int, v uint64) {
	if y < 0 || y >= b.height {
		return
	}

	b.lineVersions[y] = v
}

func (b *CellBuf) Line(y int) []Cell {
	if y < 0 || y >= b.height {
		return nil
	}

	return b.cells[y*b.width : (y+1)*b.width]
}

func (b *CellBuf) WriteANSIString(x, y int, s string) (endX, endY int) {
	p := &ansiParser{
		fg:     DefaultColor,
		bg:     DefaultColor,
		attrs:  0,
		zoneID: 0,
		buf:    b,
		curX:   x,
		curY:   y,
	}
	p.parse(s)

	return p.curX, p.curY
}

func (b *CellBuf) ClearLinesBelow(y int) {
	for row := y; row < b.height; row++ {
		for col := range b.width {
			b.SetCell(col, row, EmptyCell)
		}
	}
}

func (b *CellBuf) WriteStyledText(x, y int, text string, fg, bg Color, attrs Attr, zoneID uint16) (endX, endY int) {
	curX, curY := x, y
	startX := x
	pos := 0
	gs := -1

	for pos < len(text) {
		if curY >= b.height {
			break
		}

		r, size := decodeRune(text, pos)
		if r == '\n' {
			gs = -1
			pos += size
			curX = startX
			curY++

			continue
		}

		if r == '\r' {
			gs = -1
			pos += size
			curX = startX

			continue
		}

		cluster, rest, w, newState := uniseg.FirstGraphemeCluster([]byte(text[pos:]), gs)
		gs = newState
		pos = len(text) - len(rest)

		if curX+w-1 < b.width && curY < b.height {
			b.SetCell(curX, curY, Cell{
				Content: string(cluster),
				Width:   uint8(w),
				Fg:      fg,
				Bg:      bg,
				Attrs:   attrs,
				ZoneID:  zoneID,
			})

			for i := 1; i < w; i++ {
				b.SetCell(curX+i, curY, Cell{
					Content: "",
					Width:   0,
					Fg:      fg,
					Bg:      bg,
					Attrs:   attrs,
					ZoneID:  zoneID,
				})
			}
		}

		curX += w
	}

	return curX, curY
}

// ansiParser parses ANSI escape sequences in a string and writes cells.
type ansiParser struct {
	fg     Color
	bg     Color
	attrs  Attr
	zoneID uint16
	buf    *CellBuf
	curX   int
	curY   int
}

//nolint:cyclop,funlen,gocognit
func (p *ansiParser) parse(s string) {
	pos := 0
	gs := -1 // uniseg state for FirstGraphemeCluster
	startX := p.curX

	for pos < len(s) {
		if s[pos] == '\x1b' {
			gs = -1
			pos = p.parseEscape(s, pos)

			continue
		}

		r, size := decodeRune(s, pos)
		if r == '\n' {
			gs = -1

			p.padLineToWidth(startX)

			pos += size
			p.curX = startX
			p.curY++

			continue
		}

		if r == '\r' {
			gs = -1
			pos += size
			p.curX = startX

			continue
		}

		if r == '\t' {
			gs = -1
			pos += size
			p.curX = ((p.curX / 8) + 1) * 8

			continue
		}

		if r < 0x20 && r != 0 {
			gs = -1
			pos += size

			continue
		}

		if p.curY >= p.buf.height {
			return
		}

		cluster, rest, w, newState := uniseg.FirstGraphemeCluster([]byte(s[pos:]), gs)
		gs = newState
		pos = len(s) - len(rest)

		if p.curX >= p.buf.width {
			p.curX = 0

			p.curY++
			if p.curY >= p.buf.height {
				return
			}
		}

		if p.curX+w <= p.buf.width {
			p.buf.SetCell(p.curX, p.curY, Cell{
				Content: string(cluster),
				Width:   uint8(w),
				Fg:      p.fg,
				Bg:      p.bg,
				Attrs:   p.attrs,
				ZoneID:  p.zoneID,
			})

			for i := 1; i < w; i++ {
				p.buf.SetCell(p.curX+i, p.curY, Cell{
					Content: "",
					Width:   0,
					Fg:      p.fg,
					Bg:      p.bg,
					Attrs:   p.attrs,
					ZoneID:  p.zoneID,
				})
			}

			p.curX += w
		}
	}

	p.padLineToWidth(startX)
}

func (p *ansiParser) padLineToWidth(startX int) {
	if p.curY >= p.buf.height {
		return
	}

	for x := p.curX; x < p.buf.width; x++ {
		p.buf.SetCell(x, p.curY, EmptyCell)
	}
}

//nolint:cyclop,funlen,mnd
func (p *ansiParser) parseEscape(s string, pos int) int {
	if pos+1 >= len(s) {
		return pos + 1
	}

	next := s[pos+1]
	switch {
	case next == '[':
		return p.parseCSI(s, pos+2)
	case next == ']':
		return p.skipOSC(s, pos+2)
	case next >= 0x40 && next <= 0x5F:
		return pos + 2
	default:
		return pos + 1
	}
}

//nolint:cyclop,funlen,mnd
func (p *ansiParser) parseCSI(s string, pos int) int {
	paramStart := pos
	for pos < len(s) && s[pos] >= 0x30 && s[pos] <= 0x3F {
		pos++
	}

	intermediateStart := pos
	for pos < len(s) && s[pos] >= 0x20 && s[pos] <= 0x2F {
		pos++
	}
	// Some CSI sequences (like zone markers ESC[/IDz) have parameter-like
	// bytes after intermediate bytes. Re-scan for trailing param bytes.
	for pos < len(s) && s[pos] >= 0x30 && s[pos] <= 0x3F {
		pos++
	}

	if pos >= len(s) || s[pos] < 0x40 || s[pos] > 0x7E {
		return pos
	}

	finalByte := s[pos]
	paramStr := s[paramStart:intermediateStart]
	intermediateAndTrailing := s[intermediateStart:pos]
	pos++

	switch finalByte {
	case 'm':
		p.applySGR(paramStr)
	case 'z':
		p.applyZoneMarker(paramStr + intermediateAndTrailing)
	}

	return pos
}

//nolint:cyclop,funlen,mnd,gocognit
func (p *ansiParser) applySGR(params string) {
	if params == "" || params == "0" {
		p.fg = DefaultColor
		p.bg = DefaultColor
		p.attrs = 0

		return
	}

	parts := splitParams(params)

	i := 0
	for i < len(parts) {
		switch parts[i] {
		case "0":
			p.fg = DefaultColor
			p.bg = DefaultColor
			p.attrs = 0
		case "1":
			p.attrs |= AttrBold
		case "2":
			p.attrs |= AttrDim
		case "3":
			p.attrs |= AttrItalic
		case "4":
			p.attrs |= AttrUnderline
		case "5":
			p.attrs |= AttrBlink
		case "7":
			p.attrs |= AttrReverse
		case "8":
			p.attrs |= AttrHidden
		case "9":
			p.attrs |= AttrStrikethrough
		case "22":
			p.attrs &^= AttrBold | AttrDim
		case "23":
			p.attrs &^= AttrItalic
		case "24":
			p.attrs &^= AttrUnderline
		case "25":
			p.attrs &^= AttrBlink
		case "27":
			p.attrs &^= AttrReverse
		case "28":
			p.attrs &^= AttrHidden
		case "29":
			p.attrs &^= AttrStrikethrough
		case "30", "31", "32", "33", "34", "35", "36", "37":
			n, _ := strconv.Atoi(parts[i])
			p.fg = color16(uint8(n - 30))
		case "38":
			c, adv := p.parseColor(parts, i)
			if adv > 0 {
				p.fg = c
				i += adv

				continue
			}
		case "39":
			p.fg = DefaultColor
		case "40", "41", "42", "43", "44", "45", "46", "47":
			n, _ := strconv.Atoi(parts[i])
			p.bg = color16(uint8(n - 40))
		case "48":
			c, adv := p.parseColor(parts, i)
			if adv > 0 {
				p.bg = c
				i += adv

				continue
			}
		case "49":
			p.bg = DefaultColor
		case "90", "91", "92", "93", "94", "95", "96", "97":
			n, _ := strconv.Atoi(parts[i])
			p.fg = color16(uint8(n - 90 + 8))
		case "100", "101", "102", "103", "104", "105", "106", "107":
			n, _ := strconv.Atoi(parts[i])
			p.bg = color16(uint8(n - 100 + 8))
		}

		i++
	}
}

//nolint:mnd
func (p *ansiParser) parseColor(parts []string, i int) (Color, int) {
	if i+1 >= len(parts) {
		return DefaultColor, 0
	}

	switch parts[i+1] {
	case "2":
		if i+4 < len(parts) {
			r, _ := strconv.Atoi(parts[i+2])
			g, _ := strconv.Atoi(parts[i+3])
			b, _ := strconv.Atoi(parts[i+4])

			return NewColor(uint8(r), uint8(g), uint8(b)), 5
		}

		return DefaultColor, 0
	case "5":
		if i+2 < len(parts) {
			n, _ := strconv.Atoi(parts[i+2])

			return color256(uint8(n)), 3
		}

		return DefaultColor, 0
	default:
		return DefaultColor, 0
	}
}

func (p *ansiParser) applyZoneMarker(params string) {
	if len(params) > 0 && params[0] == '/' {
		id, err := strconv.ParseUint(params[1:], 10, 16)
		if err == nil {
			_ = globalZones.release(uint16(id))
		}

		p.zoneID = 0

		return
	}

	id, err := strconv.ParseUint(params, 10, 16)
	if err == nil {
		p.zoneID = uint16(id)
		_ = globalZones.acquire(uint16(id))
	}
}

func (p *ansiParser) skipOSC(s string, pos int) int {
	for pos < len(s) {
		if s[pos] == 0x07 {
			return pos + 1
		}

		if s[pos] == '\x1b' && pos+1 < len(s) && s[pos+1] == '\\' {
			return pos + 2
		}

		pos++
	}

	return pos
}

func splitParams(s string) []string {
	var parts []string

	start := 0

	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ';' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}

	return parts
}

//nolint:mnd
func color256(n uint8) Color {
	switch {
	case n < 16:
		return NewColor16(n)
	default:
		return NewColor256(n)
	}
}

//nolint:mnd
func color16(n uint8) Color {
	if n < 16 {
		return NewColor16(n)
	}

	return DefaultColor
}

func decodeRune(s string, pos int) (rune, int) {
	if pos >= len(s) {
		return 0, 0
	}

	if s[pos] < 0x80 {
		return rune(s[pos]), 1
	}

	r := rune(s[pos])
	size := 1

	switch {
	case r&0xE0 == 0xC0:
		if pos+1 < len(s) {
			r = rune(s[pos]&0x1F)<<6 | rune(s[pos+1]&0x3F)
			size = 2
		}
	case r&0xF0 == 0xE0:
		if pos+2 < len(s) {
			r = rune(s[pos]&0x0F)<<12 | rune(s[pos+1]&0x3F)<<6 | rune(s[pos+2]&0x3F)
			size = 3
		}
	case r&0xF8 == 0xF0:
		if pos+3 < len(s) {
			r = rune(s[pos]&0x07)<<18 | rune(s[pos+1]&0x3F)<<12 | rune(s[pos+2]&0x3F)<<6 | rune(s[pos+3]&0x3F)
			size = 4
		}
	}

	return r, size
}

func runeDisplayWidth(r rune) int {
	width := style.RuneWidth(r)
	if width < 0 {
		width = 1
	}

	return width
}
