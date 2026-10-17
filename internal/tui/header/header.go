package header

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// Header renders a static snapshot banner built once; only the width varies at
// view time. The width output is cached and rebuilt on resize or theme change.
type Header struct {
	isSnapshot bool
	snapshot   config.Snapshot
	content    *buffer.LinesBuf

	cachedWidth  int
	cachedRender *buffer.LinesBuf
}

func New(isSnapshot bool, snapshot config.Snapshot, scheme *theme.Theme) *Header {
	header := &Header{
		isSnapshot: isSnapshot,
		snapshot:   snapshot,

		cachedWidth:  -1,
		cachedRender: buffer.NewLinesBuf(),
	}
	header.ApplyTheme(scheme)

	return header
}

// ApplyTheme rebuilds the pre-rendered banner and invalidates the cached render.
func (h *Header) ApplyTheme(scheme *theme.Theme) {
	h.cachedWidth = -1

	if !h.isSnapshot {
		return
	}

	if h.content == nil {
		h.content = buffer.NewLinesBuf()
	} else {
		h.content.Reset()
	}

	renderLine(h.content, h.snapshot, scheme)
}

// Render applies the width constraint to the pre-built content and caches the
// result; nil for non-snapshot mode.
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
func renderLine(lineBuf *buffer.LinesBuf, snapshot config.Snapshot, colors *theme.Theme) {
	title := colors.Header.Title.Style
	border := colors.Table.Border.Style

	sep := border.RenderLine(append(append([]byte{' '}, colors.Chars.HeaderSeparatorBytes...), ' '))

	title.RenderAppend(lineBuf, append(append([]byte{}, colors.Chars.SnapshotIconBytes...), " Snapshot"...))
	border.RenderAppend(lineBuf, colors.Chars.HeaderTitleSepBytes)
	lineBuf.AppendToLine([]byte{' '})

	colors.Status.Running.Style.RenderAppend(lineBuf, []byte("v"+snapshot.PanixVersion))
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
