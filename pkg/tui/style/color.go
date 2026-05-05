package style

import (
	"fmt"
	"strconv"
)

type Color string

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

//nolint:mnd
func hexVal(hexStr string) uint8 {
	var val uint8

	for i := range len(hexStr) {
		char := hexStr[i]

		switch {
		case char >= '0' && char <= '9':
			val = val*16 + char - '0'
		case char >= 'a' && char <= 'f':
			val = val*16 + char - 'a' + 10
		case char >= 'A' && char <= 'F':
			val = val*16 + char - 'A' + 10
		}
	}

	return val
}

//nolint:mnd
func colorToFgPrefix(c Color) string {
	if c == "" {
		return ""
	}

	red, green, blue, alpha := c.RGBA()
	if alpha == 0 {
		return ""
	}

	return "\x1b[38;2;" +
		strconv.Itoa(int(red>>8)) + ";" +
		strconv.Itoa(int(green>>8)) + ";" +
		strconv.Itoa(int(blue>>8)) + "m"
}

//nolint:mnd
func colorToBgPrefix(c Color) string {
	if c == "" {
		return ""
	}

	red, green, blue, alpha := c.RGBA()
	if alpha == 0 {
		return ""
	}

	return "\x1b[48;2;" +
		strconv.Itoa(int(red>>8)) + ";" +
		strconv.Itoa(int(green>>8)) + ";" +
		strconv.Itoa(int(blue>>8)) + "m"
}

// ColorToRGB8 extracts 8-bit RGB components from a Color.
func ColorToRGB8(c Color) (uint8, uint8, uint8) {
	ru, gu, bu, _ := c.RGBA()

	//nolint:gosec,mnd // G115: safe; 8 = standard RGBA 16→8 bit depth shift
	return uint8(ru >> 8), uint8(gu >> 8), uint8(bu >> 8)
}

// ColorToStyle converts any color.Color to a style.Color hex string.
func ColorToStyle(c Color) Color {
	r, g, b := ColorToRGB8(c)

	return Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}
