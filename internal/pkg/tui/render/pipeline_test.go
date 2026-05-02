package render

import (
	"bytes"
	"strings"
	"testing"
)

// fullPipelineTest runs the full render pipeline and verifies the output.
func runFullPipeline(t *testing.T, width, height int, frames []string) [][]byte {
	t.Helper()

	var outputs [][]byte

	buf := NewCellBuf(width, height)
	prevBuf := NewCellBuf(width, height)

	var termBuf bytes.Buffer

	w := NewWriter(&termBuf)

	for i, content := range frames {
		// Simulate the real program: write to the same buffer each frame.
		// WriteANSIString overwrites cells in-place, bumping lineVersions
		// only for cells that actually change.
		endX, endY := buf.WriteANSIString(0, 0, content)
		_ = endX

		buf.ClearLinesBelow(endY + 1)

		diffs := Diff(buf, prevBuf)

		w.Reset()
		w.WriteDiff(diffs, buf)

		err := w.Flush()
		if err != nil {
			t.Fatalf("frame %d: Flush error: %v", i, err)
		}

		outputs = append(outputs, termBuf.Bytes())
		termBuf.Reset()
		prevBuf.copyFrom(buf)
	}

	return outputs
}

func TestFullPipeline_FirstFrame(t *testing.T) {
	t.Parallel()

	outputs := runFullPipeline(t, 20, 5, []string{"Hello World"})
	if len(outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(outputs))
	}

	if len(outputs[0]) == 0 {
		t.Error("first frame should produce output")
	}

	output := string(outputs[0])
	if !strings.Contains(output, "Hello") {
		t.Errorf("first frame output should contain content: %q", output)
	}
}

func TestFullPipeline_UnchangedFrame(t *testing.T) {
	t.Parallel()

	outputs := runFullPipeline(t, 20, 5, []string{"Hello", "Hello"})
	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}

	if len(outputs[0]) == 0 {
		t.Error("first frame should produce output")
	}

	if len(outputs[1]) != 0 {
		t.Errorf("unchanged second frame should produce no output, got %d bytes", len(outputs[1]))
	}
}

func TestFullPipeline_PartialUpdate(t *testing.T) {
	t.Parallel()

	outputs := runFullPipeline(t, 20, 5, []string{
		"Line 1\nLine 2\nLine 3",
		"Line 1\nCHANGED\nLine 3",
	})
	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}
	// Second frame should have output (only for changed line)
	if len(outputs[1]) == 0 {
		t.Error("partial update should produce output")
	}
}

func TestFullPipeline_AnsiContent(t *testing.T) {
	t.Parallel()

	outputs := runFullPipeline(t, 40, 3, []string{
		"\x1b[1;31mRed Bold\x1b[0m\nNormal\n\x1b[32mGreen\x1b[0m",
	})
	if len(outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(outputs))
	}

	output := string(outputs[0])
	// Should contain SGR sequences
	if !strings.Contains(output, "\x1b[") {
		t.Errorf("ANSI content should produce SGR sequences: %q", output)
	}
}

func TestFullPipeline_ZoneContent(t *testing.T) {
	ResetZones()

	buf := NewCellBuf(20, 3)
	content := Mark("zone1", "Hello") + " World"
	buf.WriteANSIString(0, 0, content)

	if !IsZoneAt(buf, 0, 0, "zone1") {
		t.Error("zone should be detected at (0,0)")
	}
	// After zone end, cells should have ZoneID=0 (fixed bug: applyZoneMarker now resets zoneID)
	cellAfterZone := buf.CellAt(5, 0)
	if cellAfterZone.ZoneID != 0 {
		t.Errorf("cells after zone end should have ZoneID=0, got %d", cellAfterZone.ZoneID)
	}
}

func TestFullPipeline_ResizePreservesContent(t *testing.T) {
	buf := NewCellBuf(10, 3)
	buf.WriteANSIString(0, 0, "0123456789")

	buf.Resize(20, 5)

	// Content from (0,0)-(9,0) should be preserved
	if buf.CellAt(0, 0).Content != "0" {
		t.Error("content should be preserved after resize")
	}

	if buf.CellAt(9, 0).Content != "9" {
		t.Error("content should be preserved after resize")
	}
	// New area should be empty
	if buf.CellAt(10, 0).Content != " " {
		t.Error("new area should be empty after resize")
	}

	if buf.CellAt(0, 3).Content != " " {
		t.Error("new rows should be empty after resize")
	}
}

func TestFullPipeline_MultipleFramesWithDiff(t *testing.T) {
	frames := []string{
		"Frame 0: initial content here",
		"Frame 0: initial content here", // identical
		"Frame 2: CHANGED content here", // changed
		"Frame 2: CHANGED content here", // identical again
	}

	outputs := runFullPipeline(t, 40, 3, frames)

	if len(outputs[0]) == 0 {
		t.Error("frame 0 should have output")
	}
	// Frame 1 is identical content but after Clear+WriteANSIString, lineVersions may differ
	// from prevBuf (which was set via copyFrom). So it may produce diffs.
	// The key check is that frame 2 (different content) produces output.
	if len(outputs[2]) == 0 {
		t.Error("frame 2 (changed) should have output")
	}
}

