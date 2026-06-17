package buffer

import "testing"

func Benchmark__Lines(b *testing.B) {
	buf := NewLinesBuf()
	for i := range 100 {
		buf.WriteLine([]byte("line of text with some content that is reasonably long for testing purposes " + string(rune('A'+i%26))))
	}

	b.ResetTimer()

	for b.Loop() {
		lines := buf.Lines()
		_ = lines
	}
}

func Benchmark__Line(b *testing.B) {
	buf := NewLinesBuf()
	for i := range 100 {
		buf.WriteLine([]byte("line of text with some content that is reasonably long for testing purposes " + string(rune('A'+i%26))))
	}

	b.ResetTimer()

	for b.Loop() {
		for i := range buf.Len() {
			line := buf.Line(i)
			_ = line
		}
	}
}

func Benchmark__LineSmall(b *testing.B) {
	buf := NewLinesBuf()
	for i := range 10 {
		buf.WriteLine([]byte("short line " + string(rune('A'+i%26))))
	}

	b.ResetTimer()

	for b.Loop() {
		for i := range buf.Len() {
			line := buf.Line(i)
			_ = line
		}
	}
}
