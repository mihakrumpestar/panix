package style

import (
	"bytes"
	"testing"
)

func TestNormalBorder(t *testing.T) {
	t.Parallel()

	brd := NormalBorder()

	if !bytes.Equal(brd.TopLeft, []byte("┌")) || !bytes.Equal(brd.TopRight, []byte("┐")) {
		t.Errorf("NormalBorder corners = (%q, %q), want (┌, ┐)", brd.TopLeft, brd.TopRight)
	}

	if !bytes.Equal(brd.BottomLeft, []byte("└")) || !bytes.Equal(brd.BottomRight, []byte("┘")) {
		t.Errorf("NormalBorder bottom corners = (%q, %q), want (└, ┘)", brd.BottomLeft, brd.BottomRight)
	}

	if !bytes.Equal(brd.Horizontal, []byte("─")) || !bytes.Equal(brd.Vertical, []byte("│")) {
		t.Errorf("NormalBorder lines = (%q, %q), want (─, │)", brd.Horizontal, brd.Vertical)
	}

	if !bytes.Equal(brd.TopMid, []byte("┬")) ||
		!bytes.Equal(brd.BottomMid, []byte("┴")) ||
		!bytes.Equal(brd.LeftMid, []byte("├")) ||
		!bytes.Equal(brd.RightMid, []byte("┤")) {
		t.Errorf("NormalBorder mids = (%q, %q, %q, %q), want (┬, ┴, ├, ┤)",
			brd.TopMid, brd.BottomMid, brd.LeftMid, brd.RightMid)
	}

	if !bytes.Equal(brd.MidMid, []byte("┼")) {
		t.Errorf("NormalBorder MidMid = %q, want ┼", brd.MidMid)
	}
}

func TestRoundedBorder(t *testing.T) {
	t.Parallel()

	brd := RoundedBorder()

	if !bytes.Equal(brd.TopLeft, []byte("╭")) || !bytes.Equal(brd.TopRight, []byte("╮")) {
		t.Errorf("RoundedBorder top corners = (%q, %q), want (╭, ╮)", brd.TopLeft, brd.TopRight)
	}

	if !bytes.Equal(brd.BottomLeft, []byte("╰")) || !bytes.Equal(brd.BottomRight, []byte("╯")) {
		t.Errorf("RoundedBorder bottom corners = (%q, %q), want (╰, ╯)", brd.BottomLeft, brd.BottomRight)
	}

	if !bytes.Equal(brd.Horizontal, []byte("─")) || !bytes.Equal(brd.Vertical, []byte("│")) {
		t.Errorf("RoundedBorder lines = (%q, %q), want (─, │)", brd.Horizontal, brd.Vertical)
	}
}

func TestHiddenBorder(t *testing.T) {
	t.Parallel()

	brd := HiddenBorder()

	if len(brd.TopLeft) != 0 || len(brd.Horizontal) != 0 || len(brd.Vertical) != 0 {
		t.Errorf("HiddenBorder should have all empty, got TopLeft=%q Horizontal=%q Vertical=%q",
			brd.TopLeft, brd.Horizontal, brd.Vertical)
	}
}

func TestMarkdownBorder(t *testing.T) {
	t.Parallel()

	brd := MarkdownBorder()

	if !bytes.Equal(brd.TopLeft, []byte("|")) ||
		!bytes.Equal(brd.TopRight, []byte("|")) ||
		!bytes.Equal(brd.BottomLeft, []byte("|")) ||
		!bytes.Equal(brd.BottomRight, []byte("|")) {
		t.Error("MarkdownBorder corners should all be |")
	}

	if !bytes.Equal(brd.Horizontal, []byte("-")) || !bytes.Equal(brd.Vertical, []byte("|")) {
		t.Errorf("MarkdownBorder lines = (%q, %q), want (-, |)", brd.Horizontal, brd.Vertical)
	}
}

func TestBorder_NoPerSideColorByDefault(t *testing.T) {
	t.Parallel()

	brd := NormalBorder()

	if len(brd.topFg) != 0 || len(brd.rightFg) != 0 || len(brd.bottomFg) != 0 || len(brd.leftFg) != 0 {
		t.Error("New border should have empty per-side color prefixes")
	}
}
