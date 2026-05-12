package viewport

import (
	"fmt"
	"strings"
	"testing"

	bubbles "charm.land/bubbles/v2/viewport"
	"codeberg.org/tslocum/cview"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

func makeStringLines(n int) []string {
	lines := make([]string, n)
	for i := range n {
		lines[i] = "line of text with some content that is reasonably long for testing purposes " + string(rune('A'+i%26))
	}

	return lines
}

func makeLines(n int) [][]byte {
	lines := make([][]byte, n)
	for i := range n {
		lines[i] = []byte("line of text with some content that is reasonably long for testing purposes " + string(rune('A'+i%26)))
	}

	return lines
}

// makeANSILines creates n lines where every 3rd line has ANSI escape codes
// and every 10th line has wide Unicode characters, matching real TUI output.
func makeStringANSILines(n int) []string {
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

func makeANSILines(n int) [][]byte {
	lines := make([][]byte, n)
	for idx := range n {
		switch {
		case idx%10 == 0:
			lines[idx] = []byte("\x1b[1;34msrc/\x1b[0m \x1b[32mwide-unicode-here\x1b[0m package with a longer description")
		case idx%3 == 0:
			lines[idx] = fmt.Appendf(nil, "\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix", idx%6, idx)
		default:
			lines[idx] = fmt.Appendf(nil, "line %d: plain text with some content that is reasonably long for testing", idx)
		}
	}

	return lines
}

// cview color names matching ANSI 30-35 range used in makeANSILines.
var cviewColorNames = []string{"black", "red", "green", "yellow", "blue", "magenta"}

// makeCviewColorLines creates n lines using cview's [color]text[-] tag format,
// matching the visual intent of makeANSILines: periodic styled source lines,
// colored text every 3rd line, and plain text otherwise.
func makeCviewColorLines(n int) []string {
	lines := make([]string, n)
	for idx := range n {
		switch {
		case idx%10 == 0:
			lines[idx] = "[blue::b]src/[-] [green]wide-unicode-here[-] package with a longer description"
		case idx%3 == 0:
			lines[idx] = fmt.Sprintf("[%s]line %d: colored text with escape sequences[-] and plain suffix", cviewColorNames[idx%6], idx)
		default:
			lines[idx] = fmt.Sprintf("line %d: plain text with some content that is reasonably long for testing", idx)
		}
	}

	return lines
}

// mustInitScreen creates an initialized tcell Screen via a mock terminal
// for benchmarking cview's Draw method. Panics on failure (test-only helper).
func mustInitScreen(height int) tcell.Screen {
	mt := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(80), Y: vt.Row(height)})

	scr, err := tcell.NewTerminfoScreenFromTty(mt)
	if err != nil {
		panic(fmt.Sprintf("create screen from mock tty: %v", err))
	}

	err = scr.Init()
	if err != nil {
		panic(fmt.Sprintf("init screen: %v", err))
	}

	return scr
}

// newCviewTV creates a cview TextView sized to width×height with scrollbar
// disabled (matching our viewport benchmarks which don't use scrollbars).
func newCviewTV(height int) *cview.TextView {
	textView := cview.NewTextView()
	textView.SetRect(0, 0, 80, height)
	textView.SetScrollBarVisibility(cview.ScrollBarNever)

	return textView
}

// --- New (viewport creation) ---

func Benchmark__New(b *testing.B) {
	for b.Loop() {
		_ = New(WithWidth(80), WithHeight(24))
	}
}

func Benchmark_Bubbles__New(b *testing.B) {
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

func Benchmark_Bubbles__NewAndSetContentLines(b *testing.B) {
	lines := makeStringLines(1000)

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

func Benchmark_Bubbles__SetContentLines(b *testing.B) {
	lines := makeStringLines(1000)
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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__View(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeStringLines(1000))
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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__ViewANSI(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeStringANSILines(1000))
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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__ViewANSIScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeStringANSILines(1000))

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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__ViewScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(24))
	mdl.SetContentLines(makeStringLines(1000))

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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__ViewSmall(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeStringLines(50))
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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__ViewSmallANSI(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeStringANSILines(50))
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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__ViewSmallANSIScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeStringANSILines(50))

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
		_ = mdl.Render()
	}
}

