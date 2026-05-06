package zeroterm

import (
	"strings"
	"testing"
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

	var prevLines [][]byte

	// Step 1: initial full frame (prevLines empty → all lines are diffs)
	diffs1 := DiffLines(fullFrame, prevLines)
	RenderLines(nil, diffs1, fullFrame, 0, terminalHeight)
	updatePrev(&prevLines, fullFrame)

	// Step 2: shrink to short frame
	out2 := renderShrinkStep(t, shortFrame, &prevLines, terminalHeight)
	verifyShrinkOutput(t, out2, shortFrame, fullFrame)

	// Step 3: grow back to full frame
	out3 := renderGrowBackStep(t, fullFrame, &prevLines, terminalHeight)
	verifyGrowBackOutput(t, out3, fullFrame, shortFrame)
}

func makeIdenticalPrefixFullFrame() [][]byte {
	return toByteLines(
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
	)
}

func makeIdenticalPrefixShortFrame() [][]byte {
	return toByteLines(
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
	)
}

func renderShrinkStep(t *testing.T, shortFrame [][]byte, prevLines *[][]byte, terminalHeight int) string {
	t.Helper()

	diffs2 := DiffLines(shortFrame, *prevLines)

	prevCount2 := len(*prevLines)
	if len(shortFrame) < prevCount2 {
		*prevLines = (*prevLines)[:0]
		diffs2 = DiffLines(shortFrame, *prevLines)
	}

	buf := RenderLines(nil, diffs2, shortFrame, prevCount2, terminalHeight)
	out2 := string(buf)

	t.Logf("Step 2 shrink: %d lines vs prev %d, %d diffs",
		len(shortFrame), prevCount2, len(diffs2))
	updatePrev(prevLines, shortFrame)

	return out2
}

func verifyShrinkOutput(t *testing.T, out2 string, shortFrame, fullFrame [][]byte) {
	t.Helper()

	for i, line := range shortFrame {
		prefix := "\x1b[" + itoa(i+1) + ";1H"
		if !strings.Contains(out2, prefix+string(line)) {
			t.Errorf("shrink missing line %d at row %d: %q%q",
				i, i+1, prefix, string(line))
		}
	}

	for i := len(shortFrame); i < len(fullFrame); i++ {
		oldPrefix := "\x1b[" + itoa(i+1) + ";1H"

		oldLine := oldPrefix + string(fullFrame[i])
		if strings.Contains(out2, oldLine) {
			t.Errorf("stale fullFrame[%d]=%q at row %d leaked", i, string(fullFrame[i]), i+1)
		}
	}
}

func renderGrowBackStep(t *testing.T, fullFrame [][]byte, prevLines *[][]byte, terminalHeight int) string {
	t.Helper()

	prevCount3 := len(*prevLines)
	if len(fullFrame) < prevCount3 {
		*prevLines = (*prevLines)[:0]
	}

	diffs3 := DiffLines(fullFrame, *prevLines)
	buf := RenderLines(nil, diffs3, fullFrame, prevCount3, terminalHeight)
	out3 := string(buf)

	t.Logf("Step 3 grow-back: %d lines vs prev %d, %d diffs",
		len(fullFrame), prevCount3, len(diffs3))
	updatePrev(prevLines, fullFrame)

	return out3
}

