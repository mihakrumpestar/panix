package flow

import (
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestStyles(hexFn func(string) colorful.Color) Styles {
	white := style.NewStyle().Foreground(style.Color("#FFFFFF")).Bold(true).Padding(0, 1)

	return Styles{
		GradientRunning: GradientPair{Dark: hexFn("#01536e"), Light: hexFn("#007da7")},
		GradientFailed:  GradientPair{Dark: hexFn("#5f1414"), Light: hexFn("#DC2626")},
		GradientDone:    GradientPair{Dark: hexFn("#14532D"), Light: hexFn("#11883d")},
		GradientDefault: GradientPair{Dark: hexFn("#535862"), Light: hexFn("#6B7280")},
		Pill:            white,
		Status: StatusStyles{
			Running: style.NewStyle().Foreground(style.Color("#00BFFF")),
			Failed:  style.NewStyle().Foreground(style.Color("#FF5555")),
			Done:    style.NewStyle().Foreground(style.Color("#50FA7B")),
		},
		StatusSeparator: style.NewStyle().Foreground(style.Color("#6272A4")),
		Arrow:           style.NewStyle().Foreground(style.Color("#6272A4")),
		PhaseArrow:      []byte(string(rune(0x1FB74))),
		SelectionBg:     style.Color("#3B3258"),
	}
}

func TestDetermineState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		data PhaseData
		want PhaseState
		name string
	}{
		{PhaseData{}, Idle, "empty"},
		{PhaseData{Running: 1}, StateRunning, "running"},
		{PhaseData{Failed: 1}, StateFailed, "failed"},
		{PhaseData{Done: 1}, StateDone, "done"},
		{PhaseData{Running: 1, Failed: 1}, StateRunning, "running_takes_priority"},
		{PhaseData{Failed: 1, Done: 1}, StateFailed, "failed_over_done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, determineState(tt.data))
		})
	}
}

// testStyles builds the flow styles with a concrete arrow glyph so tests can
// pin the arrow column width.
func testStyles(arrow []byte) Styles {
	styles := makeTestStyles(func(s string) colorful.Color {
		c, _ := colorful.Hex(s)

		return c
	})
	styles.PhaseArrow = arrow

	return styles
}

// TestRender_ArrowWidth pins that arrow column width comes from the theme glyph
// and phase columns still fill the requested width exactly.
func TestRender_ArrowWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		arrow []byte
		want  int
	}{
		{name: "one cell arrow", arrow: []byte("→"), want: 1},
		{name: "two cell arrow", arrow: []byte("⏩"), want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, style.CellWidth(tt.arrow), "test arrow must be %d cells", tt.want)

			const width = 60

			pf := New().Width(width).Phases("A", "B", "C").ApplyTheme(testStyles(tt.arrow))
			pf.SetData([]PhaseData{{Done: 1}, {Done: 1}, {Done: 1}})

			out := pf.Render()
			require.NotZero(t, out.Len(), "rendered flow must not be empty")

			for i := range out.Len() {
				assert.Equal(t, width, style.CellWidth(out.Line(i)), "line %d must fill the width", i)
			}

			assert.Contains(t, string(out.Line(0)), string(tt.arrow),
				"a %d-cell arrow must render without truncation", tt.want)
		})
	}
}

// TestRender_TooNarrow pins that an unfittable width renders nothing instead of
// overflowing or dividing by zero.
func TestRender_TooNarrow(t *testing.T) {
	t.Parallel()

	phaseFlow := New().Width(1).Phases("A", "B", "C").ApplyTheme(testStyles([]byte("⏩")))
	phaseFlow.SetData([]PhaseData{{Done: 1}, {Done: 1}, {Done: 1}})

	assert.Zero(t, phaseFlow.Render().Len(), "a width too narrow for the arrows must render nothing")
}

// TestRender_CacheHit pins buffer reuse for an unchanged flow.
func TestRender_CacheHit(t *testing.T) {
	t.Parallel()

	phaseFlow := New().Width(60).Phases("A", "B").ApplyTheme(testStyles([]byte("→")))
	phaseFlow.SetData([]PhaseData{{Done: 1}, {Done: 1}})

	first := phaseFlow.Render()
	second := phaseFlow.Render()

	assert.Same(t, first, second, "an unchanged flow must reuse its cached buffer")
}

// TestRenderInto_Appends pins that RenderInto appends rather than overwrites.
func TestRenderInto_Appends(t *testing.T) {
	t.Parallel()

	phaseFlow := New().Width(60).Phases("A", "B").ApplyTheme(testStyles([]byte("→")))
	phaseFlow.SetData([]PhaseData{{Done: 1}, {Done: 1}})

	dst := buffer.NewLinesBuf()
	dst.WriteLine([]byte("prefix"))

	phaseFlow.RenderInto(dst)

	assert.Greater(t, dst.Len(), 1, "RenderInto must append the rendered flow")
}
