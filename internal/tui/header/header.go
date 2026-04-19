package header

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
)

type headerCacheKey struct {
	width int
}

type Header struct {
	isSnapshot bool
	snapshot   config.Snapshot

	cache cache.Cache[ContentAndHeight, headerCacheKey]
}

type ContentAndHeight struct {
	Content string
	Height  int
}

func New(isSnapshot bool, snapshot config.Snapshot) *Header {
	return &Header{
		isSnapshot: isSnapshot,
		snapshot:   snapshot,
	}
}

func (h *Header) View(width int, colorScheme *colorscheme.ColorScheme) ContentAndHeight {
	if !h.isSnapshot {
		return ContentAndHeight{}
	}

	return h.cache.Get(func() (ContentAndHeight, bool) {
		content := h.render(width, colorScheme)
		height := lipgloss.Height(content) - 1 // -1 to account for next view

		return ContentAndHeight{Content: content, Height: height}, true
	}, headerCacheKey{width: width})
}

func (h *Header) render(width int, colorScheme *colorscheme.ColorScheme) string {
	reason := h.snapshot.Reason.String()

	parts := []string{
		colorScheme.Status.Running.Render("v" + h.snapshot.PanixVersion),
		colorScheme.Table.Border.Render(reason),
		colorScheme.Table.Border.Render("started:", formatTime(h.snapshot.StartTime)),
		colorScheme.Table.Border.Render("taken:", formatTime(h.snapshot.SnapshotTime)),
	}

	if h.snapshot.WorkflowError != nil {
		parts = append(parts, colorScheme.Status.Failed.Render(h.snapshot.WorkflowError.Error()))
	}

	sep := colorScheme.Table.Border.Render(" │ ")
	line := colorScheme.Header.Title.Render("◉ Snapshot") + colorScheme.Table.Border.Render(": ") + strings.Join(parts, sep)

	return lipgloss.NewStyle().Width(width).Render(line) + "\n\n"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	return t.Format("2006-01-02 15:04:05")
}
