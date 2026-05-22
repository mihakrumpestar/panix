package zeroterm

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
)

// TestShrinkThenGrowWithIdenticalPrefix verifies that when content
// shrinks and then grows back, and some of the short frame's lines
// happen to match the full frame's lines at the same position (index),
// the grow-back frame still renders all lines correctly.
//
// This is the real-world bug: after a workflow restart, the interim
// short frame (empty fleet) may have lines like "=== Stats Table ==="
// that match the full frame's same-position lines. The diff would skip
// them, but the terminal may show different (stale) content at those
// positions from an older frame.
//
// Fix: renderFrame clears prevLines when the frame shrinks, so DiffLines
// reports every line as changed and they all get rewritten.

//nolint:paralleltest // package-level globals not concurrency-safe
func TestShrinkThenGrowWithIdenticalPrefix(t *testing.T) {
	const terminalHeight = 120

	fullFrame := makeIdenticalPrefixFullFrame()
	shortFrame := makeIdenticalPrefixShortFrame()

	prevBuf := buffer.NewLinesBufDiff()

	// Step 1: initial full frame (prevLines empty → all lines are diffs)
	diffs1 := fullFrame.Diff(prevBuf)
	_ = RenderLines(nil, diffs1, fullFrame, 0, terminalHeight)

	// Set prevBuf to fullFrame contents
	prevBuf.Reset()

	for i := range fullFrame.Len() {
		prevBuf.Write(fullFrame.Line(i))
	}

	// Step 2: shrink to short frame
	out2 := renderShrinkStep(t, shortFrame, prevBuf, terminalHeight)
	verifyShrinkOutput(t, out2, shortFrame, fullFrame)

	// Step 3: grow back to full frame
	out3 := renderGrowBackStep(t, fullFrame, prevBuf, terminalHeight)
	verifyGrowBackOutput(t, out3, fullFrame, shortFrame)
}

func makeIdenticalPrefixFullFrame() *buffer.LinesBufDiff {
	buf := buffer.NewLinesBufDiff()
	for _, line := range []string{
		"=== Header ===",
		"",
		"=== Stats Table ===",
		"",
		"row1 | data1",
		"row2 | data2",
		"",
		"=== Phase Status ===",
		"",
		"INSPECT  BUILD  DONE",
		"   1      2      3",
		"",
		"=== Build Logs ===",
		"",
		"flake1 (5.23s)",
		"  machine (2.50s)",
		"===",
	} {
		buf.Write([]byte(line))
	}

	return buf
}

func makeIdenticalPrefixShortFrame() *buffer.LinesBufDiff {
	buf := buffer.NewLinesBufDiff()
	for _, line := range []string{
		"=== Header ===",
		"",
		"=== Stats Table ===",
		"",
		"",
		"",
		"",
		"=== Build Logs ===",
		"",
		"",
		"",
		"",
		"=== Build Logs ===",
		"",
	} {
		buf.Write([]byte(line))
	}

	return buf
}

func renderShrinkStep(t *testing.T, shortFrame *buffer.LinesBufDiff, prevBuf *buffer.LinesBufDiff, terminalHeight int) string {
	t.Helper()

	prevCount2 := prevBuf.Len()
	if shortFrame.Len() < prevCount2 {
		prevBuf.Reset()

		prevCount2 = 0
	}

	diffs2 := shortFrame.Diff(prevBuf)
	buf := RenderLines(nil, diffs2, shortFrame, prevCount2, terminalHeight)
	out2 := string(buf)

	t.Logf("Step 2 shrink: %d lines vs prev %d, %d diffs",
		shortFrame.Len(), prevCount2, len(diffs2))

	// Update prevBuf
	prevBuf.Reset()

	for i := range shortFrame.Len() {
		prevBuf.Write(shortFrame.Line(i))
	}

	return out2
}

func verifyShrinkOutput(t *testing.T, out2 string, shortFrame, fullFrame *buffer.LinesBufDiff) {
	t.Helper()

	for i := range shortFrame.Len() {
		prefix := "\x1b[" + itoa(i+1) + ";1H"
		assert.Contains(t, out2, prefix+string(shortFrame.Line(i)),
			"shrink missing line %d at row %d: %q%q",
			i, i+1, prefix, string(shortFrame.Line(i)))
	}

	for i := shortFrame.Len(); i < fullFrame.Len(); i++ {
		oldPrefix := "\x1b[" + itoa(i+1) + ";1H"

		oldLine := oldPrefix + string(fullFrame.Line(i))
		assert.NotContains(t, out2, oldLine,
			"stale fullFrame[%d]=%q at row %d leaked", i, string(fullFrame.Line(i)), i+1)
	}
}

