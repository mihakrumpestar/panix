package viewport

import (
	"fmt"
	"testing"

	bubbles "charm.land/bubbles/v2/viewport"
)

// makeLines creates n ASCII-only lines (the common case).
func makeLines(n int) []string {
	lines := make([]string, n)
	for i := range n {
		lines[i] = "line of text with some content that is reasonably long for testing purposes " + string(rune('A'+i%26))
	}

	return lines
}

// makeANSILines creates n lines where every 3rd line has ANSI escape codes
// and every 10th line has wide Unicode characters, matching real TUI output.
func makeANSILines(n int) []string {
	lines := make([]string, n)
	for idx := range n {
		switch {
		case idx%10 == 0:
			lines[idx] = "\x1b[1;34msrc/\x1b[0m \x1b[32mwide-unicode-here\x1b[0m package with a longer description"
		case idx%3 == 0:
			lines[idx] = fmt.Sprintf("\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix", idx%6, idx)
		default:
			lines[idx] = fmt.Sprintf("line %d: plain text with some content that is reasonably long for testing", idx)
		}
	}

	return lines
}

// --- New (viewport creation) ---

func Benchmark__New(b *testing.B) {
	for b.Loop() {
		_ = New(WithWidth(80), WithHeight(24))
	}
}

func BenchmarkRef_Bubbles__New(b *testing.B) {
	for b.Loop() {
		_ = bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	}
}

// --- New + SetContentLines (typical usage) ---

func Benchmark__NewAndSetContentLines(b *testing.B) {
	lines := makeLines(1000)

	b.ResetTimer()

	for b.Loop() {
		mdl := New(WithWidth(80), WithHeight(24))
		mdl.SetContentLines(lines)
	}
}

func BenchmarkRef_Bubbles__NewAndSetContentLines(b *testing.B) {
	lines := makeLines(1000)

	b.ResetTimer()

	for b.Loop() {
		mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
		mdl.SetContentLines(lines)
	}
}

// --- SetContentLines (reuse viewport) ---

func Benchmark__SetContentLines(b *testing.B) {
	lines := makeLines(1000)
	mdl := New(WithWidth(80), WithHeight(24))

	b.ResetTimer()

	for b.Loop() {
		mdl.SetContentLines(lines)
	}
}

func BenchmarkRef_Bubbles__SetContentLines(b *testing.B) {
	lines := makeLines(1000)
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))

	b.ResetTimer()

	for b.Loop() {
		mdl.SetContentLines(lines)
	}
}

// --- View (large, 1000 lines) ---

func Benchmark__View(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeLines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__View(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeLines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func Benchmark__ViewANSI(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__ViewANSI(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func Benchmark__ViewANSIScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__ViewANSIScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

func Benchmark__ViewScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeLines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__ViewScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeLines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

// --- View (small, 50 lines) ---

func Benchmark__ViewSmall(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeLines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__ViewSmall(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeLines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func Benchmark__ViewSmallANSI(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__ViewSmallANSI(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func Benchmark__ViewSmallANSIScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__ViewSmallANSIScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}

func Benchmark__ViewSmallScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeLines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}

func BenchmarkRef_Bubbles__ViewSmallScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeLines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}
