package render

type Attr uint16

const (
	AttrBold Attr = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrReverse
	AttrStrikethrough
	AttrHidden
)

type Color uint32

const DefaultColor Color = 0

const (
	colorTypeShift   = 4
	colorTypeMask    = 0xF
	colorTypeTrue    = 0x1
	colorType16      = 0x2
	colorType256     = 0x3
	colorPaletteMask = 0xFF
)

// asciiStrings pre-allocates string representations for bytes 0-127.
// This eliminates per-cell heap allocations for ASCII content in the
// hot parse path — instead of string([]byte{ch}), we index this table.
var asciiStrings [128]string

// emptyStr is the pre-allocated continuation cell content string.
var emptyStr = ""

func init() {
	for i := range asciiStrings {
		asciiStrings[i] = string([]byte{byte(i)})
	}
}

func NewColor(r, g, b uint8) Color {
	return Color(uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | colorTypeTrue<<colorTypeShift | 0x1)
}

func NewColor16(palette uint8) Color {
	return Color(uint32(palette)<<24 | uint32(colorType16)<<colorTypeShift | 0x1)
}

func NewColor256(index uint8) Color {
	return Color(uint32(index)<<24 | uint32(colorType256)<<colorTypeShift | 0x1)
}

func ColorFromRGBA(r, g, b, a uint32) Color {
	return Color((r>>8)<<24 | (g>>8)<<16 | (b>>8)<<8 | colorTypeTrue<<colorTypeShift | 0x1)
}

func (c Color) R() uint8       { return uint8(c >> 24) }
func (c Color) G() uint8       { return uint8(c >> 16) }
func (c Color) B() uint8       { return uint8(c >> 8) }
func (c Color) A() uint8       { return uint8(c) & 0x1 }
func (c Color) IsDefault() bool { return c == 0 }
func (c Color) IsRGB() bool     { return c != 0 }

func (c Color) ColorType() int     { return int((c >> colorTypeShift) & colorTypeMask) }
func (c Color) PaletteIndex() uint8 { return uint8(c >> 24) }

type Cell struct {
	Content string
	Width   uint8
	Fg      Color
	Bg      Color
	Attrs   Attr
	ZoneID  uint16
}

var EmptyCell = Cell{
	Content: " ",
	Width:   1,
	Fg:      DefaultColor,
	Bg:      DefaultColor,
}

func (c Cell) VisualEqual(o Cell) bool {
	return c.Content == o.Content && c.Fg == o.Fg && c.Bg == o.Bg && c.Attrs == o.Attrs && c.Width == o.Width
}
