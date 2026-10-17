package zeroterm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// probeChunkWriter records each Write and forwards it to dst so a test can act
// as the terminal and observe write boundaries.
type probeChunkWriter struct {
	dst *os.File

	mu     sync.Mutex
	chunks [][]byte
}

func (w *probeChunkWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	w.chunks = append(w.chunks, append([]byte(nil), data...))
	w.mu.Unlock()

	return w.dst.Write(data) //nolint:wrapcheck // transparent test forwarder
}

func (w *probeChunkWriter) all() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()

	var out []byte

	for _, chunk := range w.chunks {
		out = append(out, chunk...)
	}

	return out
}

func (w *probeChunkWriter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return len(w.chunks)
}

// probePipes bundles the fake terminal plumbing of one probe test.
type probePipes struct {
	terminal *Terminal
	requests *os.File
	replies  *os.File
	writer   *probeChunkWriter
}

// assertBalancedSync checks emitted contains exactly one synchronized output
// begin and end, with the begin before the end.
func assertBalancedSync(t *testing.T, emitted []byte) {
	t.Helper()

	text := string(emitted)

	assert.Equal(t, 1, strings.Count(text, probeSyncBegin))
	assert.Equal(t, 1, strings.Count(text, probeSyncEnd))
	assert.Less(t, strings.Index(text, probeSyncBegin), strings.Index(text, probeSyncEnd))
}

// serveWidthProbe acts as the terminal, reading the program's output pipe and
// writing replies to its input pipe. respond returns bytes for a glyph, or nil to
// stay silent.
func serveWidthProbe(t *testing.T, requests, replies *os.File, respond func(glyph string) []byte) {
	t.Helper()

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		var acc []byte

		buf := make([]byte, 1024) //nolint:mnd // test read buffer

		for {
			n, err := requests.Read(buf)
			if n > 0 {
				acc = append(acc, buf[:n]...)
				acc = respondToProbe(acc, respond, replies)
			}

			if err != nil {
				return
			}

			select {
			case <-stop:
				return
			default:
			}
		}
	}()

	t.Cleanup(func() {
		close(stop)

		_ = requests.Close()

		<-done
	})
}

// respondToProbe answers every complete cursor position request in acc.
func respondToProbe(acc []byte, respond func(string) []byte, replies *os.File) []byte {
	for {
		idx := bytes.Index(acc, []byte(probeCPRRequest))
		if idx < 0 {
			return acc
		}

		start := bytes.LastIndexByte(acc[:idx], '\r')
		if start < 0 {
			start = 0
		} else {
			start++
		}

		glyph := string(acc[start:idx])

		if reply := respond(glyph); reply != nil {
			_, _ = replies.Write(reply)
		}

		acc = acc[idx+len(probeCPRRequest):]
	}
}

// newProbePipes wires a Terminal to pipes and returns the request and reply pipes
// plus the probe output recorder.
func newProbePipes(t *testing.T) *probePipes {
	t.Helper()

	inRead, inWrite, err := os.Pipe()
	require.NoError(t, err)

	outRead, outWrite, err := os.Pipe()
	require.NoError(t, err)

	writer := &probeChunkWriter{dst: outWrite}

	t.Cleanup(func() {
		_ = inRead.Close()
		_ = inWrite.Close()
		_ = outRead.Close()
		_ = outWrite.Close()
	})

	return &probePipes{
		terminal: &Terminal{in: inRead, out: outWrite, probeOut: writer},
		requests: outRead,
		replies:  inWrite,
		writer:   writer,
	}
}

// widthResponder answers cursor position reports from a width table.
func widthResponder(widths map[string]int) func(glyph string) []byte {
	return func(glyph string) []byte {
		width, ok := widths[glyph]
		if !ok {
			width = 1
		}

		return fmt.Appendf(nil, "\x1b[1;%dR", width+1)
	}
}