func renderGrowBackStep(t *testing.T, fullFrame *buffer.LinesBufDiff, prevBuf *buffer.LinesBufDiff, terminalHeight int) string {
	t.Helper()

	prevCount3 := prevBuf.Len()
	if fullFrame.Len() < prevCount3 {
		prevBuf.Reset()

		prevCount3 = 0
	}

	diffs3 := fullFrame.Diff(prevBuf)
	buf := RenderLines(nil, diffs3, fullFrame, prevCount3, terminalHeight)
	out3 := string(buf)

	t.Logf("Step 3 grow-back: %d lines vs prev %d, %d diffs",
		fullFrame.Len(), prevCount3, len(diffs3))

	// Update prevBuf
	prevBuf.Reset()

	for i := range fullFrame.Len() {
		prevBuf.Write(fullFrame.Line(i))
	}

	return out3
}

func verifyGrowBackOutput(t *testing.T, out3 string, fullFrame, shortFrame *buffer.LinesBufDiff) {
	t.Helper()

	for lineIdx := range fullFrame.Len() {
		line := fullFrame.Line(lineIdx)
		if lineIdx < shortFrame.Len() && string(line) == string(shortFrame.Line(lineIdx)) {
			continue
		}

		prefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
		assert.Contains(t, out3, prefix+string(line),
			"grow-back missing line %d at row %d: %q%q",
			lineIdx, lineIdx+1, prefix, string(line))
	}

	for lineIdx := shortFrame.Len(); lineIdx < fullFrame.Len(); lineIdx++ {
		prefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
		assert.Contains(t, out3, prefix+string(fullFrame.Line(lineIdx)),
			"grow-back missing new line %d at row %d: %q%q",
			lineIdx, lineIdx+1, prefix, string(fullFrame.Line(lineIdx)))
	}
}

//nolint:funlen,paralleltest // package-level globals not concurrency-safe
func TestShrinkThenGrow(t *testing.T) {
	const terminalHeight = 120

	fullFrame := toLinesBuffer(
		"F-000", "F-001", "F-002", "F-003", "F-004",
		"F-005", "F-006", "F-007", "F-008", "F-009",
		"F-010", "F-011", "F-012", "F-013", "F-014",
		"F-015", "F-016", "F-017",
	)

	shortFrame := toLinesBuffer(
		"S-000", "S-001", "S-002", "S-003", "S-004",
		"S-005", "S-006", "S-007", "S-008",
	)

	prevBuf := buffer.NewLinesBufDiff()

	diffs1 := fullFrame.Diff(prevBuf)

	var buf []byte

	buf = RenderLines(buf[:0], diffs1, fullFrame, 0, terminalHeight)

	prevBuf.Reset()

	for i := range fullFrame.Len() {
		prevBuf.Write(fullFrame.Line(i))
	}

	diffs2 := shortFrame.Diff(prevBuf)
	buf = buf[:0]
	buf = RenderLines(buf, diffs2, shortFrame, fullFrame.Len(), terminalHeight)
	output2 := string(buf)

	for i := 9; i < fullFrame.Len(); i++ {
		assert.NotContains(t, output2, string(fullFrame.Line(i)),
			"FULL frame[%d]=%q leaked into shrink output", i, string(fullFrame.Line(i)))
	}

	prevBuf.Reset()

	for i := range shortFrame.Len() {
		prevBuf.Write(shortFrame.Line(i))
	}

	diffs3 := fullFrame.Diff(prevBuf)
	buf = buf[:0]
	buf = RenderLines(buf, diffs3, fullFrame, shortFrame.Len(), terminalHeight)
	output3 := string(buf)

	for lineIdx := range fullFrame.Len() {
		assert.Contains(t, output3, string(fullFrame.Line(lineIdx)),
			"Render3 missing fullFrame[%d]=%q", lineIdx, string(fullFrame.Line(lineIdx)))
	}

	for lineIdx := range fullFrame.Len() {
		want := string(fullFrame.Line(lineIdx))

		prefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
		assert.Contains(t, output3, prefix+want,
			"Render3 line %d not positioned at row %d. Expected %q%q in output",
			lineIdx, lineIdx+1, prefix, want)
	}
}

// TestRenderFrameWithANSIContent tests the render pipeline with
// realistic ANSI-styled content that may contain \r characters
// and escape sequences.
//
//nolint:paralleltest // package-level globals not concurrency-safe
func TestRenderFrameWithANSIContent(t *testing.T) {
	const terminalHeight = 120

	ansiFull := ansiFullContent()
	ansiShort := ansiShortContent()

	renderFrameWithANSIContentShrink(t, ansiFull, ansiShort, terminalHeight)
}

