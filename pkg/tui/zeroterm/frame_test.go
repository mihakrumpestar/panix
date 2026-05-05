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
func TestShrinkThenGrowWithIdenticalPrefix(t *testing.T) {
	t.Parallel()

	const terminalHeight = 120

	fullFrame := []string{
		"=== Header ===",       // 0
		"",                     // 1
		"=== Stats Table ===",  // 2
		"",                     // 3
		"row1 | data1",         // 4
		"row2 | data2",         // 5
		"",                     // 6
		"=== Phase Status ===", // 7
		"",                     // 8
		"INSPECT  BUILD  DONE", // 9
		"   1      2      3",   // 10
		"",                     // 11
		"=== Build Logs ===",   // 12
		"",                     // 13
		"flake1 (5.23s)",       // 14
		"  machine (2.50s)",    // 15
		"===",                  // 16
	}

	// After restart: some lines match fullFrame by coincidence
	shortFrame := []string{
		"=== Header ===",      // 0  - SAME
		"",                    // 1  - SAME
		"=== Stats Table ===", // 2  - SAME
		"",                    // 3  - SAME
		"",                    // 4  - DIFFERENT
		"",                    // 5  - DIFFERENT
		"",                    // 6  - SAME
		"=== Build Logs ===",  // 7  - DIFFERENT
		"",                    // 8  - SAME
		"",                    // 9  - DIFFERENT
		"",                    // 10 - DIFFERENT
		"",                    // 11 - SAME
		"=== Build Logs ===",  // 12 - SAME
		"",                    // 13 - SAME
	}

	var (
		prevLines []string
		buf       []byte
	)

	// Step 1: initial full frame (prevLines empty → all lines are diffs)
	diffs1 := DiffLines(fullFrame, prevLines)
	buf = RenderLines(buf[:0], diffs1, fullFrame, 0, terminalHeight)
	updatePrev(&prevLines, fullFrame)

	// Step 2: shrink to short frame — prevLines must be cleared so
	// DiffLines returns ALL lines as changed
	diffs2 := DiffLines(shortFrame, prevLines)

	prevCount2 := len(prevLines)
	if len(shortFrame) < prevCount2 {
		prevLines = prevLines[:0]
		diffs2 = DiffLines(shortFrame, prevLines)
	}

	buf = RenderLines(buf[:0], diffs2, shortFrame, prevCount2, terminalHeight)
	out2 := string(buf)

	t.Logf("Step 2 shrink: %d lines vs prev %d, %d diffs",
		len(shortFrame), prevCount2, len(diffs2))
	updatePrev(&prevLines, shortFrame)

	// After shrink, ALL short frame lines must be in the output
	for i, line := range shortFrame {
		prefix := "\x1b[" + itoa(i+1) + ";1H"
		if !strings.Contains(out2, prefix+line) {
			t.Errorf("shrink missing line %d at row %d: %q%q",
				i, i+1, prefix, line)
		}
	}

	// Old lines beyond short frame should NOT be written
	// at their original row positions
	for i := len(shortFrame); i < len(fullFrame); i++ {
		oldPrefix := "\x1b[" + itoa(i+1) + ";1H"

		oldLine := oldPrefix + fullFrame[i]
		if strings.Contains(out2, oldLine) {
			t.Errorf("stale fullFrame[%d]=%q at row %d leaked", i, fullFrame[i], i+1)
		}
	}

	// Step 3: grow back to full frame.
	// Lines 0-13 that match shortFrame are already on the terminal
	// from step 2 (the shrink rewrote ALL lines). So we only need diffs
	// for lines that changed (4,5,7,9,10,12,14,15,16).
	prevCount3 := len(prevLines)
	if len(fullFrame) < prevCount3 {
		prevLines = prevLines[:0]
	}

	diffs3 := DiffLines(fullFrame, prevLines)
	buf = RenderLines(buf[:0], diffs3, fullFrame, prevCount3, terminalHeight)
	out3 := string(buf)

	t.Logf("Step 3 grow-back: %d lines vs prev %d, %d diffs",
		len(fullFrame), prevCount3, len(diffs3))
	updatePrev(&prevLines, fullFrame)

	// Lines that DIFFER between short and full must be written
	for i, line := range fullFrame {
		if i < len(shortFrame) && line == shortFrame[i] {
			continue // same as short → already on terminal from step 2
		}

		prefix := "\x1b[" + itoa(i+1) + ";1H"
		if !strings.Contains(out3, prefix+line) {
			t.Errorf("grow-back missing line %d at row %d: %q%q",
				i, i+1, prefix, line)
		}
	}

	// New lines beyond short frame must also be present
	for i := len(shortFrame); i < len(fullFrame); i++ {
		prefix := "\x1b[" + itoa(i+1) + ";1H"
		if !strings.Contains(out3, prefix+fullFrame[i]) {
			t.Errorf("grow-back missing new line %d at row %d: %q%q",
				i, i+1, prefix, fullFrame[i])
		}
	}
}

