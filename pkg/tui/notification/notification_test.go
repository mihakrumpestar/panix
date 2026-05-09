package notification

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

func TestViewRendersBorderedBox(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	_ = n.Set("hello", style.Color("#50FA7B"))

	result := n.View(style.NewStyle().Bold(true))
	if result == "" {
		t.Fatal("expected non-empty output")
	}

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (top border, content, bottom border), got %d", len(lines))
	}

	top := lines[0]
	content := lines[1]
	bottom := lines[2]

	// Top border should start with ╭ and end with ╮
	if !strings.Contains(top, "╭") || !strings.Contains(top, "╮") {
		t.Fatalf("top border missing rounded corners: %q", top)
	}

	// Top border should have horizontal fill between corners
	if !strings.Contains(top, "─") {
		t.Fatalf("top border missing horizontal fill: %q", top)
	}

	// Content line should have vertical borders
	if !strings.Contains(content, "│") {
		t.Fatalf("content line missing vertical borders: %q", content)
	}

	// Bottom border should start with ╰ and end with ╯
	if !strings.Contains(bottom, "╰") || !strings.Contains(bottom, "╯") {
		t.Fatalf("bottom border missing rounded corners: %q", bottom)
	}

	// Bottom border should have horizontal fill
	if !strings.Contains(bottom, "─") {
		t.Fatalf("bottom border missing horizontal fill: %q", bottom)
	}
}

func TestViewEmptyWhenExpired(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))

	result := n.View(style.NewStyle())
	if result != "" {
		t.Fatalf("expected empty output for expired notification, got %q", result)
	}
}

func TestViewBorderWidthMatchesContent(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	_ = n.Set("short", style.Color("#50FA7B"))

	result := n.View(style.NewStyle())
	topWidth := style.CellWidth(strings.Split(strings.TrimRight(result, "\n"), "\n")[0])

	n2 := New(style.Color("#B4B4B4"))
	_ = n2.Set("a much longer message text", style.Color("#50FA7B"))

	result2 := n2.View(style.NewStyle())
	topWidth2 := style.CellWidth(strings.Split(strings.TrimRight(result2, "\n"), "\n")[0])

	if topWidth2 <= topWidth {
		t.Fatalf("longer content should produce wider border: short=%d, long=%d", topWidth, topWidth2)
	}
}

func TestViewAllLinesEqualWidth(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	_ = n.Set("Here is a notification", style.Color("#50FA7B"))

	result := n.View(style.NewStyle().Bold(true))
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = style.CellWidth(line)
	}

	for i := 1; i < len(widths); i++ {
		if widths[i] != widths[0] {
			t.Fatalf("lines have mismatched widths: %v (lines: %q)", widths, lines)
		}
	}
}

func TestViewContentHasHorizontalPadding(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	_ = n.Set("hello", style.Color("#50FA7B"))

	result := n.View(style.NewStyle().Bold(true))
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	content := lines[1]

	clean := style.StripANSI(content)
	if !strings.HasPrefix(clean, "│ ") || !strings.HasSuffix(clean, " │") {
		t.Fatalf("content line should have padding inside borders: %q", clean)
	}
}