func Benchmark_Bubbles__ViewSmallScroll(b *testing.B) {
	mdl := bubbles.New(bubbles.WithWidth(80), bubbles.WithHeight(8))
	mdl.SetContentLines(makeStringLines(50))

	b.ResetTimer()

	for i := range b.N {
		mdl.SetYOffset(i % 43)
		_ = mdl.View()
	}
}

// --- Content append (most common TUI pattern: logs grow) ---

func Benchmark__SetContentLines_Append(b *testing.B) {
	base := makeANSILines(500)
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(base)

	b.ResetTimer()

	for i := range b.N {
		appended := make([][]byte, len(base)+5)
		copy(appended, base)

		for j := range 5 {
			appended[len(base)+j] = fmt.Appendf(nil, "\x1b[32mnew line %d\x1b[0m appended content here", i*5+j)
		}

		mdl.SetContentLines(appended)
		base = appended
	}
}

func Benchmark__SetContentLines_Append_ThenView(b *testing.B) {
	base := makeANSILines(500)
	mdl := New(WithWidth(80), WithHeight(24))
	mdl.SetContentLines(base)
	_ = mdl.Render()

	b.ResetTimer()

	for i := range b.N {
		appended := make([][]byte, len(base)+5)
		copy(appended, base)

		for j := range 5 {
			appended[len(base)+j] = fmt.Appendf(nil, "\x1b[32mnew line %d\x1b[0m appended content here", i*5+j)
		}

		mdl.SetContentLines(appended)
		_ = mdl.Render()
		base = appended
	}
}

