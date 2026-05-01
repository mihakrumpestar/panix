package viewport

import (
	"testing"

	bubbles "charm.land/bubbles/v2/viewport"
)

func makeLines(n int) []string {
	lines := make([]string, n)
	for i := range n {
		lines[i] = "line of text with some content that is reasonably long for testing purposes " + string(rune('A'+i%26))
	}

	return lines
}

func BenchmarkSetContentLines(b *testing.B) {
	lines := makeLines(1000)

	b.ResetTimer()

	for b.Loop() {
		mdl := New(WithWidth(80), WithHeight(24))
		mdl.SetContentLines(lines)
	}
}

func BenchmarkView(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(makeLines(1000))
	mdl.SetYOffset(500)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
	}
}

func BenchmarkViewSmall(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(8))
	mdl.SetContentLines(makeLines(50))
	mdl.SetYOffset(20)

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

func BenchmarkBubblesViewSmall(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeLines(50))
	mdl.SetYOffset(20)

	b.ResetTimer()

	for b.Loop() {
		_ = mdl.View()
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
