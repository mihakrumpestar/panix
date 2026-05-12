package flow

import (
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

func benchStyles() Styles {
	white := style.NewStyle().Foreground(style.Color("#FFFFFF")).Bold(true).Padding(0, 1)

	return Styles{
		GradientRunning: GradientPair{Dark: benchMustHex("#01536e"), Light: benchMustHex("#007da7")},
		GradientFailed:  GradientPair{Dark: benchMustHex("#5f1414"), Light: benchMustHex("#DC2626")},
		GradientDone:    GradientPair{Dark: benchMustHex("#14532D"), Light: benchMustHex("#11883d")},
		GradientDefault: GradientPair{Dark: benchMustHex("#535862"), Light: benchMustHex("#6B7280")},
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

func Benchmark__Render_4Phases(b *testing.B)             { benchRender(b, 4, 80, false) }
func Benchmark__Render_7Phases(b *testing.B)             { benchRender(b, 7, 200, false) }
func Benchmark__Render_7Phases_Selected(b *testing.B)    { benchRender(b, 7, 200, true) }
func Benchmark__Render_CacheHit(b *testing.B)            { benchRenderCacheHit(b, 7, 200) }
func Benchmark__Render_FullRebuild_7Phases(b *testing.B) { benchRenderFullRebuild(b, 7, 200, false) }
func Benchmark__Render_FullRebuild_7Sel(b *testing.B)    { benchRenderFullRebuild(b, 7, 200, true) }

func benchRender(b *testing.B, numPhases, width int, selected bool) {
	b.Helper()

	sty := benchStyles()
	pf := New().Width(width).Styles(sty)

	names := make([]string, numPhases)
	for i := range numPhases {
		names[i] = "P" + string(rune('A'+i))
	}

	pf.Phases(names...)

	data := make([]PhaseData, numPhases)
	for i := range numPhases {
		switch i % 3 {
		case 0:
			data[i] = PhaseData{Running: 1, Failed: 0, Done: 0}
		case 1:
			data[i] = PhaseData{Running: 0, Failed: 1, Done: 2}
		case 2:
			data[i] = PhaseData{Running: 0, Failed: 0, Done: 3}
		}
	}

	pf.SetData(data)

	if selected {
		pf.selectedIndex = numPhases / 2
		pf.outDirty = true
	}

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		result := pf.Render()
		buf.AppendFrom(result)
		buf.Reset()
	}
}

func benchRenderCacheHit(b *testing.B, numPhases, width int) {
	b.Helper()

	sty := benchStyles()
	pf := New().Width(width).Styles(sty)

	names := make([]string, numPhases)
	for i := range numPhases {
		names[i] = "P" + string(rune('A'+i))
	}

	pf.Phases(names...)

	data := make([]PhaseData, numPhases)
	for i := range numPhases {
		data[i] = PhaseData{Running: 0, Failed: 0, Done: i + 1}
	}

	pf.SetData(data)

	result := pf.Render()
	if result.Len() == 0 {
		b.Fatal("expected non-empty output")
	}

	b.ResetTimer()

	for b.Loop() {
		_ = pf.Render()
	}
}

func benchRenderFullRebuild(b *testing.B, numPhases, width int, selected bool) {
	b.Helper()

	sty := benchStyles()
	pf := New().Width(width).Styles(sty)

	names := make([]string, numPhases)
	for i := range numPhases {
		names[i] = "P" + string(rune('A'+i))
	}

	pf.Phases(names...)

	data := make([]PhaseData, numPhases)
	for i := range numPhases {
		switch i % 3 {
		case 0:
			data[i] = PhaseData{Running: 1, Failed: 0, Done: 0}
		case 1:
			data[i] = PhaseData{Running: 0, Failed: 1, Done: 2}
		case 2:
			data[i] = PhaseData{Running: 0, Failed: 0, Done: 3}
		}
	}

	if selected {
		pf.selectedIndex = numPhases / 2
	}

	b.ResetTimer()

	for b.Loop() {
		pf.outDirty = true
		_ = pf.Render()
	}
}

func benchMustHex(s string) colorful.Color {
	c, err := colorful.Hex(s)
	if err != nil {
		panic(err)
	}

	return c
}