func Benchmark__SetContentLines_FullReplace(b *testing.B) {
	mdl := New(WithWidth(80), WithHeight(24))
	lines := makeANSILines(500)
	mdl.SetContentLines(lines)

	b.ResetTimer()

	for range b.N {
		newLines := makeANSILines(500)
		mdl.SetContentLines(newLines)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// cview benchmarks
//
// cview.TextView renders to a tcell.Screen via Draw() (cell-by-cell with
// tcell.Style), while our viewport and Bubbles return strings via View().
// The benchmarks compare equivalent high-level operations (create, set content,
// render a viewport's worth of content, scroll, append) using each library's
// idiomatic API. cview uses its own [color]tag[-] format instead of ANSI
// escape codes, so the "ANSI" benchmarks use cview's dynamic color tags with
// SetDynamicColors(true). Scrollbar is disabled on cview (SetScrollBarNever)
// to match our viewport benchmarks which don't use scrollbars.
// ────────────────────────────────────────────────────────────────────────────

// --- Cview New ---

func Benchmark_Cview__New(b *testing.B) {
	for b.Loop() {
		_ = newCviewTV(24)
	}
}

// --- Cview New + SetContentLines (typical usage) ---

func Benchmark_Cview__NewAndSetContentLines(b *testing.B) {
	lines := makeStringLines(1000)
	text := strings.Join(lines, "\n")

	b.ResetTimer()

	for b.Loop() {
		textView := newCviewTV(24)
		textView.SetText(text)
	}
}

// --- Cview SetContentLines (reuse viewport) ---

func Benchmark_Cview__SetContentLines(b *testing.B) {
	lines := makeStringLines(1000)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(24)

	b.ResetTimer()

	for b.Loop() {
		textView.SetText(text)
	}
}

// --- Cview View (large, 1000 lines) ---

func Benchmark_Cview__View(b *testing.B) {
	lines := makeStringLines(1000)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(24)
	textView.SetText(text)
	textView.ScrollTo(500, 0)

	screen := mustInitScreen(24)

	b.ResetTimer()

	for b.Loop() {
		textView.Draw(screen)
	}
}

func Benchmark_Cview__ViewANSI(b *testing.B) {
	lines := makeCviewColorLines(1000)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(24)
	textView.SetDynamicColors(true)
	textView.SetText(text)
	textView.ScrollTo(500, 0)

	screen := mustInitScreen(24)

	b.ResetTimer()

	for b.Loop() {
		textView.Draw(screen)
	}
}

func Benchmark_Cview__ViewANSIScroll(b *testing.B) {
	lines := makeCviewColorLines(1000)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(24)
	textView.SetDynamicColors(true)
	textView.SetText(text)

	screen := mustInitScreen(24)

	b.ResetTimer()

	for i := range b.N {
		textView.ScrollTo(i%977, 0)
		textView.Draw(screen)
	}
}

func Benchmark_Cview__ViewScroll(b *testing.B) {
	lines := makeStringLines(1000)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(24)
	textView.SetText(text)

	screen := mustInitScreen(24)

	b.ResetTimer()

	for i := range b.N {
		textView.ScrollTo(i%977, 0)
		textView.Draw(screen)
	}
}

// --- Cview View (small, 50 lines) ---

func Benchmark_Cview__ViewSmall(b *testing.B) {
	lines := makeStringLines(50)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(8)
	textView.SetText(text)
	textView.ScrollTo(20, 0)

	screen := mustInitScreen(8)

	b.ResetTimer()

	for b.Loop() {
		textView.Draw(screen)
	}
}

func Benchmark_Cview__ViewSmallANSI(b *testing.B) {
	lines := makeCviewColorLines(50)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(8)
	textView.SetDynamicColors(true)
	textView.SetText(text)
	textView.ScrollTo(20, 0)

	screen := mustInitScreen(8)

	b.ResetTimer()

	for b.Loop() {
		textView.Draw(screen)
	}
}

func Benchmark_Cview__ViewSmallANSIScroll(b *testing.B) {
	lines := makeCviewColorLines(50)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(8)
	textView.SetDynamicColors(true)
	textView.SetText(text)

	screen := mustInitScreen(8)

	b.ResetTimer()

	for i := range b.N {
		textView.ScrollTo(i%43, 0)
		textView.Draw(screen)
	}
}

func Benchmark_Cview__ViewSmallScroll(b *testing.B) {
	lines := makeStringLines(50)
	text := strings.Join(lines, "\n")
	textView := newCviewTV(8)
	textView.SetText(text)

	screen := mustInitScreen(8)

	b.ResetTimer()

	for i := range b.N {
		textView.ScrollTo(i%43, 0)
		textView.Draw(screen)
	}
}

// --- Cview Content append (most common TUI pattern: logs grow) ---
//
// cview implements io.Writer on TextView, so Write() is the idiomatic way
// to append content incrementally — each Write adds to the internal buffer
// without replacing existing content, unlike our SetContentLines which takes
// the full slice. Both represent the "content grows" use case.

func Benchmark_Cview__SetContentLines_Append(b *testing.B) {
	base := makeCviewColorLines(500)
	text := strings.Join(base, "\n")
	textView := newCviewTV(24)
	textView.SetDynamicColors(true)
	textView.SetText(text)

	b.ResetTimer()

	for i := range b.N {
		for j := range 5 {
			_, _ = fmt.Fprintf(textView, "\n[green]new line %d[-] appended content here", i*5+j)
		}
	}
}

func Benchmark_Cview__SetContentLines_Append_ThenView(b *testing.B) {
	base := makeCviewColorLines(500)
	text := strings.Join(base, "\n")
	textView := newCviewTV(24)
	textView.SetDynamicColors(true)
	textView.SetText(text)

	screen := mustInitScreen(24)
	textView.Draw(screen) // Build initial index

	b.ResetTimer()

	for i := range b.N {
		for j := range 5 {
			_, _ = fmt.Fprintf(textView, "\n[green]new line %d[-] appended content here", i*5+j)
		}

		textView.Draw(screen)
	}
}

func Benchmark_Cview__SetContentLines_FullReplace(b *testing.B) {
	textView := newCviewTV(24)
	textView.SetDynamicColors(true)
	textView.SetText(strings.Join(makeCviewColorLines(500), "\n"))

	b.ResetTimer()

	for range b.N {
		textView.SetText(strings.Join(makeCviewColorLines(500), "\n"))
	}
}