func renderFrameWithANSIContentShrink(t *testing.T, ansiFull, ansiShort *buffer.LinesBufDiff, terminalHeight int) {
	t.Helper()

	prevBuf := buffer.NewLinesBufDiff()

	diffs1 := ansiFull.Diff(prevBuf)

	var buf []byte

	buf = RenderLines(buf[:0], diffs1, ansiFull, 0, terminalHeight)

	prevBuf.Reset()

	for i := range ansiFull.Len() {
		prevBuf.Write(ansiFull.Line(i))
	}

	diffs2 := ansiShort.Diff(prevBuf)
	buf = buf[:0]
	buf = RenderLines(buf, diffs2, ansiShort, ansiFull.Len(), terminalHeight)
	t.Logf("ANSI shrink output: %q", string(buf))

	output2 := string(buf)
	assert.Contains(t, output2, "=== Build Logs ===", "changed build logs header missing from shrink output")
	assert.Contains(t, output2, "===", "changed footer missing from shrink output")

	for lineIdx := 9; lineIdx < ansiFull.Len(); lineIdx++ {
		oldPrefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
		assert.NotContains(t, output2, oldPrefix+string(ansiFull.Line(lineIdx)),
			"stale ansi line %d written: prefix %q", lineIdx, oldPrefix+string(ansiFull.Line(lineIdx)))
	}

	prevBuf.Reset()

	for i := range ansiShort.Len() {
		prevBuf.Write(ansiShort.Line(i))
	}

	diffs3 := ansiFull.Diff(prevBuf)
	buf = buf[:0]
	buf = RenderLines(buf, diffs3, ansiFull, ansiShort.Len(), terminalHeight)

	output3 := string(buf)

	for i := range ansiFull.Len() {
		line := ansiFull.Line(i)
		if len(line) > 0 && !strings.Contains(output3, string(line)) {
			if i > 4 || string(ansiShort.Line(i)) != string(line) {
				assert.Contains(t, output3, string(line), "ANSIGrow Render3 missing line %d: %q", i, string(line))
			}
		}
	}
}

func ansiFullContent() *buffer.LinesBufDiff {
	buf := buffer.NewLinesBufDiff()
	for _, line := range []string{
		"\x1b[1;34m=== HEADER ===\x1b[0m",
		"",
		"\x1b[1m=== Stats Table ===\x1b[0m",
		"",
		"\x1b[1m\x1b[2m\x1b[3m\x1b[4m\x1b[5m\x1b[6m\x1b[7m\x1b[8m\x1b[9m\x1b[10m\x1b[11m\x1b[12m",
		"row 1 | value \x1b[31mERROR\x1b[0m",
		"",
		"\x1b[1m=== Phase Status ===\x1b[0m",
		"",
		"INSPECT  BUILD  TRANSFER  ACTIVATE  DONE",
		"   1       2       1         2        3",
		"",
		"\x1b[1m=== Build Logs ===\x1b[0m",
		"",
		"\x1b[33mflake\x1b[0m (5.23s)",
		"  \x1b[32mconfig\x1b[0m",
		"    \x1b[34mmachine\x1b[0m (2.50s)",
		"      INSPECT  (0.50s)",
		"        1 check (0.50s)",
		"===",
	} {
		buf.Write([]byte(line))
	}

	return buf
}

func ansiShortContent() *buffer.LinesBufDiff {
	buf := buffer.NewLinesBufDiff()
	for _, line := range []string{
		"\x1b[1;34m=== HEADER ===\x1b[0m",
		"",
		"\x1b[1m=== Stats Table ===\x1b[0m",
		"",
		"",
		"\x1b[1m=== Build Logs ===\x1b[0m",
		"",
		"",
		"===",
	} {
		buf.Write([]byte(line))
	}

	return buf
}

// TestDiffLinesSameLengthDifferentContent ensures that when frames have
// the same length but different content on every line, all lines are in diffs.
//
//nolint:paralleltest // package-level globals not concurrency-safe
func TestDiffLinesSameLengthDifferentContent(t *testing.T) {
	a := toLinesBuffer("A", "B", "C", "D", "E")
	b := toLinesBuffer("X", "Y", "Z", "W", "V")

	diffs := b.Diff(a)
	assert.Len(t, diffs, 5)
}

// TestDiffLinesPartialIdentical ensures lines that are identical are not in diffs.
//
//nolint:paralleltest // package-level globals not concurrency-safe
func TestDiffLinesPartialIdentical(t *testing.T) {
	a := toLinesBuffer("A", "B", "C", "D", "E")
	b := toLinesBuffer("A", "Y", "C", "W", "E")

	diffs := b.Diff(a)
	assert.Len(t, diffs, 2)

	for _, d := range diffs {
		assert.True(t, d == 1 || d == 3, "unexpected diff at line %d", d)
	}
}

// toLinesBuffer converts string arguments to a *LinesBufDiff.
func toLinesBuffer(lines ...string) *buffer.LinesBufDiff {
	buf := buffer.NewLinesBufDiff()
	for _, line := range lines {
		buf.Write([]byte(line))
	}

	return buf
}

func itoa(val int) string {
	if val < 10 {
		//nolint:gosec // G115: safe — val<10, so '0'+val is in ['0','9']
		return string([]byte{byte('0' + val)})
	}

	if val < 100 {
		return string([]byte{byte('0' + val/10), byte('0' + val%10)})
	}

	return "BIG"
}
