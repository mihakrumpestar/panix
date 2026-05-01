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

// --- SetContentLines ---

func BenchmarkSetContentLines(b *testing.B) {
	lines := makeLines(1000)

	b.ResetTimer()

	for b.Loop() {
		mdl := New(WithWidth(80), WithHeight(24))
		mdl.SetContentLines(lines)
	}
}

func BenchmarkBubblesSetContentLines(b *testing.B) {
	lines := makeLines(1000)

	b.ResetTimer()

	for b.Loop() {
		mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
		mdl.SetContentLines(lines)
	}
}

// --- View (large, 1000 lines) ---

func BenchmarkView(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeLines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkBubblesView(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeLines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkViewANSI(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkBubblesViewANSI(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkViewANSIScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

func BenchmarkBubblesViewANSIScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeANSILines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

func BenchmarkViewScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeLines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

func BenchmarkBubblesViewScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeLines(1000))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 977)
		_ = mdl.View()
	}
}

// --- View (small, 50 lines) ---

func BenchmarkViewSmall(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeLines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkBubblesViewSmall(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeLines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkViewSmallANSI(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkBubblesViewSmallANSI(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkViewSmallANSIScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}

func BenchmarkBubblesViewSmallANSIScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeANSILines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}

func BenchmarkViewSmallScroll(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeLines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}

func BenchmarkBubblesViewSmallScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeLines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}