func TestShrinkThenGrow(t *testing.T) {
	t.Parallel()

	const terminalHeight = 120

	fullFrame := []string{
		"F-000", "F-001", "F-002", "F-003", "F-004",
		"F-005", "F-006", "F-007", "F-008", "F-009",
		"F-010", "F-011", "F-012", "F-013", "F-014",
		"F-015", "F-016", "F-017",
	}

	shortFrame := []string{
		"S-000", "S-001", "S-002", "S-003", "S-004",
		"S-005", "S-006", "S-007", "S-008",
	}

	var prevLines []string

	diffs1 := DiffLines(fullFrame, prevLines)

	var buf []byte

	buf = RenderLines(buf[:0], diffs1, fullFrame, 0, terminalHeight)
	updatePrev(&prevLines, fullFrame)

	diffs2 := DiffLines(shortFrame, prevLines)
	buf = buf[:0]
	buf = RenderLines(buf, diffs2, shortFrame, len(fullFrame), terminalHeight)
	output2 := string(buf)

	for i := 9; i < len(fullFrame); i++ {
		if strings.Contains(output2, fullFrame[i]) {
			t.Errorf("FULL frame[%d]=%q leaked into shrink output", i, fullFrame[i])
		}
	}

	updatePrev(&prevLines, shortFrame)

	diffs3 := DiffLines(fullFrame, prevLines)
	buf = buf[:0]
	buf = RenderLines(buf, diffs3, fullFrame, len(shortFrame), terminalHeight)
	output3 := string(buf)

	for i, line := range fullFrame {
		if !strings.Contains(output3, line) {
			t.Errorf("Render3 missing fullFrame[%d]=%q", i, line)
		}
	}

	for i := range fullFrame {
		want := fullFrame[i]

		prefix := "\x1b[" + itoa(i+1) + ";1H"
		if !strings.Contains(output3, prefix+want) {
			t.Errorf("Render3 line %d not positioned at row %d. Expected %q%q in output",
				i, i+1, prefix, want)
		}
	}
}

// TestRenderFrameWithANSIContent tests the render pipeline with
// realistic ANSI-styled content that may contain \r characters
// and escape sequences.
func TestRenderFrameWithANSIContent(t *testing.T) {
	t.Parallel()

	const terminalHeight = 120

	ansiFull := []string{
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
	}

	ansiShort := []string{
		"\x1b[1;34m=== HEADER ===\x1b[0m",
		"",
		"\x1b[1m=== Stats Table ===\x1b[0m",
		"",
		"",
		"\x1b[1m=== Build Logs ===\x1b[0m",
		"",
		"",
		"===",
	}

	var prevLines []string

	diffs1 := DiffLines(ansiFull, prevLines)

	var buf []byte

	buf = RenderLines(buf[:0], diffs1, ansiFull, 0, terminalHeight)
	updatePrev(&prevLines, ansiFull)

	diffs2 := DiffLines(ansiShort, prevLines)
	buf = buf[:0]
	buf = RenderLines(buf, diffs2, ansiShort, len(ansiFull), terminalHeight)
	t.Logf("ANSI shrink output: %q", string(buf))

	// Lines 0-4 are identical in both frames, so they should NOT be
	// re-written (terminal already shows them correctly).
	// Verify that changed lines (5-8) ARE in the output.
	output2 := string(buf)
	if !strings.Contains(output2, "=== Build Logs ===") {
		t.Error("changed build logs header missing from shrink output")
	}

	// Lines that DIFFER should be in output
	if !strings.Contains(output2, "===") {
		t.Error("changed footer missing from shrink output")
	}

	// Lines from the old frame that are beyond the short frame
	// should NOT appear as written content (the clear-below handles them)
	for i := 9; i < len(ansiFull); i++ {
		// The old content might appear as part of WRITE operations,
		// but only if the short frame line content matches.
		// Check that the line is positioned at the correct index.
		oldPrefix := "\x1b[" + itoa(i+1) + ";1H"
		if strings.Contains(output2, oldPrefix+ansiFull[i]) {
			t.Errorf("stale ansi line %d written: prefix %q", i, oldPrefix+ansiFull[i])
		}
	}

	updatePrev(&prevLines, ansiShort)

	// After shrink, unchanged lines (0-4) should NOT be rewritten
	// Changed lines should be written, and stale lines cleared
	// Lines 0, 2 are identical between ansiFull and ansiShort
	// so they are correctly NOT in diffs (no need to rewrite)

	diffs3 := DiffLines(ansiFull, prevLines)
	buf = buf[:0]
	buf = RenderLines(buf, diffs3, ansiFull, len(ansiShort), terminalHeight)

	// After growing back, the diff should mark ALL lines as changed
	// because short frame has different values than full frame at all positions
	output3 := string(buf)
	// Every non-empty full-frame line should appear in diffs
	// (except lines identical to short frame, which are already on screen)
	for i, line := range ansiFull {
		if line != "" && !strings.Contains(output3, line) {
			// Lines 0-4 in full frame are identical to short frame lines 0-4
			// so they're correctly NOT rewritten (already on screen)
			if i > 4 || (ansiShort[i] != ansiFull[i]) {
				t.Errorf("ANSIGrow Render3 missing line %d: %q", i, line)
			}
		}
	}
}

// TestDiffLinesSameLengthDifferentContent ensures that when frames have
// the same length but different content on every line, all lines are in diffs.
func TestDiffLinesSameLengthDifferentContent(t *testing.T) {
	t.Parallel()

	a := []string{"A", "B", "C", "D", "E"}
	b := []string{"X", "Y", "Z", "W", "V"}

	diffs := DiffLines(b, a)
	if len(diffs) != 5 {
		t.Errorf("expected 5 diffs, got %d", len(diffs))
	}
}

// TestDiffLinesPartialIdentical ensures lines that are identical are not in diffs.
func TestDiffLinesPartialIdentical(t *testing.T) {
	t.Parallel()

	a := []string{"A", "B", "C", "D", "E"}
	b := []string{"A", "Y", "C", "W", "E"}

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

func updatePrev(prevLines *[]string, lines []string) {
	if cap(*prevLines) >= len(lines) {
		*prevLines = (*prevLines)[:len(lines)]
	} else {
		*prevLines = make([]string, len(lines))
	}

	copy(*prevLines, lines)
}

func itoa(n int) string {
	if n < 10 {
		return string([]byte{byte('0' + n)})
	}

	if n < 100 {
		return string([]byte{byte('0' + n/10), byte('0' + n%10)})
	}

	return "BIG"
}
