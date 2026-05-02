package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

func TestPipelineBlankLineWithANSI(t *testing.T) {
	bufWidth := 120
	bufHeight := 24

	treePrefix := strings.Repeat("│  ", 5)
	if style.CellWidth(treePrefix) != 15 {
		t.Fatalf("tree prefix width = %d, want 15", style.CellWidth(treePrefix))
	}

	innerVPWidth := 103
	contentW := 99

	borderSeq := "\x1b[36m"
	trackSeq := "\x1b[90m"
	reset := "\x1b[0m"

	topBorder := borderSeq + "╭" + reset + strings.Repeat("─", contentW+2) + borderSeq + "╮" + reset
	contentLine := borderSeq + "│" + reset + strings.Repeat(" ", contentW) + " " + trackSeq + "░" + reset + borderSeq + "│" + reset
	blankLine := borderSeq + "│" + reset + strings.Repeat(" ", contentW) + " " + trackSeq + "░" + reset + borderSeq + "│" + reset
	botBorder := borderSeq + "╰" + reset + strings.Repeat("─", contentW+2) + borderSeq + "╯" + reset

	innerView := topBorder + "\n" + contentLine + "\n" + blankLine + "\n" + contentLine + "\n" + botBorder

	for i, line := range strings.Split(innerView, "\n") {
		w := style.CellWidth(line)
		if w != innerVPWidth {
			t.Errorf("inner viewport line %d: width = %d, want %d", i, w, innerVPWidth)
		}
	}

	marked := Mark("test-vp", innerView)

	var prefixed strings.Builder

	for i, line := range strings.Split(marked, "\n") {
		if i > 0 {
			prefixed.WriteString("\n")
		}

		prefixed.WriteString(treePrefix)
		prefixed.WriteString(line)
	}

	mainContentW := bufWidth - 2
	mainSB := "  "

	var mainLines []string

	for line := range strings.SplitSeq(prefixed.String(), "\n") {
		lineW := style.CellWidth(line)

		pad := max(mainContentW-lineW, 0)

		mainLines = append(mainLines, line+strings.Repeat(" ", pad)+mainSB)
	}

	mainView := strings.Join(mainLines, "\n") + "\n"

	buf := NewCellBuf(bufWidth, bufHeight)
	buf.WriteANSIString(0, 0, mainView)

	blankRow := 2

	scrollbarCell := buf.CellAt(116, blankRow)
	if scrollbarCell.Content != "░" {
		t.Errorf("blank line scrollbar (116,%d): content=%q want=%q", blankRow, scrollbarCell.Content, "░")
	}

	rightBorderCell := buf.CellAt(117, blankRow)
	if rightBorderCell.Content != "│" {
		t.Errorf("blank line right border (117,%d): content=%q want=%q", blankRow, rightBorderCell.Content, "│")
	}

	borderChars := map[int]string{0: "╮", 1: "│", 2: "│", 3: "│", 4: "╯"}
	for row, want := range borderChars {
		cell := buf.CellAt(117, row)
		if cell.Content != want {
			t.Errorf("row %d: right border at col 117 = %q, want %q", row, cell.Content, want)
		}
	}
}

func TestPipelineWriteDiffClearsStaleScrollbar(t *testing.T) {
	bufWidth := 30
	bufHeight := 5

	buf1 := NewCellBuf(bufWidth, bufHeight)
	prevBuf := NewCellBuf(bufWidth, bufHeight)

	trackColor := NewColor16(8)
	borderColor := NewColor16(6)

	buf1.SetCell(20, 0, Cell{Content: "░", Width: 1, Fg: trackColor, ZoneID: 1})
	buf1.SetCell(21, 0, Cell{Content: "│", Width: 1, Fg: borderColor, ZoneID: 1})

	diffs := Diff(buf1, prevBuf)
	if len(diffs) == 0 {
		t.Fatal("expected diffs on frame 1")
	}

	var discard bytes.Buffer

	w1 := NewWriter(&discard)
	w1.WriteDiff(diffs, buf1)
	w1.Flush()

	prevBuf.copyFrom(buf1)

	buf2 := NewCellBuf(bufWidth, bufHeight)

	diffs2 := Diff(buf2, prevBuf)
	if len(diffs2) == 0 {
		t.Fatal("expected diffs on frame 2")
	}

	var discard2 bytes.Buffer

	w2 := NewWriter(&discard2)
	w2.WriteDiff(diffs2, buf2)
	output2 := string(w2.buf)

	if !strings.Contains(output2, "\x1b[K") {
		t.Errorf("frame 2 output missing \\x1b[K, got: %q", output2)
	}
}

func TestPipelineWriteDiffClearLineWithHighlightBg(t *testing.T) {
	bufWidth := 30
	bufHeight := 5

	buf := NewCellBuf(bufWidth, bufHeight)
	prevBuf := NewCellBuf(bufWidth, bufHeight)

	buf.WriteANSIString(0, 0, "\x1b[48;5;12m░│\x1b[0m")

	diffs := Diff(buf, prevBuf)

	var discard bytes.Buffer

	w1 := NewWriter(&discard)
	w1.WriteDiff(diffs, buf)
	w1.Flush()

	prevBuf.copyFrom(buf)

	buf.WriteANSIString(0, 0, "\x1b[48;5;12m  \x1b[0m")

	diffs2 := Diff(buf, prevBuf)

	var discard2 bytes.Buffer

	w2 := NewWriter(&discard2)
	w2.WriteDiff(diffs2, buf)
	output2 := string(w2.buf)

	if !strings.Contains(output2, "\x1b[K") {
		t.Errorf("highlight bg frame: missing \\x1b[K, got: %q", output2)
	}
}

func TestPipelineDiffClearsStaleContentOnShrinkingLine(t *testing.T) {
	bufWidth := 20
	bufHeight := 3

	borderColor := NewColor16(7)

	buf1 := NewCellBuf(bufWidth, bufHeight)
	buf1.WriteANSIString(0, 0, "Hello World")
	buf1.SetCell(19, 0, Cell{Content: "│", Width: 1, Fg: borderColor})

	prevBuf := NewCellBuf(bufWidth, bufHeight)
	diffs1 := Diff(buf1, prevBuf)

	var discard bytes.Buffer

	w1 := NewWriter(&discard)
	w1.WriteDiff(diffs1, buf1)
	w1.Flush()

	prevBuf.copyFrom(buf1)

	buf2 := NewCellBuf(bufWidth, bufHeight)
	buf2.WriteANSIString(0, 0, "Hi")
	buf2.SetCell(19, 0, Cell{Content: "│", Width: 1, Fg: borderColor})

	diffs2 := Diff(buf2, prevBuf)
	if len(diffs2) == 0 {
		t.Fatal("frame 2 should have diffs")
	}

	// Verify line 0 is in the diffs (the only line that changed)
	found := false

	for _, d := range diffs2 {
		if d.Y == 0 {
			found = true

			break
		}
	}

	if !found {
		t.Error("line 0 should be in diffs")
	}

	var discard2 bytes.Buffer

	w2 := NewWriter(&discard2)
	w2.WriteDiff(diffs2, buf2)
	output2 := string(w2.buf)

	// With full-line write, the output should write positions 0-19 as spaces/content
	// then \x1b[K to clear beyond. This clears the stale "rld" from "World".
	if !strings.Contains(output2, "\x1b[K") {
		t.Errorf("should emit \\x1b[K after full-line write, got: %q", output2)
	}
}
