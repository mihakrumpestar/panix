package header

import (
	"strings"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

// Header renders a static snapshot banner. All styled content is pre-rendered
// at construction time since snapshot data and colors never change; only the
// terminal width varies at view time. The rendered output is cached by width
// so re-rendering only happens on resize.
type Header struct {
	isSnapshot  bool
	line        string
	cachedWidth int
	cachedView  string
}

// New creates a Header. When isSnapshot is true the styled line is rendered
// immediately so that View() only needs to apply a width constraint.
func New(isSnapshot bool, snapshot config.Snapshot, colorScheme *colorscheme.ColorScheme) *Header {
	header := &Header{
		isSnapshot:  isSnapshot,
		cachedWidth: -1,
	}

	if isSnapshot {
		header.line = renderLine(snapshot, colorScheme)
	}

	return header
}

func (h *Header) View(width int) string {
	if !h.isSnapshot {
		return ""
	}

	if width == h.cachedWidth {
		return h.cachedView
	}

	h.cachedWidth = width
	h.cachedView = style.NewStyle().Width(width).Render(h.line) + "\n\n"

	return h.cachedView
}

// renderLine builds the fully-styled, width-independent header line from the
// snapshot data. Called once at construction time.
func renderLine(snapshot config.Snapshot, colorScheme *colorscheme.ColorScheme) string {
	reason := snapshot.Reason.String()
	sep := colorScheme.Table.Border.Render(" " + colorScheme.Chars.HeaderSeparator + " ")

	parts := []string{
		colorScheme.Status.Running.Render("v" + snapshot.PanixVersion),
		colorScheme.Table.Border.Render(reason),
		colorScheme.Table.Border.Render("started: ", formatTime(snapshot.StartTime)),
		colorScheme.Table.Border.Render("taken: ", formatTime(snapshot.SnapshotTime)),
	}

	if snapshot.WorkflowError != nil {
		parts = append(parts, colorScheme.Status.Failed.Render(snapshot.WorkflowError.Error()))
	}

	line := colorScheme.Header.Title.Render(colorScheme.Chars.SnapshotIcon+" Snapshot") +
		colorScheme.Table.Border.Render(colorScheme.Chars.HeaderTitleSep) + " " + strings.Join(parts, sep)

	return line
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	return t.Format("2006-01-02 15:04:05")
}