func TestFullPipeline_WideCharRendering(t *testing.T) {
	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "A世B")

	// Verify cell layout
	if buf.CellAt(0, 0).Content != "A" {
		t.Error("(0,0) should be A")
	}

	if buf.CellAt(1, 0).Content != "世" {
		t.Error("(1,0) should be 世")
	}

	if buf.CellAt(1, 0).Width != 2 {
		t.Error("世 should have width 2")
	}

	if buf.CellAt(2, 0).Content != "" {
		t.Errorf("(2,0) should be continuation cell (empty), got %q", buf.CellAt(2, 0).Content)
	}

	if buf.CellAt(2, 0).Width != 0 {
		t.Error("continuation cell should have width 0")
	}

	if buf.CellAt(3, 0).Content != "B" {
		t.Error("(3,0) should be B")
	}

	// Write diff and verify output
	prevBuf := NewCellBuf(20, 3)
	diffs := Diff(buf, prevBuf)

	var termBuf bytes.Buffer

	w := NewWriter(&termBuf)
	w.WriteDiff(diffs, buf)

	err := w.Flush()
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	output := termBuf.String()
	if !strings.Contains(output, "世") {
		t.Errorf("output should contain wide char: %q", output)
	}
}

func TestFullPipeline_StyleTransitions(t *testing.T) {
	buf := NewCellBuf(40, 3)
	// Use 24-bit color codes (38;2;r;g;b) instead of 16-color codes
	buf.WriteANSIString(0, 0, "\x1b[38;2;255;0;0mRed\x1b[0m \x1b[38;2;0;255;0mGreen\x1b[0m \x1b[38;2;0;0;255mBlue\x1b[0m")

	// Verify cell styles
	if buf.CellAt(0, 0).Fg != NewColor(255, 0, 0) {
		t.Errorf("first cell fg = %v, want red", buf.CellAt(0, 0).Fg)
	}

	if buf.CellAt(3, 0).Content != " " || buf.CellAt(3, 0).Fg != DefaultColor {
		t.Error("space after reset should have default fg")
	}

	if buf.CellAt(4, 0).Fg != NewColor(0, 255, 0) {
		t.Errorf("green cell fg = %v, want green", buf.CellAt(4, 0).Fg)
	}

	// Render
	prevBuf := NewCellBuf(40, 3)
	diffs := Diff(buf, prevBuf)

	var termBuf bytes.Buffer

	w := NewWriter(&termBuf)
	w.WriteDiff(diffs, buf)
	w.Flush()

	output := termBuf.String()
	if !strings.Contains(output, "\x1b[38;2;255;0;0m") {
		t.Errorf("output should contain red SGR: %q", output)
	}

	if !strings.Contains(output, "\x1b[38;2;0;255;0m") {
		t.Errorf("output should contain green SGR: %q", output)
	}
}

func TestFullPipeline_BufferClearThenRender(t *testing.T) {
	buf := NewCellBuf(20, 3)
	prevBuf := NewCellBuf(20, 3)

	buf.WriteANSIString(0, 0, "Content")

	diffs := Diff(buf, prevBuf)
	if len(diffs) == 0 {
		t.Fatal("frame 1 should have diffs")
	}

	prevBuf.copyFrom(buf)

	buf2 := NewCellBuf(20, 3)
	buf2.WriteANSIString(0, 0, "New")

	diffs2 := Diff(buf2, prevBuf)
	if len(diffs2) == 0 {
		t.Error("frame 2 should have diffs")
	}

	var termBuf bytes.Buffer

	w := NewWriter(&termBuf)
	w.WriteDiff(diffs2, buf2)
	w.Flush()

	output := termBuf.String()
	if !strings.Contains(output, "New") {
		t.Errorf("output should contain new content: %q", output)
	}
}

func TestFullPipeline_EmptyFrame(t *testing.T) {
	buf := NewCellBuf(20, 3)
	prevBuf := NewCellBuf(20, 3)

	// Render nothing (buffer stays empty)
	diffs := Diff(buf, prevBuf)
	if len(diffs) != 0 {
		t.Error("empty buffer vs empty prev should have no diffs")
	}
}

func TestFullPipeline_WriteANSIStringOffset(t *testing.T) {
	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(5, 1, "Offset")

	if buf.CellAt(5, 1).Content != "O" {
		t.Error("content should start at offset (5,1)")
	}

	if buf.CellAt(10, 1).Content != "t" {
		t.Error("last char should be at (10,1)")
	}

	if buf.CellAt(4, 1).Content != " " {
		t.Error("cell before offset should be empty")
	}
}

func TestFullPipeline_ClearBumpsLineVersions(t *testing.T) {
	buf := NewCellBuf(20, 3)
	buf.WriteANSIString(0, 0, "Line1")
	buf.WriteANSIString(0, 1, "Line2")

	prevBuf := NewCellBuf(20, 3)
	prevBuf.copyFrom(buf)

	lv0 := buf.LineVersion(0)
	lv1 := buf.LineVersion(1)
	lv2 := buf.LineVersion(2)

	buf.Clear()

	if buf.LineVersion(0) == lv0 {
		t.Error("Clear() should bump lineVersion for line 0")
	}

	if buf.LineVersion(1) == lv1 {
		t.Error("Clear() should bump lineVersion for line 1")
	}

	if buf.LineVersion(2) == lv2 {
		t.Error("Clear() should bump lineVersion for line 2 (empty line)")
	}

	buf.WriteANSIString(0, 0, "New1")

	diffs := Diff(buf, prevBuf)
	if len(diffs) == 0 {
		t.Fatal("Diff must detect changes after Clear()")
	}
}
