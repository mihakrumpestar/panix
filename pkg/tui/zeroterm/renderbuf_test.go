package zeroterm

import (
	"testing"
)

func TestRenderBufferReset(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 10)

	renderBuf.WriteLine([]byte("line1"))
	renderBuf.WriteLine([]byte("line2"))

	if len(renderBuf.lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(renderBuf.lines))
	}

	renderBuf.Reset()

	if len(renderBuf.lines) != 0 {
		t.Errorf("Reset should clear lines, got %d", len(renderBuf.lines))
	}

	if cap(renderBuf.lines) < 2 {
		t.Errorf("Reset should preserve capacity, got cap=%d", cap(renderBuf.lines))
	}
}

func TestRenderBufferWriteLine(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 10)

	renderBuf.WriteLine([]byte("first"))
	renderBuf.WriteLine([]byte("second"))

	lines := renderBuf.Lines()

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	if string(lines[0]) != "first" {
		t.Errorf("line[0] = %q, want %q", string(lines[0]), "first")
	}

	if string(lines[1]) != "second" {
		t.Errorf("line[1] = %q, want %q", string(lines[1]), "second")
	}
}

func TestRenderBufferWriteLineReuse(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 10)

	renderBuf.WriteLine([]byte("initial"))
	line1Cap := cap(renderBuf.lines[0])

	renderBuf.Reset()
	renderBuf.WriteLine([]byte("new"))

	if len(renderBuf.lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(renderBuf.lines))
	}

	if string(renderBuf.lines[0]) != "new" {
		t.Errorf("line = %q, want %q", string(renderBuf.lines[0]), "new")
	}

	if cap(renderBuf.lines[0]) < line1Cap {
		t.Errorf("should reuse buffer with cap >= %d, got %d", line1Cap, cap(renderBuf.lines[0]))
	}
}

func TestRenderBufferWriteString(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 10)

	renderBuf.WriteString("line1\nline2\nline3")

	lines := renderBuf.Lines()

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	expected := []string{"line1", "line2", "line3"}
	for idx, want := range expected {
		if string(lines[idx]) != want {
			t.Errorf("line[%d] = %q, want %q", idx, string(lines[idx]), want)
		}
	}
}

func TestRenderBufferWriteStringEmpty(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.WriteString("")

	if len(renderBuf.lines) != 0 {
		t.Errorf("empty string should produce 0 lines, got %d", len(renderBuf.lines))
	}
}

func TestRenderBufferWriteStringSingleLine(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.WriteString("single")

	if len(renderBuf.lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(renderBuf.lines))
	}

	if string(renderBuf.lines[0]) != "single" {
		t.Errorf("line = %q, want %q", string(renderBuf.lines[0]), "single")
	}
}

func TestRenderBufferLines(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 10)

	renderBuf.WriteLine([]byte("test"))

	lines := renderBuf.Lines()

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	if string(lines[0]) != "test" {
		t.Errorf("line = %q, want %q", string(lines[0]), "test")
	}
}

func TestRenderBufferANSIContent(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 10)

	renderBuf.WriteString("\x1b[1;34mblue\x1b[0m\n\x1b[32mgreen\x1b[0m")

	lines := renderBuf.Lines()

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	if string(lines[0]) != "\x1b[1;34mblue\x1b[0m" {
		t.Errorf("line[0] = %q", string(lines[0]))
	}

	if string(lines[1]) != "\x1b[32mgreen\x1b[0m" {
		t.Errorf("line[1] = %q", string(lines[1]))
	}
}

func TestRenderBufferMultipleReset(t *testing.T) {
	t.Parallel()

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 10)

	for iter := range 5 {
		renderBuf.Reset()
		renderBuf.WriteString("iteration")
		lines := renderBuf.Lines()

		if len(lines) != 1 {
			t.Errorf("iteration %d: expected 1 line, got %d", iter, len(lines))
		}

		if string(lines[0]) != "iteration" {
			t.Errorf("iteration %d: line = %q, want %q", iter, string(lines[0]), "iteration")
		}
	}
}
