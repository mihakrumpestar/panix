package style

var (
	normalBorder = Border{
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
	}

	roundedBorder = Border{
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
	}

	markdownBorder = Border{
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
	}
)

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

func NormalBorder() Border {
	return normalBorder
}

func RoundedBorder() Border {
	return roundedBorder
}

func MarkdownBorder() Border {
	return markdownBorder
}

func HiddenBorder() Border {
	return Border{}
}