//nolint:paralleltest // exercises the process-global probe over pipes
func TestProbeGlyphWidths_BatchedSingleWrite(t *testing.T) {
	pipes := newProbePipes(t)

	widths := map[string]int{"a": 1, "\u6f22": 2, "🪜": 1, "📋": 2}

	serveWidthProbe(t, pipes.requests, pipes.replies, widthResponder(widths))

	measured, leftover, err := pipes.terminal.probeGlyphWidths([]string{"🪜", "📋", "abc", ""})
	require.NoError(t, err)
	assert.Empty(t, leftover)
	assert.Equal(t, 1, measured["🪜"])
	assert.Equal(t, 2, measured["📋"])
	assert.NotContains(t, measured, "abc", "ASCII glyphs are skipped")

	assert.Equal(t, 1, pipes.writer.count(), "the whole probe must be issued in one write")

	emitted := pipes.writer.all()
	assertBalancedSync(t, emitted)
	assert.Contains(t, string(emitted), "\r🪜"+probeCPRRequest)
	assert.Contains(t, string(emitted), "\r📋"+probeCPRRequest)
}

//nolint:paralleltest // exercises the process-global probe over pipes
func TestProbeGlyphWidths_ControlGateAborts(t *testing.T) {
	pipes := newProbePipes(t)

	serveWidthProbe(t, pipes.requests, pipes.replies, func(string) []byte {
		return []byte("\x1b[1;5R") // every glyph measures four cells: the gate fails
	})

	measured, _, err := pipes.terminal.probeGlyphWidths([]string{"🪜"})
	require.Error(t, err)
	assert.Empty(t, measured, "a failed control gate yields no measurements")
}

//nolint:paralleltest // exercises the process-global probe over pipes
func TestProbeGlyphWidths_PartialReplies(t *testing.T) {
	pipes := newProbePipes(t)

	widths := map[string]int{"a": 1, "\u6f22": 2, "🪜": 1, "📋": 2}
	answered := map[string]bool{"a": true, "\u6f22": true, "🪜": true}

	serveWidthProbe(t, pipes.requests, pipes.replies, func(glyph string) []byte {
		if !answered[glyph] {
			return nil // stay silent: this glyph's reply misses the budget
		}

		return widthResponder(widths)(glyph)
	})

	measured, _, err := pipes.terminal.probeGlyphWidths([]string{"🪜", "📋"})
	require.NoError(t, err)
	assert.Equal(t, 1, measured["🪜"])
	assert.NotContains(t, measured, "📋", "a glyph whose reply missed the budget stays unmeasured")
	assertBalancedSync(t, pipes.writer.all())
}

//nolint:paralleltest // exercises the process-global probe over pipes
func TestProbeGlyphWidths_SilentTerminal(t *testing.T) {
	pipes := newProbePipes(t)

	serveWidthProbe(t, pipes.requests, pipes.replies, func(string) []byte { return nil })

	start := time.Now()

	measured, _, err := pipes.terminal.probeGlyphWidths([]string{"🪜"})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Empty(t, measured)
	assert.Less(t, elapsed, 2*probeBudget, "a silent terminal must not hang the probe")
	assertBalancedSync(t, pipes.writer.all())
}

//nolint:paralleltest // exercises the process-global probe over pipes
func TestProbeGlyphWidths_ReturnsLeftoverBytes(t *testing.T) {
	pipes := newProbePipes(t)

	widths := map[string]int{"a": 1, "\u6f22": 2}

	serveWidthProbe(t, pipes.requests, pipes.replies, func(glyph string) []byte {
		return append([]byte("typed"), widthResponder(widths)(glyph)...)
	})

	measured, leftover, err := pipes.terminal.probeGlyphWidths([]string{})
	require.NoError(t, err)
	assert.Empty(t, measured)
	assert.Equal(t, "typedtyped", string(leftover), "bytes around the replies are handed back")
}

//nolint:paralleltest // exercises the process-global probe over pipes
func TestProbeGlyphWidths_NonTerminalErrors(t *testing.T) {
	pipes := newProbePipes(t)

	_, _, err := pipes.terminal.ProbeGlyphWidths([]string{"🪜"})
	require.ErrorIs(t, err, ErrProbeNotTerminal)
}

