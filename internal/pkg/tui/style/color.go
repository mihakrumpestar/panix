package style

import (
	"image/color"
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

func (c Color) RGBA() (r, g, b, a uint32) {
	s := string(c)
	if len(s) == 0 {
		return 0, 0, 0, 0
	}

	if s[0] == '#' {
		s = s[1:]

		if len(s) != 6 {
			return 0, 0, 0, 0
		}

		rv := uint32(hexVal(s[0:2]))
		gv := uint32(hexVal(s[2:4]))
		bv := uint32(hexVal(s[4:6]))

		return rv | rv<<8, gv | gv<<8, bv | bv<<8, 0xFFFF
	}

	idx, err := strconv.Atoi(s)
	if err != nil || idx < 0 || idx > 255 {
		return 0, 0, 0, 0
	}

	var rv, gv, bv uint8

	if idx < 16 {
		rv = ansi16[idx][0]
		gv = ansi16[idx][1]
		bv = ansi16[idx][2]
	} else if idx < 232 {
		idx -= 16

		bv = uint8(idx % 6) * 51
		gv = uint8((idx / 6) % 6) * 51
		rv = uint8((idx / 36) % 6) * 51
	} else {
		v := uint8(8 + (idx-232)*10)
		rv, gv, bv = v, v, v
	}

	ru, gu, bu := uint32(rv), uint32(gv), uint32(bv)

	return ru | ru<<8, gu | gu<<8, bu | bu<<8, 0xFFFF
}

func hexVal(s string) uint8 {
	var v uint8

	for i := range len(s) {
		c := s[i]

		switch {
		case c >= '0' && c <= '9':
			v = v*16 + c - '0'
		case c >= 'a' && c <= 'f':
			v = v*16 + c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = v*16 + c - 'A' + 10
		}
	}

	return v
}

func colorToFgPrefix(c color.Color) string {
	if c == nil {
		return ""
	}

	r, g, b, a := c.RGBA()
	if a == 0 {
		return ""
	}

	return "\x1b[38;2;" +
		strconv.Itoa(int(r>>8)) + ";" +
		strconv.Itoa(int(g>>8)) + ";" +
		strconv.Itoa(int(b>>8)) + "m"
}

func colorToBgPrefix(c color.Color) string {
	if c == nil {
		return ""
	}

	r, g, b, a := c.RGBA()
	if a == 0 {
		return ""
	}

	return "\x1b[48;2;" +
		strconv.Itoa(int(r>>8)) + ";" +
		strconv.Itoa(int(g>>8)) + ";" +
		strconv.Itoa(int(b>>8)) + "m"
}
