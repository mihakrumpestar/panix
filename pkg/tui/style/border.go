package style

import "slices"

// borderPresets is the single source of truth for named border glyph sets; every
// preset and named constructor resolves through it. Keys are the preset names
// accepted by theme border sections.
var borderPresets = map[string]Border{
	"sharp": {
		TopLeft:     []byte("┌"),
		TopRight:    []byte("┐"),
		BottomLeft:  []byte("└"),
		BottomRight: []byte("┘"),
		Horizontal:  []byte("─"),
		Vertical:    []byte("│"),
		TopMid:      []byte("┬"),
		BottomMid:   []byte("┴"),
		LeftMid:     []byte("├"),
		RightMid:    []byte("┤"),
		MidMid:      []byte("┼"),
	},

	"rounded": {
		TopLeft:     []byte("╭"),
		TopRight:    []byte("╮"),
		BottomLeft:  []byte("╰"),
		BottomRight: []byte("╯"),
		Horizontal:  []byte("─"),
		Vertical:    []byte("│"),
		TopMid:      []byte("┬"),
		BottomMid:   []byte("┴"),
		LeftMid:     []byte("├"),
		RightMid:    []byte("┤"),
		MidMid:      []byte("┼"),
	},

	"double": {
		TopLeft:     []byte("╔"),
		TopRight:    []byte("╗"),
		BottomLeft:  []byte("╚"),
		BottomRight: []byte("╝"),
		Horizontal:  []byte("═"),
		Vertical:    []byte("║"),
		TopMid:      []byte("╦"),
		BottomMid:   []byte("╩"),
		LeftMid:     []byte("╠"),
		RightMid:    []byte("╣"),
		MidMid:      []byte("╬"),
	},

	"heavy": {
		TopLeft:     []byte("┏"),
		TopRight:    []byte("┓"),
		BottomLeft:  []byte("┗"),
		BottomRight: []byte("┛"),
		Horizontal:  []byte("━"),
		Vertical:    []byte("┃"),
		TopMid:      []byte("┳"),
		BottomMid:   []byte("┻"),
		LeftMid:     []byte("┣"),
		RightMid:    []byte("┫"),
		MidMid:      []byte("╋"),
	},

	"ascii": {
		TopLeft:     []byte("+"),
		TopRight:    []byte("+"),
		BottomLeft:  []byte("+"),
		BottomRight: []byte("+"),
		Horizontal:  []byte("-"),
		Vertical:    []byte("|"),
		TopMid:      []byte("+"),
		BottomMid:   []byte("+"),
		LeftMid:     []byte("+"),
		RightMid:    []byte("+"),
		MidMid:      []byte("+"),
	},

	"dashed": {
		TopLeft:     []byte("┌"),
		TopRight:    []byte("┐"),
		BottomLeft:  []byte("└"),
		BottomRight: []byte("┘"),
		Horizontal:  []byte("╌"),
		Vertical:    []byte("┊"),
		TopMid:      []byte("┬"),
		BottomMid:   []byte("┴"),
		LeftMid:     []byte("├"),
		RightMid:    []byte("┤"),
		MidMid:      []byte("┼"),
	},

	"blank": {
		TopLeft:     []byte(" "),
		TopRight:    []byte(" "),
		BottomLeft:  []byte(" "),
		BottomRight: []byte(" "),
		Horizontal:  []byte(" "),
		Vertical:    []byte(" "),
		TopMid:      []byte(" "),
		BottomMid:   []byte(" "),
		LeftMid:     []byte(" "),
		RightMid:    []byte(" "),
		MidMid:      []byte(" "),
	},

	"markdown": {
		TopLeft:     []byte("|"),
		TopRight:    []byte("|"),
		BottomLeft:  []byte("|"),
		BottomRight: []byte("|"),
		Horizontal:  []byte("-"),
		Vertical:    []byte("|"),
		TopMid:      []byte("|"),
		BottomMid:   []byte("|"),
		LeftMid:     []byte("|"),
		RightMid:    []byte("|"),
		MidMid:      []byte("|"),
	},
}

type Border struct {
	TopLeft     []byte
	TopRight    []byte
	BottomLeft  []byte
	BottomRight []byte
	Horizontal  []byte
	Vertical    []byte

	TopMid    []byte
	BottomMid []byte
	LeftMid   []byte
	RightMid  []byte

	MidMid []byte

	topFg    []byte
	rightFg  []byte
	bottomFg []byte
	leftFg   []byte
}

// BorderByName returns the glyph set of the named preset (see BorderNames).
// ok reports whether the name is known; unknown or empty names return a zero
// Border so callers can fall back without guessing.
func BorderByName(name string) (Border, bool) {
	border, ok := borderPresets[name]

	return border, ok
}

// BorderNames returns the known preset names, sorted.
func BorderNames() []string {
	names := make([]string, 0, len(borderPresets))
	for name := range borderPresets {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// preset resolves a registry preset for the named constructors below.
func preset(name string) Border {
	border, _ := BorderByName(name)

	return border
}

// NormalBorder returns the "sharp" preset: single line corners and edges.
func NormalBorder() Border {
	return preset("sharp")
}

// RoundedBorder returns the "rounded" preset: rounded corners on single line
// edges.
func RoundedBorder() Border {
	return preset("rounded")
}

// MarkdownBorder returns the "markdown" preset: ASCII pipes and dashes.
func MarkdownBorder() Border {
	return preset("markdown")
}

// HiddenBorder returns a zero Border that draws nothing. It is not a named
// preset; themed regions resolve through BorderByName and are never empty.
func HiddenBorder() Border {
	return Border{}
}
