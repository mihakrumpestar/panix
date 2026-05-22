package notification

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
)

func viewString(n *Notification) string {
	v := n.Render()
	if v == nil {
		return ""
	}

	return v.String()
}

func TestViewRendersBorderedBox(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	n.SetBaseStyle(style.NewStyle().Bold(true))
	_ = n.Set("hello", style.Color("#50FA7B"))

	result := viewString(n)
	assert.NotEmpty(t, result, "expected non-empty output")

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "expected at least 3 lines")

	top := lines[0]
	content := lines[1]
	bottom := lines[2]

	assert.Contains(t, top, "╭", "top border missing ╭")
	assert.Contains(t, top, "╮", "top border missing ╮")
	assert.Contains(t, top, "─", "top border missing horizontal fill")
	assert.Contains(t, content, "│", "content line missing vertical borders")
	assert.Contains(t, bottom, "╰", "bottom border missing ╰")
	assert.Contains(t, bottom, "╯", "bottom border missing ╯")
	assert.Contains(t, bottom, "─", "bottom border missing horizontal fill")
}

func TestViewEmptyWhenExpired(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	n.SetBaseStyle(style.NewStyle())

	assert.Empty(t, viewString(n), "expected empty output for expired notification")
}

func TestViewBorderWidthMatchesContent(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	n.SetBaseStyle(style.NewStyle())
	_ = n.Set("short", style.Color("#50FA7B"))

	result := viewString(n)
	topWidth := style.CellWidth([]byte(strings.Split(strings.TrimRight(result, "\n"), "\n")[0]))

	n2 := New(style.Color("#B4B4B4"))
	n2.SetBaseStyle(style.NewStyle())
	_ = n2.Set("a much longer message text", style.Color("#50FA7B"))

	result2 := viewString(n2)
	topWidth2 := style.CellWidth([]byte(strings.Split(strings.TrimRight(result2, "\n"), "\n")[0]))

	assert.Greater(t, topWidth2, topWidth, "longer content should produce wider border")
}

func TestViewAllLinesEqualWidth(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	n.SetBaseStyle(style.NewStyle().Bold(true))
	_ = n.Set("Here is a notification", style.Color("#50FA7B"))

	result := viewString(n)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = style.CellWidth([]byte(line))
	}

	for i := 1; i < len(widths); i++ {
		assert.Equal(t, widths[0], widths[i], "lines have mismatched widths: %v", widths)
	}
}

func TestViewContentHasHorizontalPadding(t *testing.T) {
	t.Parallel()

	n := New(style.Color("#B4B4B4"))
	n.SetBaseStyle(style.NewStyle().Bold(true))
	_ = n.Set("hello", style.Color("#50FA7B"))

	result := viewString(n)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	content := lines[1]

	clean := string(style.StripANSI([]byte(content)))
	assert.True(t, strings.HasPrefix(clean, "│ "), "content line should have padding: %q", clean)
	assert.True(t, strings.HasSuffix(clean, " │"), "content line should have padding: %q", clean)
}
