package overlay

import (
	"bytes"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// titleEdgeGap is the minimum gap in cells between a title and the border
// corners on each side.
const titleEdgeGap = 1

// Box is an immutable rendered bordered box ready to be composited over a
// frame. Build it with NewBox; it can be composed any number of times.
type Box struct {
	lines *buffer.LinesBuf
	width int
}

// NewBox builds a bordered box from content lines, which may contain ANSI
// styling; they are copied and never mutated or retained. Empty content produces
// one blank body line.
func NewBox(content [][]byte, opts ...Option) *Box {
	cfg := defaultBoxOptions(opts...)

	border, sides := cfg.style.GetBorder()
	embedTitle := cfg.embedsTitle(border, sides.Top)

	src := cfg.buildContent(content, embedTitle)
	defer src.Release()

	boxStyle := cfg.renderStyle()

	dst := buffer.NewLinesBuf()
	boxStyle.RenderIntoBuf(dst, src)

	if embedTitle {
		cfg.drawTitle(dst, src, boxStyle, border)
	}

	return measureBox(dst)
}

// Line returns the i-th rendered line of the box. The returned slice is owned
// by the Box; do not modify or retain it.
func (b *Box) Line(i int) []byte {
	return b.lines.Line(i)
}

// Len returns the number of rendered lines (the box height).
func (b *Box) Len() int {
	return b.lines.Len()
}

// Width returns the visible width of the box in cells.
func (b *Box) Width() int {
	return b.width
}

// Option configures NewBox.
type Option func(*boxOptions)

type boxOptions struct {
	style      style.Style
	title      string
	titleStyle style.Style
	titleAlign style.Position
	width      int
	maxWidth   int
}

// WithStyle sets the style for the box body and border, replacing the default
// sharp preset. A title is drawn in the top border row when the style has a
// visible top border, otherwise as the first body line.
func WithStyle(st style.Style) Option {
	return func(o *boxOptions) {
		o.style = st
	}
}

// WithTitle sets the optional title. A derived width grows to fit it; WithWidth
// or WithMaxWidth truncates it to fit.
func WithTitle(title string) Option {
	return func(o *boxOptions) {
		o.title = title
	}
}

// WithTitleStyle sets the style used to render the title text. With no style
// the title inherits the border row style.
func WithTitleStyle(st style.Style) Option {
	return func(o *boxOptions) {
		o.titleStyle = st
	}
}

// WithTitleAlign sets the title alignment within the top border row
// (style.Left, style.Center, style.Right). Default: style.Center.
func WithTitleAlign(p style.Position) Option {
	return func(o *boxOptions) {
		o.titleAlign = p
	}
}

// WithWidth sets the total width in cells, including borders and padding.
// Shorter lines are padded, longer lines truncated. Zero derives the width from
// the widest content line and the title.
func WithWidth(w int) Option {
	return func(o *boxOptions) {
		o.width = w
	}
}

// WithMaxWidth caps the total box width in cells, including borders and
// padding. 0 (the default) means unlimited.
func WithMaxWidth(w int) Option {
	return func(o *boxOptions) {
		o.maxWidth = w
	}
}

// defaultBoxOptions applies opts to the defaults.
func defaultBoxOptions(opts ...Option) boxOptions {
	cfg := boxOptions{
		style:      style.NewStyle().Border(style.NormalBorder()),
		titleAlign: style.Center,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// embedsTitle reports whether the title is drawn into the top border row
// instead of becoming the first body line.
func (o boxOptions) embedsTitle(border style.Border, borderTop bool) bool {
	return o.title != "" && borderTop &&
		len(border.TopLeft) > 0 && len(border.TopRight) > 0
}

// buildContent assembles the title body line when not embedded, the copied
// content lines, and a blank line for empty content.
func (o boxOptions) buildContent(content [][]byte, embedTitle bool) *buffer.LinesBuf {
	src := buffer.NewLinesBuf()

	if o.title != "" && !embedTitle {
		src.WriteLine1(o.titleStyle.RenderLine([]byte(o.title)))
	}

	src.WriteLines(content)

	if src.Len() == 0 {
		src.EmptyLine()
	}

	return src
}

// renderStyle returns the box style with the width options applied.
func (o boxOptions) renderStyle() style.Style {
	boxStyle := o.style

	if o.width > 0 {
		boxStyle = boxStyle.Width(o.width)
	}

	if o.maxWidth > 0 {
		boxStyle = boxStyle.MaxWidth(o.maxWidth)
	}

	return boxStyle
}

// drawTitle embeds the title into dst's top border row, re-rendering wider first
// when a derived width does not fit. src is the content needed for the re-render.
func (o boxOptions) drawTitle(dst, src *buffer.LinesBuf, boxStyle style.Style, border style.Border) {
	// Measure the rendered title: a title style can add padding and would
	// otherwise overflow the row it was measured against.
	title := o.titleStyle.RenderLine([]byte(o.title))

	// Widen a derived-width box so the title fits; insertTitle truncates as the
	// fallback.
	if boxStyle.GetWidth() == 0 {
		if deficit := titleDeficit(dst.Line(0), border, title); deficit > 0 {
			boxStyle = boxStyle.Width(style.CellWidth(dst.Line(0)) + deficit)

			dst.Reset()
			boxStyle.RenderIntoBuf(dst, src)
		}
	}

	dst.SetLine(0, insertTitle(dst.Line(0), border, title, o.titleAlign))
}

// measureBox wraps the rendered lines in a Box and computes its visible width.
func measureBox(dst *buffer.LinesBuf) *Box {
	width := 0

	for i := range dst.Len() {
		width = max(width, style.CellWidth(dst.Line(i)))
	}

	return &Box{lines: dst, width: width}
}

// borderFill returns the byte range of the horizontal fill between the corner
// glyphs of a rendered border row built from border.
func borderFill(top []byte, border style.Border) (int, int, bool) {
	iTL := bytes.Index(top, border.TopLeft)
	iTR := bytes.LastIndex(top, border.TopRight)

	if iTL < 0 || iTR < iTL+len(border.TopLeft) {
		return 0, 0, false
	}

	return iTL + len(border.TopLeft), iTR, true
}

// titleDeficit returns how many cells the border fill of a rendered border
// row must grow for title to fit with its edge gaps. Returns 0 when it fits.
func titleDeficit(top []byte, border style.Border, title []byte) int {
	start, end, ok := borderFill(top, border)
	if !ok {
		return 0
	}

	needed := style.CellWidth(title) + 2*titleEdgeGap

	return max(needed-style.CellWidth(top[start:end]), 0)
}

// insertTitle embeds a rendered title in a horizontal border row built from
// border, keeping a titleEdgeGap to the corners and re-emitting the border row
// style afterwards so the rest keeps its colors. Returns top unchanged when there
// is no room.
func insertTitle(top []byte, border style.Border, title []byte, align style.Position) []byte {
	fillStart, fillEnd, ok := borderFill(top, border)
	if !ok {
		return top
	}

	fillWidth := style.CellWidth(top[fillStart:fillEnd])
	usable := fillWidth - 2*titleEdgeGap

	if usable <= 0 {
		return top
	}

	titleWidth := style.CellWidth(title)

	if titleWidth > usable {
		title = style.TruncateToWidth(title, usable, true)
		titleWidth = style.CellWidth(title)
	}

	if titleWidth == 0 {
		return top
	}

	gap := fillWidth - titleWidth
	leftGap := titleEdgeGap

	switch {
	case align >= style.Right:
		leftGap = gap - titleEdgeGap
	case align == style.Center:
		leftGap = gap / 2
	}

	rightGap := gap - leftGap
	prefix := top[:fillStart-len(border.TopLeft)] // the border row style active before the title

	out := make([]byte, 0, len(top)+len(title)+2*len(prefix)+gap)
	out = append(out, top[:fillStart]...)
	out = append(out, spaces(leftGap)...)
	out = append(out, title...)
	out = append(out, prefix...) // re-emit the border style after the title
	out = append(out, spaces(rightGap)...)
	out = append(out, top[fillEnd:]...)

	return out
}

// spaces returns n space bytes.
func spaces(n int) []byte {
	return bytes.Repeat([]byte{' '}, n)
}
