package style

type Border struct {
	TopLeft    string
	TopRight   string
	BottomLeft string
	BottomRight string
	Horizontal string
	Vertical   string

	TopMid    string
	BottomMid string
	LeftMid   string
	RightMid  string

	MidMid string

	topFg    string
	rightFg  string
	bottomFg string
	leftFg   string
}

func NormalBorder() Border {
	return Border{
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		Horizontal:  "─",
		Vertical:    "│",
		TopMid:      "┬",
		BottomMid:   "┴",
		LeftMid:     "├",
		RightMid:    "┤",
		MidMid:      "┼",
	}
}

func RoundedBorder() Border {
	return Border{
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
		Horizontal:  "─",
		Vertical:    "│",
		TopMid:      "┬",
		BottomMid:   "┴",
		LeftMid:     "├",
		RightMid:    "┤",
		MidMid:      "┼",
	}
}

func HiddenBorder() Border {
	return Border{}
}

func MarkdownBorder() Border {
	return Border{
		TopLeft:     "|",
		TopRight:    "|",
		BottomLeft:  "|",
		BottomRight: "|",
		Horizontal:  "-",
		Vertical:    "|",
		TopMid:      "|",
		BottomMid:   "|",
		LeftMid:     "|",
		RightMid:    "|",
		MidMid:      "|",
	}
}
