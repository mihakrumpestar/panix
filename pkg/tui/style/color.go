// Derived from charm.land/lipgloss/v2. See pkg/tui/LICENSE.charmbracelet.

package style

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Color string

// colorGrammar is the strict color grammar shared by config validation and
// rendering: "#RRGGBB" hex or a decimal 0-255 palette index.
const colorGrammar = `expected "#RRGGBB" hex or a decimal 0-255 palette index`

// ParseColor parses one color string. The empty string is an error: callers
// that treat empty as unset must skip it before parsing.
func ParseColor(value string) (Color, error) {
	if strings.HasPrefix(value, "#") {
		if len(value) != len("#RRGGBB") {
			return "", errors.Errorf("invalid color %q: %s", value, colorGrammar)
		}

		for _, digit := range value[1:] {
			if !isHexDigit(digit) {
				return "", errors.Errorf("invalid color %q: %s", value, colorGrammar)
			}
		}

		return Color(value), nil
	}

	index, err := strconv.Atoi(value)
	if err != nil || index < 0 || index > 255 { //nolint:mnd // palette index range 0-255
		return "", errors.Errorf("invalid color %q: %s", value, colorGrammar)
	}

	return Color(value), nil
}

// ParseColorOrEmpty is the tolerant render helper: empty or invalid values
// yield "" (uncolored).
func ParseColorOrEmpty(value string) Color {
	color, err := ParseColor(value)
	if err != nil {
		return ""
	}

	return color
}

func isHexDigit(digit rune) bool {
	return digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F'
}

var ansi16 = [16][3]uint8{
	{0, 0, 0},
	{128, 0, 0},
	{0, 128, 0},
	{128, 128, 0},
	{0, 0, 128},
	{128, 0, 128},
	{0, 128, 128},
	{192, 192, 192},
	{128, 128, 128},
	{255, 0, 0},
	{0, 255, 0},
	{255, 255, 0},
	{0, 0, 255},
	{255, 0, 255},
	{0, 255, 255},
	{255, 255, 255},
}

//nolint:mnd
func (c Color) RGBA() (uint32, uint32, uint32, uint32) {
	colorStr := string(c)
	if len(colorStr) == 0 {
		return 0, 0, 0, 0
	}

	if colorStr[0] == '#' {
		colorStr = colorStr[1:]

		if len(colorStr) != 6 {
			return 0, 0, 0, 0
		}

		rv := uint32(hexVal(colorStr[0:2]))
		gv := uint32(hexVal(colorStr[2:4]))
		bv := uint32(hexVal(colorStr[4:6]))

		return rv | rv<<8, gv | gv<<8, bv | bv<<8, 0xFFFF
	}

	idx, err := strconv.Atoi(colorStr)
	if err != nil || idx < 0 || idx > 255 {
		return 0, 0, 0, 0
	}

	var redVal, greenVal, blueVal uint8

	switch {
	case idx < 16:
		redVal = ansi16[idx][0]
		greenVal = ansi16[idx][1]
		blueVal = ansi16[idx][2]
	case idx < 232:
		idx -= 16

		blueVal = uint8(idx%6) * 51
		greenVal = uint8((idx/6)%6) * 51
		redVal = uint8((idx/36)%6) * 51
	default:
		grayVal := uint8(8 + (idx-232)*10)
		redVal, greenVal, blueVal = grayVal, grayVal, grayVal
	}

	ru, gu, bu := uint32(redVal), uint32(greenVal), uint32(blueVal)

	return ru | ru<<8, gu | gu<<8, bu | bu<<8, 0xFFFF
}

func hexVal(hexStr string) uint8 {
	var val uint8

	for i := range len(hexStr) {
		char := hexStr[i]

		switch {
		case char >= '0' && char <= '9':
			val = val*16 + char - '0'
		case char >= 'a' && char <= 'f':
			val = val*16 + char - 'a' + 10 //nolint:mnd
		case char >= 'A' && char <= 'F':
			val = val*16 + char - 'A' + 10 //nolint:mnd
		}
	}

	return val
}

func colorToFgPrefix(c Color) []byte {
	return colorToXPrefix(ansiForeground, c)
}

func colorToBgPrefix(c Color) []byte {
	return colorToXPrefix(ansiBackground, c)
}

func colorToXPrefix(prefix []byte, c Color) []byte {
	if c == "" {
		return nil
	}

	red, green, blue, alpha := c.RGBA()
	if alpha == 0 {
		return nil
	}

	buf := make([]byte, 0, len(prefix)+20) //nolint:mnd
	buf = append(buf, prefix...)

	buf = strconv.AppendInt(buf, int64(red>>8), 10) //nolint:mnd
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(green>>8), 10) //nolint:mnd
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(blue>>8), 10) //nolint:mnd
	buf = append(buf, 'm')

	return buf
}

// ColorToRGB8 extracts 8-bit RGB components from a Color.
func ColorToRGB8(c Color) (uint8, uint8, uint8) {
	ru, gu, bu, _ := c.RGBA()

	//nolint:gosec // G115: safe; 8 = standard RGBA 16→8 bit depth shift
	return uint8(ru >> 8), uint8(gu >> 8), uint8(bu >> 8) //nolint:mnd
}