func verifyGrowBackOutput(t *testing.T, out3 string, fullFrame, shortFrame [][]byte) {
	t.Helper()

	for lineIdx, line := range fullFrame {
		if lineIdx < len(shortFrame) && string(line) == string(shortFrame[lineIdx]) {
			continue
		}

		prefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
		if !strings.Contains(out3, prefix+string(line)) {
			t.Errorf("grow-back missing line %d at row %d: %q%q",
				lineIdx, lineIdx+1, prefix, string(line))
		}
	}

	for lineIdx := len(shortFrame); lineIdx < len(fullFrame); lineIdx++ {
		prefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
		if !strings.Contains(out3, prefix+string(fullFrame[lineIdx])) {
			t.Errorf("grow-back missing new line %d at row %d: %q%q",
				lineIdx, lineIdx+1, prefix, string(fullFrame[lineIdx]))
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestShrinkThenGrow(t *testing.T) {
	const terminalHeight = 120

	fullFrame := toByteLines(
		"F-000", "F-001", "F-002", "F-003", "F-004",
		"F-005", "F-006", "F-007", "F-008", "F-009",
		"F-010", "F-011", "F-012", "F-013", "F-014",
		"F-015", "F-016", "F-017",
	)

	shortFrame := toByteLines(
		"S-000", "S-001", "S-002", "S-003", "S-004",
		"S-005", "S-006", "S-007", "S-008",
	)

	var prevLines [][]byte

	diffs1 := DiffLines(fullFrame, prevLines)

	var buf []byte

	buf = RenderLines(buf[:0], diffs1, fullFrame, 0, terminalHeight)
	updatePrev(&prevLines, fullFrame)

	diffs2 := DiffLines(shortFrame, prevLines)
	buf = buf[:0]
	buf = RenderLines(buf, diffs2, shortFrame, len(fullFrame), terminalHeight)
	output2 := string(buf)

	for i := 9; i < len(fullFrame); i++ {
		if strings.Contains(output2, string(fullFrame[i])) {
			t.Errorf("FULL frame[%d]=%q leaked into shrink output", i, string(fullFrame[i]))
		}
	}

	updatePrev(&prevLines, shortFrame)

	diffs3 := DiffLines(fullFrame, prevLines)
	buf = buf[:0]
	buf = RenderLines(buf, diffs3, fullFrame, len(shortFrame), terminalHeight)
	output3 := string(buf)

	for lineIdx, line := range fullFrame {
		if !strings.Contains(output3, string(line)) {
			t.Errorf("Render3 missing fullFrame[%d]=%q", lineIdx, string(line))
		}
	}

	for lineIdx := range fullFrame {
		want := string(fullFrame[lineIdx])

		prefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
		if !strings.Contains(output3, prefix+want) {
			t.Errorf("Render3 line %d not positioned at row %d. Expected %q%q in output",
				lineIdx, lineIdx+1, prefix, want)
		}
	}
}

// TestRenderFrameWithANSIContent tests the render pipeline with
// realistic ANSI-styled content that may contain \r characters
// and escape sequences.

//nolint:paralleltest // package-level globals not concurrency-safe
func TestRenderFrameWithANSIContent(t *testing.T) {
	const terminalHeight = 120

	ansiFull := ansiFullContent()
	ansiShort := ansiShortContent()

	t.Run("shrink", func(t *testing.T) {
		var prevLines [][]byte

		diffs1 := DiffLines(ansiFull, prevLines)

		var buf []byte

		buf = RenderLines(buf[:0], diffs1, ansiFull, 0, terminalHeight)
		updatePrev(&prevLines, ansiFull)

		diffs2 := DiffLines(ansiShort, prevLines)
		buf = buf[:0]
		buf = RenderLines(buf, diffs2, ansiShort, len(ansiFull), terminalHeight)
		t.Logf("ANSI shrink output: %q", string(buf))

		output2 := string(buf)
		if !strings.Contains(output2, "=== Build Logs ===") {
			t.Error("changed build logs header missing from shrink output")
		}

		if !strings.Contains(output2, "===") {
			t.Error("changed footer missing from shrink output")
		}

		for lineIdx := 9; lineIdx < len(ansiFull); lineIdx++ {
			oldPrefix := "\x1b[" + itoa(lineIdx+1) + ";1H"
			if strings.Contains(output2, oldPrefix+string(ansiFull[lineIdx])) {
				t.Errorf("stale ansi line %d written: prefix %q", lineIdx, oldPrefix+string(ansiFull[lineIdx]))
			}
		}

		updatePrev(&prevLines, ansiShort)

		diffs3 := DiffLines(ansiFull, prevLines)
		buf = buf[:0]
		buf = RenderLines(buf, diffs3, ansiFull, len(ansiShort), terminalHeight)

		output3 := string(buf)
		for i, line := range ansiFull {
			if len(line) > 0 && !strings.Contains(output3, string(line)) {
				if i > 4 || string(ansiShort[i]) != string(ansiFull[i]) {
					t.Errorf("ANSIGrow Render3 missing line %d: %q", i, string(line))
				}
			}
		}
	})
}

func ansiFullContent() [][]byte {
	return toByteLines(
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
	)
}

func ansiShortContent() [][]byte {
	return toByteLines(
		"\x1b[1;34m=== HEADER ===\x1b[0m",
		"",
		"\x1b[1m=== Stats Table ===\x1b[0m",
		"",
		"",
		"\x1b[1m=== Build Logs ===\x1b[0m",
		"",
		"",
		"===",
	)
}

// TestDiffLinesSameLengthDifferentContent ensures that when frames have
// the same length but different content on every line, all lines are in diffs.
//
//nolint:paralleltest // package-level globals not concurrency-safe
func TestDiffLinesSameLengthDifferentContent(t *testing.T) {
	a := toByteLines("A", "B", "C", "D", "E")
	b := toByteLines("X", "Y", "Z", "W", "V")

	diffs := DiffLines(b, a)
	if len(diffs) != 5 {
		t.Errorf("expected 5 diffs, got %d", len(diffs))
	}
}

// TestDiffLinesPartialIdentical ensures lines that are identical are not in diffs.
//
//nolint:paralleltest // package-level globals not concurrency-safe
func TestDiffLinesPartialIdentical(t *testing.T) {
	a := toByteLines("A", "B", "C", "D", "E")
	b := toByteLines("A", "Y", "C", "W", "E")

	diffs := DiffLines(b, a)
	if len(diffs) != 2 {
		t.Errorf("expected 2 diffs, got %d", len(diffs))
	}

	for _, d := range diffs {
		if d.Y != 1 && d.Y != 3 {
			t.Errorf("unexpected diff at line %d", d.Y)
		}
	}
}

func updatePrev(prevLines *[][]byte, lines [][]byte) {
	if cap(*prevLines) >= len(lines) {
		*prevLines = (*prevLines)[:len(lines)]
	} else {
		*prevLines = make([][]byte, len(lines))
	}

	copy(*prevLines, lines)
}

// toByteLines converts string arguments to [][]byte.
func toByteLines(lines ...string) [][]byte {
	result := make([][]byte, len(lines))
	for i, line := range lines {
		result[i] = []byte(line)
	}

	return result
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
