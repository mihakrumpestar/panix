package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColor_RGBA_Hex(t *testing.T) {
	t.Parallel()

	cases := []struct {
		hex  string
		want [4]uint32
	}{
		{"#000000", [4]uint32{0, 0, 0, 0xFFFF}},
		{"#FF0000", [4]uint32{0xFFFF, 0, 0, 0xFFFF}},
		{"#00FF00", [4]uint32{0, 0xFFFF, 0, 0xFFFF}},
		{"#0000FF", [4]uint32{0, 0, 0xFFFF, 0xFFFF}},
		{"#F1FA8C", [4]uint32{0xF1F1, 0xFAFA, 0x8C8C, 0xFFFF}},
		{"#50FA7B", [4]uint32{0x5050, 0xFAFA, 0x7B7B, 0xFFFF}},
	}

	for _, caseI := range cases {
		red, green, blue, alpha := Color(caseI.hex).RGBA()

		assert.Equal(t, caseI.want[0], red)
		assert.Equal(t, caseI.want[1], green)
		assert.Equal(t, caseI.want[2], blue)
		assert.Equal(t, caseI.want[3], alpha)
	}
}

func TestColor_RGBA_HexInvalid(t *testing.T) {
	t.Parallel()

	for _, c := range []Color{Color("#FF"), Color("#FF0000FF"), Color("FF0000")} {
		r, g, b, a := c.RGBA()
		assert.Equal(t, uint32(0), r)
		assert.Equal(t, uint32(0), g)
		assert.Equal(t, uint32(0), b)
		assert.Equal(t, uint32(0), a)
	}

	r, g, b, a := Color("#ZZZZZZ").RGBA()
	assert.Equal(t, uint32(0xFFFF), a)
	assert.Equal(t, uint32(0), r)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
}

func TestColor_RGBA_16Color(t *testing.T) {
	t.Parallel()

	// First 16 ANSI colors
	expected16 := [16][3]uint8{
		{0, 0, 0},       // 0: black
		{128, 0, 0},     // 1: red
		{0, 128, 0},     // 2: green
		{128, 128, 0},   // 3: yellow
		{0, 0, 128},     // 4: blue
		{128, 0, 128},   // 5: magenta
		{0, 128, 128},   // 6: cyan
		{192, 192, 192}, // 7: white
		{128, 128, 128}, // 8: bright black
		{255, 0, 0},     // 9: bright red
		{0, 255, 0},     // 10: bright green
		{255, 255, 0},   // 11: bright yellow
		{0, 0, 255},     // 12: bright blue
		{255, 0, 255},   // 13: bright magenta
		{0, 255, 255},   // 14: bright cyan
		{255, 255, 255}, // 15: bright white
	}

	for colorIdx, want := range expected16 {
		c := Color(intToStr(colorIdx))
		red, green, blue, alpha := c.RGBA()

		ru, gu, bu := uint32(want[0]), uint32(want[1]), uint32(want[2])

		assert.Equal(t, ru|ru<<8, red)
		assert.Equal(t, gu|gu<<8, green)
		assert.Equal(t, bu|bu<<8, blue)
		assert.Equal(t, uint32(0xFFFF), alpha)
	}
}

func TestColor_RGBA_256Color(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    Color
		want [4]uint32
	}{
		{"16", Color("16"), [4]uint32{0, 0, 0, 0xFFFF}},
		{"196", Color("196"), [4]uint32{uint32(255)<<8 | 255, 0, 0, 0xFFFF}},
		{"232", Color("232"), [4]uint32{uint32(8)<<8 | 8, uint32(8)<<8 | 8, uint32(8)<<8 | 8, 0xFFFF}},
		{"255", Color("255"), [4]uint32{uint32(238)<<8 | 238, uint32(238)<<8 | 238, uint32(238)<<8 | 238, 0xFFFF}},
	}

	for _, tt := range tests {
		r, g, b, a := tt.c.RGBA()
		assert.Equal(t, tt.want[0], r, "%s: red mismatch", tt.name)
		assert.Equal(t, tt.want[1], g, "%s: green mismatch", tt.name)
		assert.Equal(t, tt.want[2], b, "%s: blue mismatch", tt.name)
		assert.Equal(t, tt.want[3], a, "%s: alpha mismatch", tt.name)
	}
}

func TestColor_RGBA_Invalid256(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
	}{
		{"Negative", "-1"},
		{"TooLarge", "256"},
		{"NonNumeric", "abc"},
		{"Empty", ""},
	}

	for _, tc := range cases {
		red, green, blue, alpha := Color(tc.value).RGBA()

		assert.Equal(t, uint32(0), alpha)
		assert.Equal(t, uint32(0), red)
		assert.Equal(t, uint32(0), green)
		assert.Equal(t, uint32(0), blue)
	}
}

func TestColorToFgPrefix(t *testing.T) {
	t.Parallel()

	assert.Empty(t, colorToFgPrefix(""))
	assert.Empty(t, colorToFgPrefix(Color("#XYZ")))

	c := Color("#FF8000")
	got := colorToFgPrefix(c)
	expected := []byte("\x1b[38;2;255;128;0m")

	assert.Equal(t, expected, got)
}

func TestColorToBgPrefix(t *testing.T) {
	t.Parallel()

	assert.Empty(t, colorToBgPrefix(""))
	assert.Empty(t, colorToBgPrefix(Color("#XYZ")))

	c := Color("#64C832")
	got := colorToBgPrefix(c)
	expected := []byte("\x1b[48;2;100;200;50m")

	assert.Equal(t, expected, got)
}

func TestHexVal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  uint8
	}{
		{"00", 0},
		{"FF", 255},
		{"ff", 255},
		{"0A", 10},
		{"a0", 160},
		{"1F", 31},
		{"AB", 171},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.want, hexVal(tc.input))
	}
}

// intToStr converts an int to its string representation without importing strconv.
func intToStr(val int) string {
	if val == 0 {
		return "0"
	}

	digits := []byte{}

	neg := val < 0
	if neg {
		val = -val
	}

	for val > 0 {
		digits = append([]byte{byte('0' + val%10)}, digits...)
		val /= 10
	}

	if neg {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
