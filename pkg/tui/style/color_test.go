package style

import (
	"bytes"
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
		red, green, blue, alpha := Color(tc.hex).RGBA()

		if red != tc.want[0] || green != tc.want[1] || blue != tc.want[2] || alpha != tc.want[3] {
			t.Errorf("Color(%q).RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
				tc.hex, red, green, blue, alpha, tc.want[0], tc.want[1], tc.want[2], tc.want[3])
		}
	}
}

func TestColor_RGBA_HexInvalid(t *testing.T) {
	t.Parallel()

	// Too short: len != 6 after # -> returns zero alpha
	red, green, blue, alpha := Color("#FF").RGBA()
	if alpha != 0 || red != 0 || green != 0 || blue != 0 {
		t.Errorf("Color(\"#FF\").RGBA() = (%d, %d, %d, %d), want (0,0,0,0)", red, green, blue, alpha)
	}

	// Too long: len != 6 after # -> returns zero alpha
	red, green, blue, alpha = Color("#FF0000FF").RGBA()
	if alpha != 0 || red != 0 || green != 0 || blue != 0 {
		t.Errorf("Color(\"#FF0000FF\").RGBA() = (%d, %d, %d, %d), want (0,0,0,0)", red, green, blue, alpha)
	}

	// No hash prefix: treated as 256-color index. "FF0000" parsed as int fails -> zero alpha
	red, green, blue, alpha = Color("FF0000").RGBA()
	if alpha != 0 || red != 0 || green != 0 || blue != 0 {
		t.Errorf("Color(\"FF0000\").RGBA() = (%d, %d, %d, %d), want (0,0,0,0)", red, green, blue, alpha)
	}

	// Invalid hex chars: hexVal treats non-hex as 0, so "#ZZZZZZ" -> RGB(0,0,0) with full alpha.
	// This is a known limitation — invalid hex chars silently produce 0 rather than error.
	red, green, blue, alpha = Color("#ZZZZZZ").RGBA()
	if alpha != 0xFFFF {
		t.Errorf("Color(\"#ZZZZZZ\").RGBA() alpha = %d, want 0xFFFF (invalid hex chars silently treated as 0)", alpha)
	}

	if red != 0 || green != 0 || blue != 0 {
		t.Errorf("Color(\"#ZZZZZZ\").RGBA() = (%d, %d, %d), want (0,0,0) since Z is treated as 0", red, green, blue)
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

	for colorIdx, want := range expected16 {
		c := Color(intToStr(colorIdx))
		red, green, blue, alpha := c.RGBA()

		ru, gu, bu := uint32(want[0]), uint32(want[1]), uint32(want[2])

		if red != ru|ru<<8 || green != gu|gu<<8 || blue != bu|bu<<8 || alpha != 0xFFFF {
			t.Errorf("Color(%d).RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
				colorIdx, red, green, blue, alpha, ru|ru<<8, gu|gu<<8, bu|bu<<8, uint32(0xFFFF))
		}
	}
}

func TestColor_RGBA_256Color(t *testing.T) {
	t.Parallel()

	// Color 16 = 6x6x6 cube index 0,0,0 -> RGB(0,0,0)
	red, green, blue, alpha := Color("16").RGBA()

	if red != 0 || green != 0 || blue != 0 || alpha != 0xFFFF {
		t.Errorf("Color(\"16\").RGBA() = (%d, %d, %d, %d), want (0, 0, 0, 65535)", red, green, blue, alpha)
	}

	// Color 196 = cube index (5,0,0) -> idx-16=180, R=(180/36)%6=5, 5*51=255, G=0, B=0
	red, green, blue, alpha = Color("196").RGBA()

	wantR := uint32(255)
	wantR |= wantR << 8

	if red != wantR || green != 0 || blue != 0 || alpha != 0xFFFF {
		t.Errorf("Color(\"196\").RGBA() = (%d, %d, %d, %d), want (%d, 0, 0, 65535)", red, green, blue, alpha, wantR)
	}

	// Color 232 = grayscale start -> R=G=B=8
	red, green, blue, alpha = Color("232").RGBA()

	wantV := uint32(8)
	wantV |= wantV << 8

	if red != wantV || green != wantV || blue != wantV || alpha != 0xFFFF {
		t.Errorf("Color(\"232\").RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, 65535)", red, green, blue, alpha, wantV, wantV, wantV)
	}

	// Color 255 = grayscale end -> R=G=B=238
	red, green, blue, alpha = Color("255").RGBA()

	wantV = uint32(238)
	wantV |= wantV << 8

	if red != wantV || green != wantV || blue != wantV || alpha != 0xFFFF {
		t.Errorf("Color(\"255\").RGBA() = (%d, %d, %d, %d), want (%d, %d, %d, 65535)", red, green, blue, alpha, wantV, wantV, wantV)
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

		if alpha != 0 || red != 0 || green != 0 || blue != 0 {
			t.Errorf("Color(%q).RGBA() = (%d, %d, %d, %d), want (0, 0, 0, 0)", tc.value, red, green, blue, alpha)
		}
	}
}

func TestColorToFgPrefix(t *testing.T) {
	t.Parallel()

	// Empty color returns empty
	if got := colorToFgPrefix(""); len(got) != 0 {
		t.Errorf("colorToFgPrefix(\"\") = %q, want \"\"", got)
	}

	// Invalid color returns empty
	if got := colorToFgPrefix(Color("#XYZ")); len(got) != 0 {
		t.Errorf("colorToFgPrefix(#XYZ) = %q, want \"\"", got)
	}

	// Valid color produces true-color foreground sequence
	c := Color("#FF8000")
	got := colorToFgPrefix(c)
	expected := []byte("\x1b[38;2;255;128;0m")

	if !bytes.Equal(got, expected) {
		t.Errorf("colorToFgPrefix(#FF8000) = %q, want %q", got, expected)
	}
}

func TestColorToBgPrefix(t *testing.T) {
	t.Parallel()

	// Empty color returns empty
	if got := colorToBgPrefix(""); len(got) != 0 {
		t.Errorf("colorToBgPrefix(\"\") = %q, want \"\"", got)
	}

	// Invalid color returns empty
	if got := colorToBgPrefix(Color("#XYZ")); len(got) != 0 {
		t.Errorf("colorToBgPrefix(#XYZ) = %q, want \"\"", got)
	}

	// Valid color produces true-color background sequence
	c := Color("#64C832")
	got := colorToBgPrefix(c)
	expected := []byte("\x1b[48;2;100;200;50m")

	if !bytes.Equal(got, expected) {
		t.Errorf("colorToBgPrefix(#64C832) = %q, want %q", got, expected)
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
