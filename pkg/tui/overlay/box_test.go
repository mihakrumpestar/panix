package overlay

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// ansiReset is the ANSI reset the style pipeline appends to every line
// rendered with layout properties (borders, padding, width).
const ansiReset = "\x1b[m"

// newBoxCase describes one NewBox rendering expectation.
type newBoxCase struct {
	name      string
	content   []string
	opts      []Option
	want      []string
	wantWidth int
}

func toBytes(lines []string) [][]byte {
	out := make([][]byte, len(lines))

	for i, line := range lines {
		out[i] = []byte(line)
	}

	return out
}

func withReset(lines ...string) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		out[i] = line + ansiReset
	}

	return out
}

func boxStrings(box *Box) []string {
	out := make([]string, box.Len())

	for i := range box.Len() {
		out[i] = string(box.Line(i))
	}

	return out
}

func runNewBoxCases(t *testing.T, cases []newBoxCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			box := NewBox(toBytes(tt.content), tt.opts...)

			assert.Equal(t, tt.want, boxStrings(box), "rendered box lines")
			assert.Equal(t, tt.wantWidth, box.Width(), "box width in cells")
			assert.Equal(t, len(tt.want), box.Len(), "box height in lines")
		})
	}
}

func TestNewBoxLayout(t *testing.T) {
	t.Parallel()

	runNewBoxCases(t, []newBoxCase{
		{
			name:      "no title",
			content:   []string{"a"},
			opts:      []Option{WithWidth(10)},
			want:      withReset("┌────────┐", "│a       │", "└────────┘"),
			wantWidth: 10,
		},
		{
			name:      "empty content renders blank body line",
			opts:      []Option{WithWidth(6)},
			want:      withReset("┌────┐", "│    │", "└────┘"),
			wantWidth: 6,
		},
		{
			name:      "max width truncates content",
			content:   []string{"aaaaaa", "bb"},
			opts:      []Option{WithStyle(style.NewStyle()), WithMaxWidth(4)},
			want:      withReset("aaaa", "bb  "),
			wantWidth: 4,
		},
		{
			name:      "width pads and truncates content",
			content:   []string{"abc", "abcdefghij"},
			opts:      []Option{WithStyle(style.NewStyle()), WithWidth(6)},
			want:      withReset("abc   ", "abcdef"),
			wantWidth: 6,
		},
	})
}

func TestNewBoxTitle(t *testing.T) {
	t.Parallel()

	runNewBoxCases(t, []newBoxCase{
		{
			name:      "bordered box with centered title",
			content:   []string{"a"},
			opts:      []Option{WithWidth(10), WithTitle("T")},
			want:      withReset("┌   T    ┐", "│a       │", "└────────┘"),
			wantWidth: 10,
		},
		{
			name:      "title left aligned",
			content:   []string{"a"},
			opts:      []Option{WithWidth(10), WithTitle("T"), WithTitleAlign(style.Left)},
			want:      withReset("┌ T      ┐", "│a       │", "└────────┘"),
			wantWidth: 10,
		},
		{
			name:      "title right aligned",
			content:   []string{"a"},
			opts:      []Option{WithWidth(10), WithTitle("T"), WithTitleAlign(style.Right)},
			want:      withReset("┌      T ┐", "│a       │", "└────────┘"),
			wantWidth: 10,
		},
		{
			name:      "title truncated to fit explicit width",
			content:   []string{"a"},
			opts:      []Option{WithWidth(10), WithTitle("abcdefgh")},
			want:      withReset("┌ abcd.. ┐", "│a       │", "└────────┘"),
			wantWidth: 10,
		},
		{
			name:      "derived width grows to fit title",
			content:   []string{"a"},
			opts:      []Option{WithTitle("Longer title")},
			want:      withReset("┌ Longer title ┐", "│a             │", "└──────────────┘"),
			wantWidth: 16,
		},
		{
			name:    "title style padding counts toward the measured title width",
			content: []string{"a"},
			opts: []Option{
				WithWidth(10),
				WithTitle("T"),
				WithTitleStyle(style.NewStyle().Padding(0, 1)),
			},
			want:      withReset("┌   T    ┐", "│a       │", "└────────┘"),
			wantWidth: 10,
		},
	})
}

func TestNewBoxStyleVariants(t *testing.T) {
	t.Parallel()

	runNewBoxCases(t, []newBoxCase{
		{
			name:      "plain style draws title as body line",
			content:   []string{"a", "b"},
			opts:      []Option{WithStyle(style.NewStyle()), WithTitle("T")},
			want:      []string{"T", "a", "b"},
			wantWidth: 1,
		},
		{
			name:    "hidden border draws title as body line",
			content: []string{"a"},
			opts: []Option{
				WithStyle(style.NewStyle().Border(style.HiddenBorder()).Width(6)),
				WithTitle("T"),
			},
			want:      withReset("", "T   ", "a   ", ""),
			wantWidth: 4,
		},
	})
}

func TestNewBoxDoesNotMutateContent(t *testing.T) {
	t.Parallel()

	content := toBytes([]string{"a", "bb"})
	snapshot := toBytes([]string{"a", "bb"})

	box := NewBox(content, WithWidth(5), WithTitle("T"))

	assert.Equal(t, snapshot, content, "content lines must not be mutated")

	content[0][0] = 'Z'

	assert.Equal(t, "│a  │"+ansiReset, string(box.Line(1)), "box must not alias caller content")
}
