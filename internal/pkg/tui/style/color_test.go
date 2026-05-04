package style

import (
	"image/color"
	"testing"
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

	for _, tc := range cases {
		r, g, b, a := Color(tc.hex).RGBA()

		if r != tc.want[0] || g != tc.want[1] || b != tc.want[2] || a != tc.want[3] {
			t.Errorf("Color(%q).RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
				tc.hex, r, g, b, a, tc.want[0], tc.want[1], tc.want[2], tc.want[3])
		}
	}
}

func TestColor_RGBA_HexInvalid(t *testing.T) {
	t.Parallel()

	// Too short: len != 6 after # -> returns zero alpha
	r, g, b, a := Color("#FF").RGBA()
	if a != 0 || r != 0 || g != 0 || b != 0 {
		t.Errorf("Color(\"#FF\").RGBA() = (%d, %d, %d, %d), want (0,0,0,0)", r, g, b, a)
	}

	// Too long: len != 6 after # -> returns zero alpha
	r, g, b, a = Color("#FF0000FF").RGBA()
	if a != 0 || r != 0 || g != 0 || b != 0 {
		t.Errorf("Color(\"#FF0000FF\").RGBA() = (%d, %d, %d, %d), want (0,0,0,0)", r, g, b, a)
	}

	// No hash prefix: treated as 256-color index. "FF0000" parsed as int fails -> zero alpha
	r, g, b, a = Color("FF0000").RGBA()
	if a != 0 || r != 0 || g != 0 || b != 0 {
		t.Errorf("Color(\"FF0000\").RGBA() = (%d, %d, %d, %d), want (0,0,0,0)", r, g, b, a)
	}

	// Invalid hex chars: hexVal treats non-hex as 0, so "#ZZZZZZ" -> RGB(0,0,0) with full alpha.
	// This is a known limitation — invalid hex chars silently produce 0 rather than error.
	r, g, b, a = Color("#ZZZZZZ").RGBA()
	if a != 0xFFFF {
		t.Errorf("Color(\"#ZZZZZZ\").RGBA() alpha = %d, want 0xFFFF (invalid hex chars silently treated as 0)", a)
	}

	if r != 0 || g != 0 || b != 0 {
		t.Errorf("Color(\"#ZZZZZZ\").RGBA() = (%d, %d, %d), want (0,0,0) since Z is treated as 0", r, g, b)
	}
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

	for i, want := range expected16 {
		c := Color(intToStr(i))
		r, g, b, a := c.RGBA()

		ru, gu, bu := uint32(want[0]), uint32(want[1]), uint32(want[2])

		if r != ru|ru<<8 || g != gu|gu<<8 || b != bu|bu<<8 || a != 0xFFFF {
			t.Errorf("Color(%d).RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
				i, r, g, b, a, ru|ru<<8, gu|gu<<8, bu|bu<<8, uint32(0xFFFF))
		}
	}
}

func TestColor_RGBA_256Color(t *testing.T) {
	t.Parallel()

	// Color 16 = 6x6x6 cube index 0,0,0 -> RGB(0,0,0)
	r, g, b, a := Color("16").RGBA()

	if r != 0 || g != 0 || b != 0 || a != 0xFFFF {
		t.Errorf("Color(\"16\").RGBA() = (%d, %d, %d, %d), want (0, 0, 0, 65535)", r, g, b, a)
	}

	// Color 196 = cube index (5,0,0) -> idx-16=180, R=(180/36)%6=5, 5*51=255, G=0, B=0
	r, g, b, a = Color("196").RGBA()

	wantR := uint32(255)
	wantR = wantR | wantR<<8

	if r != wantR || g != 0 || b != 0 || a != 0xFFFF {
		t.Errorf("Color(\"196\").RGBA() = (%d, %d, %d, %d), want (%d, 0, 0, 65535)", r, g, b, a, wantR)
	}

	// Color 232 = grayscale start -> R=G=B=8
	r, g, b, a = Color("232").RGBA()

	wantV := uint32(8)
	wantV = wantV | wantV<<8

	if r != wantV || g != wantV || b != wantV || a != 0xFFFF {
		t.Errorf("Color(\"232\").RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, 65535)", r, g, b, a, wantV, wantV, wantV)
	}

	// Color 255 = grayscale end -> R=G=B=238
	r, g, b, a = Color("255").RGBA()

	wantV = uint32(238)
	wantV = wantV | wantV<<8

	if r != wantV || g != wantV || b != wantV || a != 0xFFFF {
		t.Errorf("Color(\"255\").RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, 65535)", r, g, b, a, wantV, wantV, wantV)
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
		r, g, b, a := Color(tc.value).RGBA()

		if a != 0 || r != 0 || g != 0 || b != 0 {
			t.Errorf("Color(%q).RGBA() = (%d, %d, %d, %d), want (0, 0, 0, 0)", tc.value, r, g, b, a)
		}
	}
}

func TestColorToFgPrefix(t *testing.T) {
	t.Parallel()

	// nil color returns empty
	if got := colorToFgPrefix(nil); got != "" {
		t.Errorf("colorToFgPrefix(nil) = %q, want \"\"", got)
	}

	// Zero-alpha color returns empty
	zeroAlpha := color.RGBA{R: 255, G: 0, B: 0, A: 0}
	if got := colorToFgPrefix(zeroAlpha); got != "" {
		t.Errorf("colorToFgPrefix(zeroAlpha) = %q, want \"\"", got)
	}

	// Valid color produces true-color foreground sequence
	c := color.RGBA{R: 255, G: 128, B: 0, A: 255}
	got := colorToFgPrefix(c)
	expected := "\x1b[38;2;255;128;0m"

	if got != expected {
		t.Errorf("colorToFgPrefix(RGBA{255,128,0,255}) = %q, want %q", got, expected)
	}
}

func TestColorToBgPrefix(t *testing.T) {
	t.Parallel()

	// nil color returns empty
	if got := colorToBgPrefix(nil); got != "" {
		t.Errorf("colorToBgPrefix(nil) = %q, want \"\"", got)
	}

	// Zero-alpha color returns empty
	zeroAlpha := color.RGBA{R: 255, G: 0, B: 0, A: 0}
	if got := colorToBgPrefix(zeroAlpha); got != "" {
		t.Errorf("colorToBgPrefix(zeroAlpha) = %q, want \"\"", got)
	}

	// Valid color produces true-color background sequence
	c := color.RGBA{R: 100, G: 200, B: 50, A: 255}
	got := colorToBgPrefix(c)
	expected := "\x1b[48;2;100;200;50m"

	if got != expected {
		t.Errorf("colorToBgPrefix(RGBA{100,200,50,255}) = %q, want %q", got, expected)
	}
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
		got := hexVal(tc.input)

		if got != tc.want {
			t.Errorf("hexVal(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// intToStr converts an int to its string representation without importing strconv.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}

	digits := []byte{}

	neg := n < 0
	if neg {
		n = -n
	}

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if neg {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
