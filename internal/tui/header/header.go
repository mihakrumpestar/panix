package header

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// Header renders a static snapshot banner. Content is built once at construction;
// only the terminal width varies at view time. The width-constrained output is
// cached so re-rendering only happens on resize.
type Header struct {
	isSnapshot bool
	content    *buffer.LinesBuf

	cachedWidth  int
	cachedRender *buffer.LinesBuf
}

func New(isSnapshot bool, snapshot config.Snapshot, colorScheme *colorscheme.ColorScheme) *Header {
	header := &Header{
		isSnapshot: isSnapshot,

		cachedWidth:  -1,
		cachedRender: buffer.NewLinesBuf(),
	}

	if isSnapshot {
		header.content = buffer.NewLinesBuf()
		renderLine(header.content, snapshot, colorScheme)
	}

	return header
}

// Render applies the width constraint to the pre-built content and returns
// the cached result. Returns nil for non-snapshot mode.
func (h *Header) Render(width int) *buffer.LinesBuf {
	if !h.isSnapshot {
		return nil
	}

	if width != h.cachedWidth {
		h.cachedWidth = width

		h.cachedRender.Reset()
		style.NewStyle().MaxWidth(width).RenderIntoBuf(h.cachedRender, h.content)
	}

	return h.cachedRender
}

func (h *Header) Len() int {
	return h.cachedRender.Len()
}

// renderLine builds the header: ◉ Snapshot ─ v0.1.0 │ deploy │ started: ... │ taken: ...
func renderLine(lineBuf *buffer.LinesBuf, snapshot config.Snapshot, colors *colorscheme.ColorScheme) {
	title := colors.Header.Title
	border := colors.Table.Border

	sep := border.RenderLine(append(append([]byte{' '}, colors.Chars.HeaderSeparator...), ' '))

	title.RenderAppend(lineBuf, append(append([]byte{}, colors.Chars.SnapshotIcon...), " Snapshot"...))
	border.RenderAppend(lineBuf, colors.Chars.HeaderTitleSep)
	lineBuf.AppendToLine([]byte{' '})

	colors.Status.Running.RenderAppend(lineBuf, []byte("v"+snapshot.PanixVersion))
	lineBuf.AppendToLine(sep)
	border.RenderAppend(lineBuf, []byte(snapshot.Reason.String()))
	lineBuf.AppendToLine(sep)
	border.RenderAppend(lineBuf, []byte("started: "+formatTime(snapshot.StartTime)))
	lineBuf.AppendToLine(sep)
	border.RenderAppend(lineBuf, []byte("taken: "+formatTime(snapshot.SnapshotTime)))

	lineBuf.WriteLine(nil)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	return t.Format("2006-01-02 15:04:05")
}