//nolint:paralleltest // exercises the process-global probe over pipes
func TestProbeGlyphWidths_NilTerminalErrors(t *testing.T) {
	var terminal *Terminal

	_, _, err := terminal.ProbeGlyphWidths([]string{"🪜"})
	require.ErrorIs(t, err, ErrProbeNotTerminal)
}

func TestReadProbeRepliesTrimsPartialEscape(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "probe-replies")
	require.NoError(t, err)

	t.Cleanup(func() { _ = file.Close() })

	_, err = file.WriteString("\x1b[1;2R\x1b[1;")
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	terminal := &Terminal{in: file}

	cols, leftover := terminal.readProbeReplies(1)

	assert.Equal(t, []int{2}, cols)
	assert.Empty(t, leftover, "a partial reply must not be replayed as input")
}

func TestTrimIncompleteEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want string
	}{
		{"no escape", "typed", "typed"},
		{"complete cpr", "typed\x1b[1;5R", "typed\x1b[1;5R"},
		{"partial cpr", "typed\x1b[1;", "typed"},
		{"lone esc", "typed\x1b", "typed"},
		{"partial csi", "typed\x1b[", "typed"},
		{"partial ss3", "typed\x1bO", "typed"},
		{"complete ss3", "typed\x1bOP", "typed\x1bOP"},
		{"complete sgr mouse", "typed\x1b[<0;1;2M", "typed\x1b[<0;1;2M"},
		{"complete x10 mouse", "\x1b[Mabc", "\x1b[Mabc"},
		{"partial x10 mouse", "typed\x1b[Ma", "typed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, string(trimIncompleteEscape([]byte(test.data))))
		})
	}
}

func TestBuildProbeScript(t *testing.T) {
	t.Parallel()

	script := buildProbeScript([]string{"a", "🪜"})

	assert.True(t, strings.HasPrefix(script, probeSyncBegin), "the batch opens with the synchronized output begin")
	assert.True(t, strings.HasSuffix(script, probeSyncEnd+probeEraseLine), "the batch closes the pair and clears")
	assert.Contains(t, script, "\ra"+probeCPRRequest)
	assert.Contains(t, script, "\r🪜"+probeCPRRequest)
	assertBalancedSync(t, []byte(script))
}

func TestProbeTargets(t *testing.T) {
	t.Parallel()

	targets := probeTargets([]string{"", "abc", "🪜", "🪜", "\u6f22", "📋"})

	assert.Equal(t, []string{"a", "\u6f22", "🪜", "📋"}, targets)
}

func TestParseCPR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		data       string
		wantCol    int
		wantBefore string
		wantAfter  string
		wantOK     bool
	}{
		{"plain", "\x1b[1;5R", 5, "", "", true},
		{"before and after", "hi\x1b[12;34Ryo", 34, "hi", "yo", true},
		{"missing col", "\x1b[1;R", 0, "", "", false},
		{"missing row", "\x1b[;2R", 0, "", "", false},
		{"no final", "\x1b[1;2", 0, "", "", false},
		{"no params", "\x1b[R", 0, "", "", false},
		{"empty", "", 0, "", "", false},
		{"not a report", "\x1b[2J", 0, "", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			col, before, after, ok := parseCPR([]byte(test.data))
			require.Equal(t, test.wantOK, ok)

			if !ok {
				return
			}

			assert.Equal(t, test.wantCol, col)
			assert.Equal(t, test.wantBefore, string(before))
			assert.Equal(t, test.wantAfter, string(after))
		})
	}
}

func TestIsASCIIGlyph(t *testing.T) {
	t.Parallel()

	assert.True(t, isASCIIGlyph(""))
	assert.True(t, isASCIIGlyph("abc"))
	assert.False(t, isASCIIGlyph("🪜"))
	assert.False(t, isASCIIGlyph("a\u6f22"))
}
