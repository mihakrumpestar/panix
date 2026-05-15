package flow

import (
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func benchStyles() Styles {
	return makeTestStyles(benchMustHex)
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
	phaseFlow := New().Width(width).Styles(sty)

	names := make([]string, numPhases)
	for i := range numPhases {
		names[i] = "P" + string(rune('A'+i))
	}

	phaseFlow.Phases(names...)

	data := make([]PhaseData, numPhases)
	for idx := range numPhases {
		switch idx % 3 {
		case 0:
			data[idx] = PhaseData{Running: 1, Failed: 0, Done: 0}
		case 1:
			data[idx] = PhaseData{Running: 0, Failed: 1, Done: 2}
		case 2:
			data[idx] = PhaseData{Running: 0, Failed: 0, Done: 3}
		}
	}

	phaseFlow.SetData(data)

	if selected {
		phaseFlow.selectedIndex = numPhases / 2
		phaseFlow.outDirty = true
	}

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		result := phaseFlow.Render()
		buf.AppendFrom(result)
		buf.Reset()
	}
}

func benchRenderCacheHit(b *testing.B, numPhases, width int) {
	b.Helper()

	sty := benchStyles()
	phaseFlow := New().Width(width).Styles(sty)

	names := make([]string, numPhases)
	for i := range numPhases {
		names[i] = "P" + string(rune('A'+i))
	}

	phaseFlow.Phases(names...)

	data := make([]PhaseData, numPhases)
	for i := range numPhases {
		data[i] = PhaseData{Running: 0, Failed: 0, Done: i + 1}
	}

	phaseFlow.SetData(data)

	result := phaseFlow.Render()
	if result.Len() == 0 {
		b.Fatal("expected non-empty output")
	}

	b.ResetTimer()

	for b.Loop() {
		_ = phaseFlow.Render()
	}
}

func benchRenderFullRebuild(b *testing.B, numPhases, width int, selected bool) {
	b.Helper()

	sty := benchStyles()
	phaseFlow := New().Width(width).Styles(sty)

	names := make([]string, numPhases)
	for i := range numPhases {
		names[i] = "P" + string(rune('A'+i))
	}

	phaseFlow.Phases(names...)

	data := make([]PhaseData, numPhases)
	for idx := range numPhases {
		switch idx % 3 {
		case 0:
			data[idx] = PhaseData{Running: 1, Failed: 0, Done: 0}
		case 1:
			data[idx] = PhaseData{Running: 0, Failed: 1, Done: 2}
		case 2:
			data[idx] = PhaseData{Running: 0, Failed: 0, Done: 3}
		}
	}

	if selected {
		phaseFlow.selectedIndex = numPhases / 2
	}

	b.ResetTimer()

	for b.Loop() {
		phaseFlow.outDirty = true
		_ = phaseFlow.Render()
	}
}

func benchMustHex(s string) colorful.Color {
	c, err := colorful.Hex(s)
	if err != nil {
		panic(err)
	}

	return c
}
