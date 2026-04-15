package header

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
)

type Header struct {
	isSnapshot bool
	snapshot   config.Snapshot

	cache cache.Cache[ContentAndHeight]
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
	}, width)
}

func (h *Header) render(width int, cs *colorscheme.ColorScheme) string {
	reason := h.snapshot.Reason.String()

	parts := []string{
		cs.Status.Running.Render(fmt.Sprintf("v%s", h.snapshot.PanixVersion)),
		cs.Table.Border.Render(reason),
		cs.Status.OK.Render("started:", formatTime(h.snapshot.StartTime)),
		cs.Status.Warning.Render("taken:", formatTime(h.snapshot.SnapshotTime)),
	}

	if h.snapshot.WorkflowError != nil {
		parts = append(parts, cs.Status.Failed.Render(h.snapshot.WorkflowError.Error()))
	}

	sep := cs.Table.Border.Render(" │ ")
	line := cs.Header.Title.Render("◉ Snapshot") + cs.Table.Border.Render(": ") + strings.Join(parts, sep)

	return lipgloss.NewStyle().Width(width).Render(line) + "\n\n"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	return t.Format("2006-01-02 15:04:05")
}
